package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	k3s "github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

var (
	errNilNodeRef                 = errors.New("noderef is nil")
	errNoControlPlaneNodes        = errors.New("no control plane members")
	errClusterIsBeingDeleted      = errors.New("cluster is being deleted")
	errControlPlaneIsBeingDeleted = errors.New("control plane is being deleted")
)

// KThreesControlPlaneReconciler reconciles a KThreesControlPlane object.
type MachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	EtcdDialTimeout time.Duration
	EtcdCallTimeout time.Duration

	managementCluster         k3s.ManagementCluster
	managementClusterUncached k3s.ManagementCluster
}

func (r *MachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, log *logr.Logger, concurrency int) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Machine{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrency,
		}).
		WithEventFilter(predicates.ResourceNotPaused(r.Log)).
		Build(r)

	if r.managementCluster == nil {
		r.managementCluster = &k3s.Management{
			Client:          r.Client,
			EtcdDialTimeout: r.EtcdDialTimeout,
			EtcdCallTimeout: r.EtcdCallTimeout,
		}
	}

	if r.managementClusterUncached == nil {
		r.managementClusterUncached = &k3s.Management{
			Client:          mgr.GetAPIReader(),
			EtcdDialTimeout: r.EtcdDialTimeout,
			EtcdCallTimeout: r.EtcdCallTimeout,
		}
	}

	return err
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch;create;update;patch;delete
func (r *MachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("namespace", req.Namespace, "machine", req.Name)

	m := &clusterv1.Machine{}
	if err := r.Client.Get(ctx, req.NamespacedName, m); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if m.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// if machine registered PreTerminate hook, wait for capi asks to resolve PreTerminateDeleteHook
	if annotations.HasWithPrefix(clusterv1.PreTerminateDeleteHookAnnotationPrefix, m.ObjectMeta.Annotations) &&
		m.ObjectMeta.Annotations[clusterv1.PreTerminateDeleteHookAnnotationPrefix] == k3sHookName {
		if !conditions.IsFalse(m, clusterv1.PreTerminateDeleteHookSucceededCondition) {
			logger.Info("wait for machine drain and detech volume operation complete.")
			return ctrl.Result{}, nil
		}

		cluster, err := util.GetClusterFromMetadata(ctx, r.Client, m.ObjectMeta)
		if err != nil {
			logger.Info("unable to get cluster.")
			return ctrl.Result{}, errors.Wrapf(err, "unable to get cluster")
		}

		err = r.isRemoveEtcdMemberNeeded(ctx, cluster, m)
		isRemoveEtcdMemberNeeded := err == nil
		if err != nil {
			switch err {
			case errNoControlPlaneNodes, errNilNodeRef, errClusterIsBeingDeleted, errControlPlaneIsBeingDeleted:
				nodeName := ""
				if m.Status.NodeRef != nil {
					nodeName = m.Status.NodeRef.Name
				}
				logger.Info("Skipping removal for etcd member associated with Machine as it is not needed", "Node", klog.KRef("", nodeName), "cause", err.Error())
			default:
				return ctrl.Result{}, errors.Wrapf(err, "failed to check if k3s etcd member remove is needed")
			}
		}

		if isRemoveEtcdMemberNeeded {
			workloadCluster, err := r.managementCluster.GetWorkloadCluster(ctx, util.ObjectKey(cluster))
			if err != nil {
				logger.Error(err, "failed to create client to workload cluster")
				return ctrl.Result{}, errors.Wrapf(err, "failed to create client to workload cluster")
			}

			etcdRemoved, err := workloadCluster.RemoveEtcdMemberForMachine(ctx, m)
			if err != nil {
				logger.Error(err, "failed to remove etcd member for machine")
				return ctrl.Result{}, err
			}
			if !etcdRemoved {
				logger.Info("wait k3s embedded etcd controller to remove etcd")
				return ctrl.Result{RequeueAfter: etcdRemovalRequeueAfter}, nil
			}

			nodeName := ""
			if m.Status.NodeRef != nil {
				nodeName = m.Status.NodeRef.Name
			}

			logger.Info("etcd remove etcd member succeeded", "Node", klog.KRef("", nodeName))
		}

		patchHelper, err := patch.NewHelper(m, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create patch helper for machine")
		}

		mAnnotations := m.GetAnnotations()
		delete(mAnnotations, clusterv1.PreTerminateDeleteHookAnnotationPrefix)
		m.SetAnnotations(mAnnotations)
		if err := patchHelper.Patch(ctx, m); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed patch machine")
		}
	}

	return ctrl.Result{}, nil
}

// isRemoveEtcdMemberNeeded returns nil if the Machine's NodeRef is not nil
// and if the Machine is not the last control plane node in the cluster
// and if the Cluster/KThreesControlplane associated with the Machine is not deleted.
func (r *MachineReconciler) isRemoveEtcdMemberNeeded(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	log := ctrl.LoggerFrom(ctx)
	// Return early if the cluster is being deleted.
	if !cluster.DeletionTimestamp.IsZero() {
		return errClusterIsBeingDeleted
	}

	// Cannot remove etcd member if the node doesn't exist.
	if machine.Status.NodeRef == nil {
		return errNilNodeRef
	}

	// controlPlaneRef is an optional field in the Cluster so skip the external
	// managed control plane check if it is nil
	if cluster.Spec.ControlPlaneRef != nil {
		controlPlane, err := external.Get(ctx, r.Client, cluster.Spec.ControlPlaneRef, cluster.Spec.ControlPlaneRef.Namespace)
		if apierrors.IsNotFound(err) {
			// If control plane object in the reference does not exist, log and skip check for
			// external managed control plane
			log.Error(err, "control plane object specified in cluster spec.controlPlaneRef does not exist", "kind", cluster.Spec.ControlPlaneRef.Kind, "name", cluster.Spec.ControlPlaneRef.Name)
		} else {
			if err != nil {
				// If any other error occurs when trying to get the control plane object,
				// return the error so we can retry
				return err
			}

			// Return early if the object referenced by controlPlaneRef is being deleted.
			if !controlPlane.GetDeletionTimestamp().IsZero() {
				return errControlPlaneIsBeingDeleted
			}
		}
	}

	// Get all of the active machines that belong to this cluster.
	machines, err := collections.GetFilteredMachinesForCluster(ctx, r.Client, cluster, collections.ActiveMachines)
	if err != nil {
		return err
	}

	// Whether or not it is okay to remove etcd member depends on the
	// number of remaining control plane members and whether or not this
	// machine is one of them.
	numControlPlaneMachines := len(machines.Filter(collections.ControlPlaneMachines(cluster.Name)))
	if numControlPlaneMachines == 0 {
		// Do not remove etcd member if there are no remaining members of
		// the control plane.
		return errNoControlPlaneNodes
	}
	// Otherwise it is okay to remove etcd member.
	return nil
}
