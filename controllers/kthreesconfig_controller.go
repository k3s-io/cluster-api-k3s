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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bsutil "sigs.k8s.io/cluster-api/bootstrap/util"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1alpha1 "github.com/zawachte-msft/cluster-api-bootstrap-provider-k3s/api/v1alpha1"
)

// InitLocker is a lock that is used around kubeadm init
type InitLocker interface {
	Lock(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) bool
	Unlock(ctx context.Context, cluster *clusterv1.Cluster) bool
}

// KThreesConfigReconciler reconciles a KThreesConfig object
type KThreesConfigReconciler struct {
	client.Client
	Log             logr.Logger
	KThreesInitLock InitLocker
	Scheme          *runtime.Scheme
}

type Scope struct {
	logr.Logger
	Config      *bootstrapv1alpha1.KThreesConfig
	ConfigOwner *bsutil.ConfigOwner
	Cluster     *clusterv1.Cluster
}

// +kubebuilder:rbac:groups=bootstrap.k8s.io,resources=kthreesconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bootstrap.k8s.io,resources=kthreesconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status;machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=exp.cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;events;configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *KThreesConfigReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	log := r.Log.WithValues("kthreesconfig", req.NamespacedName)

	// Lookup the kubeadm config
	config := &bootstrapv1alpha1.KThreesConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get config")
		return ctrl.Result{}, err
	}

	// Look up the owner of this KubeConfig if there is one
	configOwner, err := bsutil.GetConfigOwner(ctx, r.Client, config)
	if apierrors.IsNotFound(errors.Cause(err)) {
		// Could not find the owner yet, this is not an error and will rereconcile when the owner gets set.
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, "Failed to get owner")
		return ctrl.Result{}, err
	}
	if configOwner == nil {
		return ctrl.Result{}, nil
	}
	log = log.WithValues("kind", configOwner.GetKind(), "version", configOwner.GetResourceVersion(), "name", configOwner.GetName())

	// Lookup the cluster the config owner is associated with
	cluster, err := util.GetClusterByName(ctx, r.Client, configOwner.GetNamespace(), configOwner.ClusterName())
	if err != nil {
		if errors.Cause(err) == util.ErrNoCluster {
			log.Info(fmt.Sprintf("%s does not belong to a cluster yet, waiting until it's part of a cluster", configOwner.GetKind()))
			return ctrl.Result{}, nil
		}

		if apierrors.IsNotFound(errors.Cause(err)) {
			log.Info("Cluster does not exist yet, waiting until it is created")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Could not get cluster with metadata")
		return ctrl.Result{}, err
	}

	if annotations.IsPaused(cluster, config) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	scope := &Scope{
		Logger:      log,
		Config:      config,
		ConfigOwner: configOwner,
		Cluster:     cluster,
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(config, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Attempt to Patch the KubeadmConfig object and status after each reconciliation if no error occurs.
	defer func() {
		// always update the readyCondition; the summary is represented using the "1 of x completed" notation.

		/**
		conditions.SetSummary(config,
			conditions.WithConditions(
				bootstrapv1.DataSecretAvailableCondition,
				bootstrapv1.CertificatesAvailableCondition,
			),
		)
		**/
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		if rerr == nil {
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		}
		if err := patchHelper.Patch(ctx, config, patchOpts...); err != nil {
			log.Error(rerr, "Failed to patch config")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	switch {
	// Wait for the infrastructure to be ready.
	case !cluster.Status.InfrastructureReady:
		log.Info("Cluster infrastructure is not ready, waiting")
		//conditions.MarkFalse(config, bootstrapv1.DataSecretAvailableCondition, bootstrapv1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	// Migrate plaintext data to secret.
	case config.Status.BootstrapData != nil && config.Status.DataSecretName == nil:
		return ctrl.Result{}, r.storeBootstrapData(ctx, scope, config.Status.BootstrapData)
	// Reconcile status for machines that already have a secret reference, but our status isn't up to date.
	// This case solves the pivoting scenario (or a backup restore) which doesn't preserve the status subresource on objects.
	case configOwner.DataSecretName() != nil && (!config.Status.Ready || config.Status.DataSecretName == nil):
		config.Status.Ready = true
		config.Status.DataSecretName = configOwner.DataSecretName()
		//conditions.MarkTrue(config, bootstrapv1.DataSecretAvailableCondition)
		return ctrl.Result{}, nil
	// Status is ready means a config has been generated.
	case config.Status.Ready:
		// If the BootstrapToken has been generated for a join and the infrastructure is not ready.
		// This indicates the token in the join config has not been consumed and it may need a refresh.
		/**
		if config.Spec.JoinConfiguration != nil && config.Spec.JoinConfiguration.Discovery.BootstrapToken != nil {
			if !configOwner.IsInfrastructureReady() {
				return r.refreshBootstrapToken(ctx, log, cluster, config)
			} else if (config.Spec.JoinConfiguration != nil && config.Spec.JoinConfiguration.Discovery.BootstrapToken != nil) && configOwner.IsMachinePool() {
				// If the BootstrapToken has been generated and infrastructure is ready but the configOwner is a MachinePool,
				// we rotate the token to keep it fresh for future scale ups.
				return r.rotateMachinePoolBootstrapToken(ctx, log, cluster, config, scope)
			}
		}
		**/
		// In any other case just return as the config is already generated and need not be generated again.
		return ctrl.Result{}, nil
	}

	if !cluster.Status.ControlPlaneInitialized {
		return r.handleClusterNotInitialized(ctx, scope)
	}

	/**
	// Every other case it's a join scenario
	// Nb. in this case ClusterConfiguration and InitConfiguration should not be defined by users, but in case of misconfigurations, CABPK simply ignore them

	// Unlock any locks that might have been set during init process
	r.KubeadmInitLock.Unlock(ctx, cluster)

	// if the JoinConfiguration is missing, create a default one
	if config.Spec.JoinConfiguration == nil {
		log.Info("Creating default JoinConfiguration")
		config.Spec.JoinConfiguration = &kubeadmv1beta1.JoinConfiguration{}
	}

	// it's a control plane join
	if configOwner.IsControlPlaneMachine() {
		return r.joinControlplane(ctx, scope)
	}


	// It's a worker join
	return r.joinWorker(ctx, scope)

	**/

	return ctrl.Result{}, nil
}

func (r *KThreesConfigReconciler) handleClusterNotInitialized(ctx context.Context, scope *Scope) (_ ctrl.Result, reterr error) {
	// initialize the DataSecretAvailableCondition if missing.
	// this is required in order to avoid the condition's LastTransitionTime to flicker in case of errors surfacing
	// using the DataSecretGeneratedFailedReason
	//if conditions.GetReason(scope.Config, bootstrapv1.DataSecretAvailableCondition) != bootstrapv1.DataSecretGenerationFailedReason {
	//	conditions.MarkFalse(scope.Config, bootstrapv1.DataSecretAvailableCondition, clusterv1.WaitingForControlPlaneAvailableReason, clusterv1.ConditionSeverityInfo, "")
	//}

	// if it's NOT a control plane machine, requeue
	if !scope.ConfigOwner.IsControlPlaneMachine() {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	machine := &clusterv1.Machine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(scope.ConfigOwner.Object, machine); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "cannot convert %s to Machine", scope.ConfigOwner.GetKind())
	}

	// acquire the init lock so that only the first machine configured
	// as control plane get processed here
	// if not the first, requeue
	if !r.KThreesInitLock.Lock(ctx, scope.Cluster, machine) {
		scope.Info("A control plane is already being initialized, requeing until control plane is ready")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	defer func() {
		if reterr != nil {
			if !r.KThreesInitLock.Unlock(ctx, scope.Cluster) {
				reterr = kerrors.NewAggregate([]error{reterr, errors.New("failed to unlock the kubeadm init lock")})
			}
		}
	}()

	scope.Info("Creating BootstrapData for the init control plane")

	/**
		certificates := secret.NewCertificatesForInitialControlPlane(scope.Config.Spec.ClusterConfiguration)
		err = certificates.LookupOrGenerate(
			ctx,
			r.Client,
			util.ObjectKey(scope.Cluster),
			*metav1.NewControllerRef(scope.Config, bootstrapv1.GroupVersion.WithKind("KThreesConfig")),
		)
		if err != nil {
			//	conditions.MarkFalse(scope.Config, bootstrapv1.CertificatesAvailableCondition, bootstrapv1.CertificatesGenerationFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return ctrl.Result{}, err
		}
		//conditions.MarkTrue(scope.Config, bootstrapv1.CertificatesAvailableCondition)
	**/

	cloudInitData := []byte{}
	if err := r.storeBootstrapData(ctx, scope, cloudInitData); err != nil {
		scope.Error(err, "Failed to store bootstrap data")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KThreesConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bootstrapv1alpha1.KThreesConfig{}).
		Complete(r)
}

// storeBootstrapData creates a new secret with the data passed in as input,
// sets the reference in the configuration status and ready to true.
func (r *KThreesConfigReconciler) storeBootstrapData(ctx context.Context, scope *Scope, data []byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scope.Config.Name,
			Namespace: scope.Config.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: scope.Cluster.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: bootstrapv1alpha1.GroupVersion.String(),
					Kind:       "KThreesConfig",
					Name:       scope.Config.Name,
					UID:        scope.Config.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Data: map[string][]byte{
			"value": data,
		},
		Type: clusterv1.ClusterSecretType,
	}

	// as secret creation and scope.Config status patch are not atomic operations
	// it is possible that secret creation happens but the config.Status patches are not applied
	if err := r.Client.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create bootstrap data secret for KubeadmConfig %s/%s", scope.Config.Namespace, scope.Config.Name)
		}
		r.Log.Info("bootstrap data secret for KubeadmConfig already exists, updating", "secret", secret.Name, "KubeadmConfig", scope.Config.Name)
		if err := r.Client.Update(ctx, secret); err != nil {
			return errors.Wrapf(err, "failed to update bootstrap data secret for KubeadmConfig %s/%s", scope.Config.Namespace, scope.Config.Name)
		}
	}
	scope.Config.Status.DataSecretName = pointer.StringPtr(secret.Name)
	scope.Config.Status.Ready = true
	//	conditions.MarkTrue(scope.Config, bootstrapv1.DataSecretAvailableCondition)
	return nil
}
