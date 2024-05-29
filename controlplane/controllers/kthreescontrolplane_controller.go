/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	k3s "github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/kubeconfig"
	"github.com/k3s-io/cluster-api-k3s/pkg/machinefilters"
	"github.com/k3s-io/cluster-api-k3s/pkg/secret"
	"github.com/k3s-io/cluster-api-k3s/pkg/token"
)

// KThreesControlPlaneReconciler reconciles a KThreesControlPlane object.
type KThreesControlPlaneReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	controller controller.Controller
	recorder   record.EventRecorder

	EtcdDialTimeout time.Duration
	EtcdCallTimeout time.Duration

	managementCluster         k3s.ManagementCluster
	managementClusterUncached k3s.ManagementCluster
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

func (r *KThreesControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("namespace", req.Namespace, "kthreesControlPlane", req.Name)

	// Fetch the KThreesControlPlane instance.
	kcp := &controlplanev1.KThreesControlPlane{}
	if err := r.Client.Get(ctx, req.NamespacedName, kcp); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kcp.ObjectMeta)
	if err != nil {
		logger.Error(err, "Failed to retrieve owner Cluster from the API Server")
		return reconcile.Result{}, err
	}
	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{Requeue: true}, nil
	}
	logger = logger.WithValues("cluster", cluster.Name)

	if annotations.IsPaused(cluster, kcp) {
		logger.Info("Reconciliation is paused for this object")
		return reconcile.Result{}, nil
	}

	// Wait for the cluster infrastructure to be ready before creating machines
	if !cluster.Status.InfrastructureReady {
		return reconcile.Result{}, nil
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(kcp, r.Client)
	if err != nil {
		logger.Error(err, "Failed to configure the patch helper")
		return ctrl.Result{Requeue: true}, nil
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(kcp, controlplanev1.KThreesControlPlaneFinalizer) {
		controllerutil.AddFinalizer(kcp, controlplanev1.KThreesControlPlaneFinalizer)

		// patch and return right away instead of reusing the main defer,
		// because the main defer may take too much time to get cluster status
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		if err := patchHelper.Patch(ctx, kcp, patchOpts...); err != nil {
			logger.Error(err, "Failed to patch KThreesControlPlane to add finalizer")
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	var res ctrl.Result
	if !kcp.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion reconciliation loop.
		res, err = r.reconcileDelete(ctx, cluster, kcp)
	} else {
		// Handle normal reconciliation loop.
		res, err = r.reconcile(ctx, cluster, kcp)
	}

	// Always attempt to update status.
	if updateErr := r.updateStatus(ctx, kcp, cluster); updateErr != nil {
		var connFailure *k3s.RemoteClusterConnectionError
		if errors.As(err, &connFailure) {
			logger.Info("Could not connect to workload cluster to fetch status", "err", updateErr.Error())
		} else {
			logger.Error(err, "Failed to update KThreesControlPlane Status")
			err = kerrors.NewAggregate([]error{err, updateErr})
		}
	}

	// Always attempt to Patch the KThreesControlPlane object and status after each reconciliation.
	if patchErr := patchKThreesControlPlane(ctx, patchHelper, kcp); patchErr != nil {
		logger.Error(err, "Failed to patch KThreesControlPlane")
		err = kerrors.NewAggregate([]error{err, patchErr})
	}

	// Only requeue if there is no error, Requeue or RequeueAfter and the object does not have a deletion timestamp.
	if err == nil && res.IsZero() && kcp.ObjectMeta.DeletionTimestamp.IsZero() {
		// Make KCP requeue in case node status is not ready, so we can check for node status without waiting for a full
		// resync (by default 10 minutes).
		// The alternative solution would be to watch the control plane nodes in the Cluster - similar to how the
		// MachineSet and MachineHealthCheck controllers watch the nodes under their control.
		if !kcp.Status.Ready {
			res = ctrl.Result{RequeueAfter: 20 * time.Second}
		}

		// Make KCP requeue if ControlPlaneComponentsHealthyCondition is false so we can check for control plane component
		// status without waiting for a full resync (by default 10 minutes).
		// Otherwise this condition can lead to a delay in provisioning MachineDeployments when MachineSet preflight checks are enabled.
		// The alternative solution to this requeue would be watching the relevant pods inside each workload cluster which would be very expensive.
		if conditions.IsFalse(kcp, controlplanev1.ControlPlaneComponentsHealthyCondition) {
			res = ctrl.Result{RequeueAfter: 20 * time.Second}
		}
	}

	return res, err
}

// reconcileDelete handles KThreesControlPlane deletion.
// The implementation does not take non-control plane workloads into consideration. This may or may not change in the future.
// Please see https://github.com/kubernetes-sigs/cluster-api/issues/2064.
func (r *KThreesControlPlaneReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster, kcp *controlplanev1.KThreesControlPlane) (ctrl.Result, error) {
	logger := r.Log.WithValues("namespace", kcp.Namespace, "KThreesControlPlane", kcp.Name, "cluster", cluster.Name)
	logger.Info("Reconcile KThreesControlPlane deletion")

	// Gets all machines, not just control plane machines.
	allMachines, err := r.managementCluster.GetMachinesForCluster(ctx, util.ObjectKey(cluster))
	if err != nil {
		return reconcile.Result{}, err
	}
	ownedMachines := allMachines.Filter(collections.OwnedMachines(kcp))

	// If no control plane machines remain, remove the finalizer
	if len(ownedMachines) == 0 {
		controllerutil.RemoveFinalizer(kcp, controlplanev1.KThreesControlPlaneFinalizer)
		return reconcile.Result{}, nil
	}

	controlPlane, err := k3s.NewControlPlane(ctx, r.Client, cluster, kcp, ownedMachines)
	if err != nil {
		logger.Error(err, "failed to initialize control plane")
		return reconcile.Result{}, err
	}

	// Updates conditions reporting the status of static pods and the status of the etcd cluster.
	// NOTE: Ignoring failures given that we are deleting
	if err := r.reconcileControlPlaneConditions(ctx, controlPlane); err != nil {
		logger.Info("failed to reconcile conditions", "error", err.Error())
	}

	// Aggregate the operational state of all the machines; while aggregating we are adding the
	// source ref (reason@machine/name) so the problem can be easily tracked down to its source machine.
	// However, during delete we are hiding the counter (1 of x) because it does not make sense given that
	// all the machines are deleted in parallel.
	conditions.SetAggregate(kcp, controlplanev1.MachinesReadyCondition, ownedMachines.ConditionGetters(), conditions.AddSourceRef(), conditions.WithStepCounterIf(false))

	// Verify that only control plane machines remain
	if len(allMachines) != len(ownedMachines) {
		logger.Info("Waiting for worker nodes to be deleted first")
		conditions.MarkFalse(kcp, controlplanev1.ResizedCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "Waiting for worker nodes to be deleted first")
		return ctrl.Result{RequeueAfter: deleteRequeueAfter}, nil
	}

	// Delete control plane machines in parallel
	machinesToDelete := ownedMachines.Filter(collections.Not(collections.HasDeletionTimestamp))
	var errs []error
	for i := range machinesToDelete {
		m := machinesToDelete[i]
		logger := logger.WithValues("machine", m)
		if err := r.Client.Delete(ctx, machinesToDelete[i]); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to cleanup owned machine")
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		err := kerrors.NewAggregate(errs)
		r.recorder.Eventf(kcp, corev1.EventTypeWarning, "FailedDelete",
			"Failed to delete control plane Machines for cluster %s/%s control plane: %v", cluster.Namespace, cluster.Name, err)
		return reconcile.Result{}, err
	}
	conditions.MarkFalse(kcp, controlplanev1.ResizedCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	return ctrl.Result{RequeueAfter: deleteRequeueAfter}, nil
}

func patchKThreesControlPlane(ctx context.Context, patchHelper *patch.Helper, kcp *controlplanev1.KThreesControlPlane) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	conditions.SetSummary(kcp,
		conditions.WithConditions(
			controlplanev1.MachinesSpecUpToDateCondition,
			controlplanev1.ResizedCondition,
			controlplanev1.MachinesReadyCondition,
			controlplanev1.AvailableCondition,
			controlplanev1.CertificatesAvailableCondition,
			controlplanev1.TokenAvailableCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		kcp,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			controlplanev1.MachinesSpecUpToDateCondition,
			controlplanev1.ResizedCondition,
			controlplanev1.MachinesReadyCondition,
			controlplanev1.AvailableCondition,
			controlplanev1.CertificatesAvailableCondition,
			controlplanev1.TokenAvailableCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
}

func (r *KThreesControlPlaneReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, log *logr.Logger) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1.KThreesControlPlane{}).
		Owns(&clusterv1.Machine{}).
		//	WithOptions(options).
		//	WithEventFilter(predicates.ResourceNotPaused(r.Log)).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	err = c.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(r.ClusterToKThreesControlPlane(ctx, log)),
		predicates.ClusterUnpausedAndInfrastructureReady(r.Log),
	)
	if err != nil {
		return fmt.Errorf("failed adding Watch for Clusters to controller manager: %w", err)
	}

	r.Scheme = mgr.GetScheme()
	r.controller = c
	r.recorder = mgr.GetEventRecorderFor("k3s-control-plane-controller")

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

	return nil
}

// ClusterToKThreesControlPlane is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for KThreesControlPlane based on updates to a Cluster.
func (r *KThreesControlPlaneReconciler) ClusterToKThreesControlPlane(ctx context.Context, log *logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		c, ok := o.(*clusterv1.Cluster)
		if !ok {
			r.Log.Error(nil, fmt.Sprintf("Expected a Cluster but got a %T", o))
			return nil
		}

		controlPlaneRef := c.Spec.ControlPlaneRef
		if controlPlaneRef != nil && controlPlaneRef.Kind == "KThreesControlPlane" {
			return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: controlPlaneRef.Namespace, Name: controlPlaneRef.Name}}}
		}

		return nil
	}
}

// updateStatus is called after every reconcilitation loop in a defer statement to always make sure we have the
// resource status subresourcs up-to-date.
func (r *KThreesControlPlaneReconciler) updateStatus(ctx context.Context, kcp *controlplanev1.KThreesControlPlane, cluster *clusterv1.Cluster) error {
	selector := collections.ControlPlaneSelectorForCluster(cluster.Name)
	// Copy label selector to its status counterpart in string format.
	// This is necessary for CRDs including scale subresources.
	kcp.Status.Selector = selector.String()

	ownedMachines, err := r.managementCluster.GetMachinesForCluster(ctx, util.ObjectKey(cluster), collections.OwnedMachines(kcp))
	if err != nil {
		return fmt.Errorf("failed to get list of owned machines: %w", err)
	}

	logger := r.Log.WithValues("namespace", kcp.Namespace, "KThreesControlPlane", kcp.Name, "cluster", cluster.Name)
	controlPlane, err := k3s.NewControlPlane(ctx, r.Client, cluster, kcp, ownedMachines)
	if err != nil {
		logger.Error(err, "failed to initialize control plane")
		return err
	}
	kcp.Status.UpdatedReplicas = int32(len(controlPlane.UpToDateMachines()))

	replicas := int32(len(ownedMachines))
	desiredReplicas := *kcp.Spec.Replicas

	// set basic data that does not require interacting with the workload cluster
	kcp.Status.Replicas = replicas
	kcp.Status.ReadyReplicas = 0
	kcp.Status.UnavailableReplicas = replicas

	// Return early if the deletion timestamp is set, because we don't want to try to connect to the workload cluster
	// and we don't want to report resize condition (because it is set to deleting into reconcile delete).
	if !kcp.DeletionTimestamp.IsZero() {
		return nil
	}

	machinesWithAgentHealthy := controlPlane.Machines.Filter(machinefilters.AgentHealthy())
	lowestVersion := machinesWithAgentHealthy.LowestVersion()
	if lowestVersion != nil {
		controlPlane.KCP.Status.Version = lowestVersion
	}

	switch {
	// We are scaling up
	case replicas < desiredReplicas:
		conditions.MarkFalse(kcp, controlplanev1.ResizedCondition, controlplanev1.ScalingUpReason, clusterv1.ConditionSeverityWarning, "Scaling up control plane to %d replicas (actual %d)", desiredReplicas, replicas)
	// We are scaling down
	case replicas > desiredReplicas:
		conditions.MarkFalse(kcp, controlplanev1.ResizedCondition, controlplanev1.ScalingDownReason, clusterv1.ConditionSeverityWarning, "Scaling down control plane to %d replicas (actual %d)", desiredReplicas, replicas)
	default:
		// make sure last resize operation is marked as completed.
		// NOTE: we are checking the number of machines ready so we report resize completed only when the machines
		// are actually provisioned (vs reporting completed immediately after the last machine object is created).
		readyMachines := ownedMachines.Filter(collections.IsReady())
		if int32(len(readyMachines)) == replicas {
			conditions.MarkTrue(kcp, controlplanev1.ResizedCondition)
		}
	}

	workloadCluster, err := r.managementCluster.GetWorkloadCluster(ctx, util.ObjectKey(cluster))
	if err != nil {
		return fmt.Errorf("failed to create remote cluster client: %w", err)
	}
	status, err := workloadCluster.ClusterStatus(ctx)
	if err != nil {
		return err
	}

	logger.Info("ClusterStatus", "workload", status)

	kcp.Status.ReadyReplicas = status.ReadyNodes
	kcp.Status.UnavailableReplicas = replicas - status.ReadyNodes

	if status.HasK3sServingSecret {
		kcp.Status.Initialized = true
	}

	if kcp.Status.ReadyReplicas > 0 {
		kcp.Status.Ready = true
		conditions.MarkTrue(kcp, controlplanev1.AvailableCondition)
	}

	// Surface lastRemediation data in status.
	// LastRemediation is the remediation currently in progress, in any, or the
	// most recent of the remediation we are keeping track on machines.
	var lastRemediation *RemediationData

	if v, ok := controlPlane.KCP.Annotations[controlplanev1.RemediationInProgressAnnotation]; ok {
		remediationData, err := RemediationDataFromAnnotation(v)
		if err != nil {
			return err
		}
		lastRemediation = remediationData
	} else {
		for _, m := range controlPlane.Machines.UnsortedList() {
			if v, ok := m.Annotations[controlplanev1.RemediationForAnnotation]; ok {
				remediationData, err := RemediationDataFromAnnotation(v)
				if err != nil {
					return err
				}
				if lastRemediation == nil || lastRemediation.Timestamp.Time.Before(remediationData.Timestamp.Time) {
					lastRemediation = remediationData
				}
			}
		}
	}

	if lastRemediation != nil {
		controlPlane.KCP.Status.LastRemediation = lastRemediation.ToStatus()
	}

	return nil
}

// reconcile handles KThreesControlPlane reconciliation.
func (r *KThreesControlPlaneReconciler) reconcile(ctx context.Context, cluster *clusterv1.Cluster, kcp *controlplanev1.KThreesControlPlane) (ctrl.Result, error) {
	logger := r.Log.WithValues("namespace", kcp.Namespace, "KThreesControlPlane", kcp.Name, "cluster", cluster.Name)
	logger.Info("Reconcile KThreesControlPlane")

	// Make sure to reconcile the external infrastructure reference.
	if err := r.reconcileExternalReference(ctx, cluster, kcp.Spec.MachineTemplate.InfrastructureRef); err != nil {
		return reconcile.Result{}, err
	}

	certificates := secret.NewCertificatesForInitialControlPlane(&kcp.Spec.KThreesConfigSpec)
	controllerRef := metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KThreesControlPlane"))
	if err := certificates.LookupOrGenerate(ctx, r.Client, util.ObjectKey(cluster), *controllerRef); err != nil {
		logger.Error(err, "unable to lookup or create cluster certificates")
		conditions.MarkFalse(kcp, controlplanev1.CertificatesAvailableCondition, controlplanev1.CertificatesGenerationFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		return reconcile.Result{}, err
	}
	conditions.MarkTrue(kcp, controlplanev1.CertificatesAvailableCondition)

	if err := token.Reconcile(ctx, r.Client, client.ObjectKeyFromObject(cluster), kcp); err != nil {
		conditions.MarkFalse(kcp, controlplanev1.TokenAvailableCondition, controlplanev1.TokenGenerationFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		return reconcile.Result{}, err
	}
	conditions.MarkTrue(kcp, controlplanev1.TokenAvailableCondition)

	// If ControlPlaneEndpoint is not set, return early
	if !cluster.Spec.ControlPlaneEndpoint.IsValid() {
		logger.Info("Cluster does not yet have a ControlPlaneEndpoint defined")
		return reconcile.Result{}, nil
	}

	// Generate Cluster Kubeconfig if needed
	if result, err := r.reconcileKubeconfig(ctx, util.ObjectKey(cluster), cluster.Spec.ControlPlaneEndpoint, kcp); err != nil {
		logger.Error(err, "failed to reconcile Kubeconfig")
		return result, err
	}

	controlPlaneMachines, err := r.managementClusterUncached.GetMachinesForCluster(ctx, util.ObjectKey(cluster), collections.ControlPlaneMachines(cluster.Name))
	if err != nil {
		logger.Error(err, "failed to retrieve control plane machines for cluster")
		return reconcile.Result{}, err
	}

	adoptableMachines := controlPlaneMachines.Filter(collections.AdoptableControlPlaneMachines(cluster.Name))
	if len(adoptableMachines) > 0 {
		// We adopt the Machines and then wait for the update event for the ownership reference to re-queue them so the cache is up-to-date
		// err = r.adoptMachines(ctx, kcp, adoptableMachines, cluster)
		return reconcile.Result{}, err
	}

	ownedMachines := controlPlaneMachines.Filter(collections.OwnedMachines(kcp))
	if len(ownedMachines) != len(controlPlaneMachines) {
		logger.Info("Not all control plane machines are owned by this KThreesControlPlane, refusing to operate in mixed management mode")
		return reconcile.Result{}, nil
	}

	controlPlane, err := k3s.NewControlPlane(ctx, r.Client, cluster, kcp, ownedMachines)
	if err != nil {
		logger.Error(err, "failed to initialize control plane")
		return reconcile.Result{}, err
	}

	// Aggregate the operational state of all the machines; while aggregating we are adding the
	// source ref (reason@machine/name) so the problem can be easily tracked down to its source machine.
	conditions.SetAggregate(controlPlane.KCP, controlplanev1.MachinesReadyCondition, ownedMachines.ConditionGetters(), conditions.AddSourceRef(), conditions.WithStepCounterIf(false))

	// Updates conditions reporting the status of static pods and the status of the etcd cluster.
	// NOTE: Conditions reporting KCP operation progress like e.g. Resized or SpecUpToDate are inlined with the rest of the execution.
	if err := r.reconcileControlPlaneConditions(ctx, controlPlane); err != nil {
		return reconcile.Result{}, err
	}

	// Ensures the number of etcd members is in sync with the number of machines/nodes.
	// NOTE: This is usually required after a machine deletion.
	if err := r.reconcileEtcdMembers(ctx, controlPlane); err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile unhealthy machines by triggering deletion and requeue if it is considered safe to remediate,
	// otherwise continue with the other KCP operations.
	if result, err := r.reconcileUnhealthyMachines(ctx, controlPlane); err != nil || !result.IsZero() {
		return result, err
	}

	// Control plane machines rollout due to configuration changes (e.g. upgrades) takes precedence over other operations.
	needRollout := controlPlane.MachinesNeedingRollout()
	switch {
	case len(needRollout) > 0:
		logger.Info("Rolling out Control Plane machines", "needRollout", needRollout.Names())
		conditions.MarkFalse(controlPlane.KCP, controlplanev1.MachinesSpecUpToDateCondition, controlplanev1.RollingUpdateInProgressReason, clusterv1.ConditionSeverityWarning, "Rolling %d replicas with outdated spec (%d replicas up to date)", len(needRollout), len(controlPlane.Machines)-len(needRollout))
		return r.upgradeControlPlane(ctx, cluster, kcp, controlPlane, needRollout)
	default:
		// make sure last upgrade operation is marked as completed.
		// NOTE: we are checking the condition already exists in order to avoid to set this condition at the first
		// reconciliation/before a rolling upgrade actually starts.
		if conditions.Has(controlPlane.KCP, controlplanev1.MachinesSpecUpToDateCondition) {
			conditions.MarkTrue(controlPlane.KCP, controlplanev1.MachinesSpecUpToDateCondition)
		}
	}

	// If we've made it this far, we can assume that all ownedMachines are up to date
	numMachines := len(ownedMachines)
	desiredReplicas := int(*kcp.Spec.Replicas)

	switch {
	// We are creating the first replica
	case numMachines < desiredReplicas && numMachines == 0:
		// Create new Machine w/ init
		logger.Info("Initializing control plane", "Desired", desiredReplicas, "Existing", numMachines)
		conditions.MarkFalse(controlPlane.KCP, controlplanev1.AvailableCondition, controlplanev1.WaitingForKthreesServerReason, clusterv1.ConditionSeverityInfo, "")
		return r.initializeControlPlane(ctx, cluster, kcp, controlPlane)
	// We are scaling up
	case numMachines < desiredReplicas && numMachines > 0:
		// Create a new Machine w/ join
		logger.Info("Scaling up control plane", "Desired", desiredReplicas, "Existing", numMachines)
		return r.scaleUpControlPlane(ctx, cluster, kcp, controlPlane)
	// We are scaling down
	case numMachines > desiredReplicas:
		logger.Info("Scaling down control plane", "Desired", desiredReplicas, "Existing", numMachines)
		// The last parameter (i.e. machines needing to be rolled out) should always be empty here.
		return r.scaleDownControlPlane(ctx, cluster, kcp, controlPlane, collections.Machines{})
	}

	// Get the workload cluster client.
	/**
	workloadCluster, err := r.managementCluster.GetWorkloadCluster(ctx, util.ObjectKey(cluster))
	if err != nil {
		logger.V(2).Info("cannot get remote client to workload cluster, will requeue", "cause", err)
		return ctrl.Result{Requeue: true}, nil
	}

	// Update kube-proxy daemonset.
	if err := workloadCluster.UpdateKubeProxyImageInfo(ctx, kcp); err != nil {
		logger.Error(err, "failed to update kube-proxy daemonset")
		return reconcile.Result{}, err
	}

	// Update CoreDNS deployment.
	if err := workloadCluster.UpdateCoreDNS(ctx, kcp); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to update CoreDNS deployment")
	}
	**/

	return reconcile.Result{}, nil
}

func (r *KThreesControlPlaneReconciler) reconcileExternalReference(ctx context.Context, cluster *clusterv1.Cluster, ref corev1.ObjectReference) error {
	if !strings.HasSuffix(ref.Kind, clusterv1.TemplateSuffix) {
		return nil
	}

	obj, err := external.Get(ctx, r.Client, &ref, cluster.Namespace)
	if err != nil {
		return err
	}

	// Note: We intentionally do not handle checking for the paused label on an external template reference

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return err
	}

	obj.SetOwnerReferences(util.EnsureOwnerRef(obj.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}))

	return patchHelper.Patch(ctx, obj)
}

func (r *KThreesControlPlaneReconciler) reconcileKubeconfig(ctx context.Context, clusterName client.ObjectKey, endpoint clusterv1.APIEndpoint, kcp *controlplanev1.KThreesControlPlane) (ctrl.Result, error) {
	if endpoint.IsZero() {
		return reconcile.Result{}, nil
	}

	controllerOwnerRef := *metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KThreesControlPlane"))
	configSecret, err := secret.GetFromNamespacedName(ctx, r.Client, clusterName, secret.Kubeconfig)
	switch {
	case apierrors.IsNotFound(err):
		createErr := kubeconfig.CreateSecretWithOwner(
			ctx,
			r.Client,
			clusterName,
			endpoint.String(),
			controllerOwnerRef,
		)
		if errors.Is(createErr, kubeconfig.ErrDependentCertificateNotFound) {
			return ctrl.Result{RequeueAfter: dependentCertRequeueAfter}, nil
		}
		// always return if we have just created in order to skip rotation checks
		return reconcile.Result{}, createErr

	case err != nil:
		return reconcile.Result{}, fmt.Errorf("failed to retrieve kubeconfig Secret: %w", err)
	}

	// only do rotation on owned secrets
	if !util.IsControlledBy(configSecret, kcp) {
		return reconcile.Result{}, nil
	}

	/**
	// TODO rotation
	needsRotation, err := kubeconfig.NeedsClientCertRotation(configSecret, certs.ClientCertificateRenewalDuration)
	if err != nil {
		return err
	}

	if needsRotation {
		r.Log.Info("rotating kubeconfig secret")
		if err := kubeconfig.RegenerateSecret(ctx, r.Client, configSecret); err != nil {
			return fmt.Errorf("failed to regenerate kubeconfig")
		}
	}
	**/

	return reconcile.Result{}, nil
}

// reconcileControlPlaneConditions is responsible of reconciling conditions reporting the status of static pods and
// the status of the etcd cluster.
func (r *KThreesControlPlaneReconciler) reconcileControlPlaneConditions(ctx context.Context, controlPlane *k3s.ControlPlane) error {
	// If the cluster is not yet initialized, there is no way to connect to the workload cluster and fetch information
	// for updating conditions. Return early.
	if !controlPlane.KCP.Status.Initialized {
		return nil
	}

	workloadCluster, err := r.managementCluster.GetWorkloadCluster(ctx, util.ObjectKey(controlPlane.Cluster))
	if err != nil {
		return fmt.Errorf("cannot get remote client to workload cluster: %w", err)
	}

	// Update conditions status
	workloadCluster.UpdateAgentConditions(ctx, controlPlane)
	workloadCluster.UpdateEtcdConditions(ctx, controlPlane)

	// Patch machines with the updated conditions.
	if err := controlPlane.PatchMachines(ctx); err != nil {
		return err
	}

	// KCP will be patched at the end of Reconcile to reflect updated conditions, so we can return now.
	return nil
}

// reconcileEtcdMembers ensures the number of etcd members is in sync with the number of machines/nodes.
// This is usually required after a machine deletion.
//
// NOTE: this func uses KCP conditions, it is required to call reconcileControlPlaneConditions before this.
func (r *KThreesControlPlaneReconciler) reconcileEtcdMembers(ctx context.Context, controlPlane *k3s.ControlPlane) error {
	log := ctrl.LoggerFrom(ctx)

	// If etcd is not managed by KCP this is a no-op.
	if !controlPlane.IsEtcdManaged() {
		return nil
	}

	// If there is no KCP-owned control-plane machines, then control-plane has not been initialized yet.
	if controlPlane.Machines.Len() == 0 {
		return nil
	}

	// Collect all the node names.
	nodeNames := []string{}
	for _, machine := range controlPlane.Machines {
		if machine.Status.NodeRef == nil {
			// If there are provisioning machines (machines without a node yet), return.
			return nil
		}
		nodeNames = append(nodeNames, machine.Status.NodeRef.Name)
	}

	// Potential inconsistencies between the list of members and the list of machines/nodes are
	// surfaced using the EtcdClusterHealthyCondition; if this condition is true, meaning no inconsistencies exists, return early.
	if conditions.IsTrue(controlPlane.KCP, controlplanev1.EtcdClusterHealthyCondition) {
		return nil
	}

	workloadCluster, err := r.managementCluster.GetWorkloadCluster(ctx, util.ObjectKey(controlPlane.Cluster))
	if err != nil {
		// Failing at connecting to the workload cluster can mean workload cluster is unhealthy for a variety of reasons such as etcd quorum loss.
		return errors.Wrap(err, "cannot get remote client to workload cluster")
	}

	removedMembers, err := workloadCluster.ReconcileEtcdMembers(ctx, nodeNames)
	if err != nil {
		return errors.Wrap(err, "failed attempt to reconcile etcd members")
	}

	if len(removedMembers) > 0 {
		log.Info("Etcd members without nodes removed from the cluster", "members", removedMembers)
	}

	return nil
}

func (r *KThreesControlPlaneReconciler) upgradeControlPlane(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	kcp *controlplanev1.KThreesControlPlane,
	controlPlane *k3s.ControlPlane,
	machinesRequireUpgrade collections.Machines,
) (ctrl.Result, error) {
	// TODO: handle reconciliation of etcd members and kubeadm config in case they get out of sync with cluster

	/**
	logger := controlPlane.Logger()
	workloadCluster, err := r.managementCluster.GetWorkloadCluster(ctx, util.ObjectKey(cluster))

	if err != nil {
		logger.Error(err, "failed to get remote client for workload cluster", "cluster key", util.ObjectKey(cluster))
		return reconcile.Result{}, err
	}

	parsedVersion, err := semver.ParseTolerant(kcp.Spec.Version)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf(err, "failed to parse kubernetes version %q", kcp.Spec.Version)
	}


	if kcp.Spec.KThreesConfigSpec.ClusterConfiguration != nil {
		imageRepository := kcp.Spec.KThreesConfigSpec.ClusterConfiguration.ImageRepository
		if err := workloadCluster.UpdateImageRepositoryInKubeadmConfigMap(ctx, imageRepository); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to update the image repository in the kubeadm config map")
		}
	}

	if kcp.Spec.KThreesConfigSpec.ClusterConfiguration != nil && kcp.Spec.KThreesConfigSpec.ClusterConfiguration.Etcd.Local != nil {
		meta := kcp.Spec.KThreesConfigSpec.ClusterConfiguration.Etcd.Local.ImageMeta
		if err := workloadCluster.UpdateEtcdVersionInKubeadmConfigMap(ctx, meta.ImageRepository, meta.ImageTag); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to update the etcd version in the kubeadm config map")
		}
	}

	if err := workloadCluster.UpdateKubeletConfigMap(ctx, parsedVersion); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to upgrade kubelet config map")
	}
	**/

	if controlPlane.Machines.Len() <= int(*kcp.Spec.Replicas) {
		// scaleUp ensures that we don't continue scaling up while waiting for Machines to have NodeRefs
		return r.scaleUpControlPlane(ctx, cluster, kcp, controlPlane)
	}
	return r.scaleDownControlPlane(ctx, cluster, kcp, controlPlane, machinesRequireUpgrade)
}
