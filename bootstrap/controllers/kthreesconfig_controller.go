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
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	bsutil "sigs.k8s.io/cluster-api/bootstrap/util"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	kubeyaml "sigs.k8s.io/yaml"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta3"
	"github.com/k3s-io/cluster-api-k3s/pkg/cloudinit"
	"github.com/k3s-io/cluster-api-k3s/pkg/etcd"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/locking"
	"github.com/k3s-io/cluster-api-k3s/pkg/secret"
	"github.com/k3s-io/cluster-api-k3s/pkg/token"
)

// InitLocker is a lock that is used around k3s init.
type InitLocker interface {
	Lock(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) bool
	Unlock(ctx context.Context, cluster *clusterv1.Cluster) bool
}

// KThreesConfigReconciler reconciles a KThreesConfig object.
type KThreesConfigReconciler struct {
	client.Client
	Log             logr.Logger
	KThreesInitLock InitLocker
	Scheme          *runtime.Scheme
}

type Scope struct {
	logr.Logger
	Config      *bootstrapv1.KThreesConfig
	ConfigOwner *bsutil.ConfigOwner
	Cluster     *clusterv1.Cluster
}

var (
	ErrInvalidRef   = errors.New("invalid reference")
	ErrFailedUnlock = errors.New("failed to unlock the k3s init lock")
)

// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kthreesconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kthreesconfigs/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status;machines;machines/status;machinepools;machinepools/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;events;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

func (r *KThreesConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, rerr error) {
	log := r.Log.WithValues("kthreesconfig", req.NamespacedName)

	// Lookup the k3s config
	config := &bootstrapv1.KThreesConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get config")
		return ctrl.Result{}, err
	}

	// Look up the owner of this KubeConfig if there is one
	configOwner, err := bsutil.GetConfigOwner(ctx, r.Client, config)
	if apierrors.IsNotFound(err) {
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
		if errors.Is(err, util.ErrNoCluster) {
			log.Info(fmt.Sprintf("%s does not belong to a cluster yet, waiting until it's part of a cluster", configOwner.GetKind()))
			return ctrl.Result{}, nil
		}

		if apierrors.IsNotFound(err) {
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

	// Attempt to Patch the KThreesConfig object and status after each reconciliation if no error occurs.
	defer func() {
		// always update the readyCondition; the summary is represented using the "1 of x completed" notation.

		v1beta1conditions.SetSummary(config,
			v1beta1conditions.WithConditions(
				bootstrapv1.DataSecretAvailableV1Beta1Condition,
				bootstrapv1.CertificatesAvailableV1Beta1Condition,
			),
		)
		if err := conditions.SetSummaryCondition(config, config, bootstrapv1.KThreesConfigReadyCondition,
			conditions.ForConditionTypes{
				bootstrapv1.KThreesConfigDataSecretAvailableCondition,
				bootstrapv1.KThreesConfigCertificatesAvailableCondition,
			},
			// Using a custom merge strategy to override reasons applied during merge and to ignore some
			// info message so the ready condition aggregation in other resources is less noisy.
			conditions.CustomMergeStrategy{
				MergeStrategy: conditions.DefaultMergeStrategy(
					// Use custom reasons.
					conditions.ComputeReasonFunc(conditions.GetDefaultComputeMergeReasonFunc(
						bootstrapv1.KThreesConfigNotReadyReason,
						bootstrapv1.KThreesConfigReadyUnknownReason,
						bootstrapv1.KThreesConfigReadyReason,
					)),
				),
			},
		); err != nil {
			rerr = kerrors.NewAggregate([]error{rerr, err})
		}

		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{
			patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
				clusterv1.ReadyV1Beta1Condition,
				bootstrapv1.DataSecretAvailableV1Beta1Condition,
				bootstrapv1.CertificatesAvailableV1Beta1Condition,
			}},
			patch.WithOwnedConditions{Conditions: []string{
				bootstrapv1.KThreesConfigReadyCondition,
				bootstrapv1.KThreesConfigDataSecretAvailableCondition,
				bootstrapv1.KThreesConfigCertificatesAvailableCondition,
			}},
		}
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
	case !ptr.Deref(cluster.Status.Initialization.InfrastructureProvisioned, false):
		log.Info("Cluster infrastructure is not ready, waiting")
		v1beta1conditions.MarkFalse(config, bootstrapv1.DataSecretAvailableV1Beta1Condition, bootstrapv1.WaitingForClusterInfrastructureV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
		conditions.Set(config, metav1.Condition{
			Type:    bootstrapv1.KThreesConfigDataSecretAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  bootstrapv1.KThreesConfigDataSecretNotAvailableReason,
			Message: "Waiting for Cluster status.initialization.infrastructureProvisioned to be true",
		})
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	// Reconcile status for machines that already have a secret reference, but our status isn't up to date.
	// This case solves the pivoting scenario (or a backup restore) which doesn't preserve the status subresource on objects.
	case configOwner.DataSecretName() != nil && (!ptr.Deref(config.Status.Initialization.DataSecretCreated, false) || config.Status.DataSecretName == ""):
		config.Status.Initialization.DataSecretCreated = ptr.To(true)
		config.Status.DataSecretName = *configOwner.DataSecretName()
		v1beta1conditions.MarkTrue(config, bootstrapv1.DataSecretAvailableV1Beta1Condition)
		markDataSecretAndCertificatesAvailable(config)
		return ctrl.Result{}, nil
	// DataSecretCreated means a config has been generated.
	// This also solves the upgrade scenario to a version which includes v1beta3 to ensure the new conditions are properly set.
	case ptr.Deref(config.Status.Initialization.DataSecretCreated, false):
		// Based on existing code paths status.initialization.dataSecretCreated is only true if status.dataSecretName is set,
		// and generating the data secret requires certificates to be available.
		v1beta1conditions.MarkTrue(config, bootstrapv1.DataSecretAvailableV1Beta1Condition)
		markDataSecretAndCertificatesAvailable(config)
		// In any other case just return as the config is already generated and need not be generated again.
		return ctrl.Result{}, nil
	}

	if !ptr.Deref(cluster.Status.Initialization.ControlPlaneInitialized, false) {
		return r.handleClusterNotInitialized(ctx, scope)
	}

	// Every other case it's a join scenario
	// Nb. in this case ClusterConfiguration and InitConfiguration should not be defined by users, but in case of misconfigurations, CABPK simply ignore them

	// Unlock any locks that might have been set during init process
	r.KThreesInitLock.Unlock(ctx, cluster)

	// it's a control plane join
	if configOwner.IsControlPlaneMachine() {
		return reconcile.Result{}, r.joinControlplane(ctx, scope)
	}

	// It's a worker join
	return reconcile.Result{}, r.joinWorker(ctx, scope)
}

func (r *KThreesConfigReconciler) joinControlplane(ctx context.Context, scope *Scope) error {
	machine := &clusterv1.Machine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(scope.ConfigOwner.Object, machine); err != nil {
		return fmt.Errorf("cannot convert %s to Machine: %w", scope.ConfigOwner.GetKind(), err)
	}

	// injects into config.Version values from top level object
	r.reconcileTopLevelObjectSettings(scope.Cluster, machine, scope.Config)

	serverURL := fmt.Sprintf("https://%s", scope.Cluster.Spec.ControlPlaneEndpoint.String())

	tokn, err := token.Lookup(ctx, r.Client, client.ObjectKeyFromObject(scope.Cluster))
	if err != nil {
		markDataSecretGenerationFailed(scope.Config, err)
		return err
	}

	configStruct := k3s.GenerateJoinControlPlaneConfig(serverURL, *tokn,
		scope.Cluster.Spec.ControlPlaneEndpoint.Host,
		scope.Config.Spec.ServerConfig,
		scope.Config.Spec.AgentConfig)
	b, err := kubeyaml.Marshal(configStruct)
	if err != nil {
		return err
	}

	workerConfigFile := bootstrapv1.File{
		Path:        k3s.DefaultK3sConfigLocation,
		Content:     string(b),
		Owner:       "root:root",
		Permissions: "0640",
	}

	files, err := r.resolveFiles(ctx, scope.Config)
	if err != nil {
		markDataSecretGenerationFailed(scope.Config, err)
		return err
	}

	if scope.Config.Spec.IsEtcdEmbedded() {
		etcdProxyFile, err := r.resolveEtcdProxyFile(scope.Config)
		if err != nil {
			markDataSecretGenerationFailed(scope.Config, err)
			return fmt.Errorf("failed to resolve etcd proxy file: %w", err)
		}

		files = append(files, *etcdProxyFile)
	}

	cpInput := &cloudinit.ControlPlaneInput{
		BaseUserData: cloudinit.BaseUserData{
			PreK3sCommands:             scope.Config.Spec.PreK3sCommands,
			PostK3sCommands:            scope.Config.Spec.PostK3sCommands,
			AdditionalFiles:            files,
			ConfigFile:                 workerConfigFile,
			K3sVersion:                 scope.Config.Spec.Version,
			AirGapped:                  scope.Config.Spec.AgentConfig.AirGapped,
			AirGappedInstallScriptPath: scope.Config.Spec.AgentConfig.AirGappedInstallScriptPath,
		},
	}

	cloudInitData, err := cloudinit.NewJoinControlPlane(cpInput)
	if err != nil {
		return err
	}

	if err := r.storeBootstrapData(ctx, scope, cloudInitData); err != nil {
		scope.Error(err, "Failed to store bootstrap data")
		return err
	}
	return nil
}

func (r *KThreesConfigReconciler) joinWorker(ctx context.Context, scope *Scope) error {
	machine := &clusterv1.Machine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(scope.ConfigOwner.Object, machine); err != nil {
		return fmt.Errorf("cannot convert %s to Machine: %w", scope.ConfigOwner.GetKind(), err)
	}

	// injects into config.Version values from top level object
	r.reconcileTopLevelObjectSettings(scope.Cluster, machine, scope.Config)

	serverURL := fmt.Sprintf("https://%s", scope.Cluster.Spec.ControlPlaneEndpoint.String())

	tokn, err := token.Lookup(ctx, r.Client, client.ObjectKeyFromObject(scope.Cluster))
	if err != nil {
		markDataSecretGenerationFailed(scope.Config, err)
		return err
	}

	configStruct := k3s.GenerateWorkerConfig(serverURL, *tokn, scope.Config.Spec.ServerConfig, scope.Config.Spec.AgentConfig)

	b, err := kubeyaml.Marshal(configStruct)
	if err != nil {
		return err
	}

	workerConfigFile := bootstrapv1.File{
		Path:        k3s.DefaultK3sConfigLocation,
		Content:     string(b),
		Owner:       "root:root",
		Permissions: "0640",
	}

	files, err := r.resolveFiles(ctx, scope.Config)
	if err != nil {
		markDataSecretGenerationFailed(scope.Config, err)
		return err
	}

	winput := &cloudinit.WorkerInput{
		BaseUserData: cloudinit.BaseUserData{
			PreK3sCommands:             scope.Config.Spec.PreK3sCommands,
			PostK3sCommands:            scope.Config.Spec.PostK3sCommands,
			AdditionalFiles:            files,
			ConfigFile:                 workerConfigFile,
			K3sVersion:                 scope.Config.Spec.Version,
			AirGapped:                  scope.Config.Spec.AgentConfig.AirGapped,
			AirGappedInstallScriptPath: scope.Config.Spec.AgentConfig.AirGappedInstallScriptPath,
		},
	}

	cloudInitData, err := cloudinit.NewWorker(winput)
	if err != nil {
		return err
	}

	if err := r.storeBootstrapData(ctx, scope, cloudInitData); err != nil {
		scope.Error(err, "Failed to store bootstrap data")
		return err
	}

	return nil
}

// resolveFiles maps .Spec.Files into cloudinit.Files, resolving any object references
// along the way.
func (r *KThreesConfigReconciler) resolveFiles(ctx context.Context, cfg *bootstrapv1.KThreesConfig) ([]bootstrapv1.File, error) {
	collected := make([]bootstrapv1.File, 0, len(cfg.Spec.Files))

	for i := range cfg.Spec.Files {
		in := cfg.Spec.Files[i]
		if in.ContentFrom != nil {
			data, err := r.resolveSecretFileContent(ctx, cfg.Namespace, in)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve file source: %w", err)
			}
			in.ContentFrom = nil
			in.Content = string(data)
		}
		collected = append(collected, in)
	}

	return collected, nil
}

// resolveSecretFileContent returns file content fetched from a referenced secret object.
func (r *KThreesConfigReconciler) resolveSecretFileContent(ctx context.Context, ns string, source bootstrapv1.File) ([]byte, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: ns, Name: source.ContentFrom.Secret.Name}
	if err := r.Client.Get(ctx, key, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("secret not found %s: %w", key, err)
		}
		return nil, fmt.Errorf("failed to retrieve Secret %q: %w", key, err)
	}
	data, ok := secret.Data[source.ContentFrom.Secret.Key]
	if !ok {
		return nil, fmt.Errorf("secret references non-existent secret key %q: %w", source.ContentFrom.Secret.Key, ErrInvalidRef)
	}
	return data, nil
}

func (r *KThreesConfigReconciler) resolveEtcdProxyFile(cfg *bootstrapv1.KThreesConfig) (*bootstrapv1.File, error) {
	// Parse the template
	tpl, err := template.New("etcd-proxy").Parse(etcd.EtcdProxyDaemonsetYamlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse etcd-proxy template: %w", err)
	}

	// If user has set the systemDefaultRegistry, will prefix the image with it.
	systemDefaultRegistry := cfg.Spec.ServerConfig.SystemDefaultRegistry
	if systemDefaultRegistry != "" {
		systemDefaultRegistry = fmt.Sprintf("%s/", systemDefaultRegistry)
	}

	// Render the template, the image name will be ${EtcdProxyImage} if the user
	// has set it, otherwise it will be ${SystemDefaultRegistry}alpine/socat
	var buf bytes.Buffer
	err = tpl.Execute(&buf, map[string]string{
		"EtcdProxyImage":        cfg.Spec.ServerConfig.EtcdProxyImage,
		"SystemDefaultRegistry": systemDefaultRegistry,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render etcd-proxy template: %w", err)
	}

	return &bootstrapv1.File{
		Path:        etcd.EtcdProxyDaemonsetYamlLocation,
		Content:     buf.String(),
		Owner:       "root:root",
		Permissions: "0640",
	}, nil
}

func (r *KThreesConfigReconciler) handleClusterNotInitialized(ctx context.Context, scope *Scope) (_ ctrl.Result, reterr error) {
	// initialize the DataSecretAvailableCondition if missing.
	// this is required in order to avoid the condition's LastTransitionTime to flicker in case of errors surfacing
	// using the DataSecretGeneratedFailedReason
	if !conditions.Has(scope.Config, bootstrapv1.KThreesConfigDataSecretAvailableCondition) {
		v1beta1conditions.MarkFalse(scope.Config, bootstrapv1.DataSecretAvailableV1Beta1Condition, clusterv1.WaitingForControlPlaneAvailableV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
		conditions.Set(scope.Config, metav1.Condition{
			Type:    bootstrapv1.KThreesConfigDataSecretAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  bootstrapv1.KThreesConfigDataSecretNotAvailableReason,
			Message: "Waiting for Cluster control plane to be initialized",
		})
	}

	// if it's NOT a control plane machine, requeue
	if !scope.ConfigOwner.IsControlPlaneMachine() {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	machine := &clusterv1.Machine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(scope.ConfigOwner.Object, machine); err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot convert %s to Machine: %w", scope.ConfigOwner.GetKind(), err)
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
				reterr = kerrors.NewAggregate([]error{reterr, ErrFailedUnlock})
			}
		}
	}()

	scope.Info("Creating BootstrapData for the init control plane")

	// injects into config.ClusterConfiguration values from top level object
	r.reconcileTopLevelObjectSettings(scope.Cluster, machine, scope.Config)

	certificates := secret.NewCertificatesForInitialControlPlane(&scope.Config.Spec)
	err := certificates.LookupOrGenerate(
		ctx,
		r.Client,
		util.ObjectKey(scope.Cluster),
		*metav1.NewControllerRef(scope.Config, bootstrapv1.GroupVersion.WithKind("KThreesConfig")),
	)
	if err != nil {
		v1beta1conditions.MarkFalse(scope.Config, bootstrapv1.CertificatesAvailableV1Beta1Condition, bootstrapv1.CertificatesGenerationFailedV1Beta1Reason, clusterv1.ConditionSeverityWarning, "failed looking up or generating certificates: %s", err.Error())
		conditions.Set(scope.Config, metav1.Condition{
			Type:    bootstrapv1.KThreesConfigCertificatesAvailableCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  bootstrapv1.KThreesConfigCertificatesAvailableInternalErrorReason,
			Message: "Please check controller logs for errors",
		})
		return ctrl.Result{}, err
	}
	v1beta1conditions.MarkTrue(scope.Config, bootstrapv1.CertificatesAvailableV1Beta1Condition)
	conditions.Set(scope.Config, metav1.Condition{
		Type:   bootstrapv1.KThreesConfigCertificatesAvailableCondition,
		Status: metav1.ConditionTrue,
		Reason: bootstrapv1.KThreesConfigCertificatesAvailableReason,
	})

	token, err := token.Lookup(ctx, r.Client, client.ObjectKeyFromObject(scope.Cluster))
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO support k3s great feature of external backends.
	// For now just use the etcd option
	configStruct := k3s.GenerateInitControlPlaneConfig(
		scope.Cluster.Spec.ControlPlaneEndpoint.Host,
		*token,
		scope.Config.Spec.ServerConfig,
		scope.Config.Spec.AgentConfig)

	b, err := kubeyaml.Marshal(configStruct)
	if err != nil {
		return ctrl.Result{}, err
	}

	initConfigFile := bootstrapv1.File{
		Path:        k3s.DefaultK3sConfigLocation,
		Content:     string(b),
		Owner:       "root:root",
		Permissions: "0640",
	}

	files, err := r.resolveFiles(ctx, scope.Config)
	if err != nil {
		markDataSecretGenerationFailed(scope.Config, err)
		return ctrl.Result{}, err
	}

	if scope.Config.Spec.IsEtcdEmbedded() {
		etcdProxyFile, err := r.resolveEtcdProxyFile(scope.Config)
		if err != nil {
			markDataSecretGenerationFailed(scope.Config, err)
			return ctrl.Result{}, fmt.Errorf("failed to resolve etcd proxy file: %w", err)
		}
		files = append(files, *etcdProxyFile)
	}

	cpinput := &cloudinit.ControlPlaneInput{
		BaseUserData: cloudinit.BaseUserData{
			PreK3sCommands:             scope.Config.Spec.PreK3sCommands,
			PostK3sCommands:            scope.Config.Spec.PostK3sCommands,
			AdditionalFiles:            files,
			ConfigFile:                 initConfigFile,
			K3sVersion:                 scope.Config.Spec.Version,
			AirGapped:                  scope.Config.Spec.AgentConfig.AirGapped,
			AirGappedInstallScriptPath: scope.Config.Spec.AgentConfig.AirGappedInstallScriptPath,
		},
		Certificates: certificates,
	}

	cloudInitData, err := cloudinit.NewInitControlPlane(cpinput)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.storeBootstrapData(ctx, scope, cloudInitData); err != nil {
		scope.Error(err, "Failed to store bootstrap data")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KThreesConfigReconciler) SetupWithManager(mgr ctrl.Manager, concurrency int) error {
	if r.KThreesInitLock == nil {
		r.KThreesInitLock = locking.NewControlPlaneInitMutex(mgr.GetClient())
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&bootstrapv1.KThreesConfig{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrency,
		}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetScheme(), r.Log)).
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
				clusterv1.ClusterNameLabel: scope.Cluster.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: bootstrapv1.GroupVersion.String(),
					Kind:       "KThreesConfig",
					Name:       scope.Config.Name,
					UID:        scope.Config.UID,
					Controller: ptr.To[bool](true),
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
			return fmt.Errorf("failed to create bootstrap data secret for KThreesConfig %s/%s: %w", scope.Config.Namespace, scope.Config.Name, err)
		}
		r.Log.Info("bootstrap data secret for KThreesConfig already exists, updating", "secret", secret.Name, "KThreesConfig", scope.Config.Name)
		if err := r.Client.Update(ctx, secret); err != nil {
			return fmt.Errorf("failed to update bootstrap data secret for KThreesConfig %s/%s: %w", scope.Config.Namespace, scope.Config.Name, err)
		}
	}

	scope.Config.Status.DataSecretName = secret.Name
	scope.Config.Status.Initialization.DataSecretCreated = ptr.To(true)
	v1beta1conditions.MarkTrue(scope.Config, bootstrapv1.DataSecretAvailableV1Beta1Condition)
	conditions.Set(scope.Config, metav1.Condition{
		Type:   bootstrapv1.KThreesConfigDataSecretAvailableCondition,
		Status: metav1.ConditionTrue,
		Reason: bootstrapv1.KThreesConfigDataSecretAvailableReason,
	})
	return nil
}

// markDataSecretAndCertificatesAvailable sets the DataSecretAvailable and CertificatesAvailable conditions to true.
// NOTE: Certificates must be available to generate the bootstrap data secret.
func markDataSecretAndCertificatesAvailable(config *bootstrapv1.KThreesConfig) {
	conditions.Set(config, metav1.Condition{
		Type:   bootstrapv1.KThreesConfigDataSecretAvailableCondition,
		Status: metav1.ConditionTrue,
		Reason: bootstrapv1.KThreesConfigDataSecretAvailableReason,
	})
	conditions.Set(config, metav1.Condition{
		Type:   bootstrapv1.KThreesConfigCertificatesAvailableCondition,
		Status: metav1.ConditionTrue,
		Reason: bootstrapv1.KThreesConfigCertificatesAvailableReason,
	})
}

// markDataSecretGenerationFailed reports a failure in generating the bootstrap data secret.
func markDataSecretGenerationFailed(config *bootstrapv1.KThreesConfig, err error) {
	v1beta1conditions.MarkFalse(config, bootstrapv1.DataSecretAvailableV1Beta1Condition, bootstrapv1.DataSecretGenerationFailedV1Beta1Reason, clusterv1.ConditionSeverityWarning, "%s", err.Error())
	conditions.Set(config, metav1.Condition{
		Type:    bootstrapv1.KThreesConfigDataSecretAvailableCondition,
		Status:  metav1.ConditionFalse,
		Reason:  bootstrapv1.KThreesConfigDataSecretNotAvailableReason,
		Message: fmt.Sprintf("Failed to generate the bootstrap data secret: %s", err.Error()),
	})
}

func (r *KThreesConfigReconciler) reconcileTopLevelObjectSettings(_ *clusterv1.Cluster, machine *clusterv1.Machine, config *bootstrapv1.KThreesConfig) {
	log := r.Log.WithValues("kthreesconfig", fmt.Sprintf("%s/%s", config.Namespace, config.Name))

	// If there are no Version settings defined in Config, use Version from machine, if defined
	if config.Spec.Version == "" && machine.Spec.Version != "" {
		config.Spec.Version = machine.Spec.Version
		log.Info("Altering Config", "Version", config.Spec.Version)
	}
}
