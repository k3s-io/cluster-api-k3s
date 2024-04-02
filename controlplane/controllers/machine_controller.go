package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k3s "github.com/k3s-io/cluster-api-k3s/pkg/k3s"
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

func (r *MachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, log *logr.Logger) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Machine{}).
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

	// if machine registered PreTerminate hook, wait for capi to drain and deattach volume, then remove etcd member
	if annotations.HasWithPrefix(clusterv1.PreTerminateDeleteHookAnnotationPrefix, m.ObjectMeta.Annotations) &&
		m.ObjectMeta.Annotations[clusterv1.PreTerminateDeleteHookAnnotationPrefix] == k3sHookName {
		if !conditions.IsTrue(m, clusterv1.DrainingSucceededCondition) || !conditions.IsTrue(m, clusterv1.VolumeDetachSucceededCondition) {
			logger.Info("wait for machine drain and detech volume operation complete.")
			return ctrl.Result{}, nil
		}

		cluster, err := util.GetClusterFromMetadata(ctx, r.Client, m.ObjectMeta)
		if err != nil {
			logger.Info("unable to get cluster.")
			return ctrl.Result{}, errors.Wrapf(err, "unable to get cluster")
		}

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
			return ctrl.Result{Requeue: true}, err
		}
		logger.Info("etcd remove etcd member succeeded", "node", m.Status.NodeRef.Name)

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
