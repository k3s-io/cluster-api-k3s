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

package v1beta2

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager will setup the webhooks for the KThreesControlPlane.
func (in *KThreesControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		WithDefaulter(&KThreesControlPlane{}).
		WithValidator(&KThreesControlPlane{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-controlplane-cluster-x-k8s-io-v1beta2-kthreescontrolplane,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=controlplane.cluster.x-k8s.io,resources=kthreescontrolplanes,versions=v1beta2,name=validation.kthreescontrolplane.controlplane.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-controlplane-cluster-x-k8s-io-v1beta2-kthreescontrolplane,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=controlplane.cluster.x-k8s.io,resources=kthreescontrolplanes,versions=v1beta2,name=default.kthreescontrolplane.controlplane.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ admission.CustomDefaulter = &KThreesControlPlane{}
var _ admission.CustomValidator = &KThreesControlPlane{}

// ValidateCreate will do any extra validation when creating a KThreesControlPlane.
func (in *KThreesControlPlane) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	kcp, ok := obj.(*KThreesControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a KThreesControlPlane but got a %T", obj))
	}

	allErrs := validateKThreesControlPlaneSpec(&kcp.Spec, field.NewPath("spec"), false)
	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("KThreesControlPlane").GroupKind(), kcp.Name, allErrs)
	}
	return nil, nil
}

// ValidateUpdate will do any extra validation when updating a KThreesControlPlane.
func (in *KThreesControlPlane) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldKCP, ok := oldObj.(*KThreesControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a KThreesControlPlane but got a %T", oldObj))
	}

	kcp, ok := newObj.(*KThreesControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a KThreesControlPlane but got a %T", newObj))
	}

	allErrs := validateKThreesControlPlaneSpec(
		&kcp.Spec,
		field.NewPath("spec"),
		retainsUnsafeZeroSurgeConfiguration(&oldKCP.Spec, &kcp.Spec),
	)
	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("KThreesControlPlane").GroupKind(), kcp.Name, allErrs)
	}
	return nil, nil
}

// ValidateDelete allows you to add any extra validation when deleting.
func (in *KThreesControlPlane) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return []string{}, nil
}

// Default will set default values for the KThreesControlPlane.
func (in *KThreesControlPlane) Default(_ context.Context, obj runtime.Object) error {
	c, ok := obj.(*KThreesControlPlane)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a KThreesControlPlane but got a %T", obj))
	}

	defaultKThreesControlPlaneSpec(&c.Spec, c.Namespace)
	return nil
}

func defaultKThreesControlPlaneSpec(s *KThreesControlPlaneSpec, namespace string) {
	if s.Replicas == nil {
		replicas := int32(1)
		s.Replicas = &replicas
	}

	if s.MachineTemplate.InfrastructureRef.Namespace == "" {
		s.MachineTemplate.InfrastructureRef.Namespace = namespace
	}

	if s.KThreesConfigSpec.ServerConfig.DisableCloudController == nil {
		s.KThreesConfigSpec.ServerConfig.DisableCloudController = ptr.To(true)
	}

	if s.KThreesConfigSpec.ServerConfig.CloudProviderName == nil {
		s.KThreesConfigSpec.ServerConfig.CloudProviderName = ptr.To("external")
	}

	if s.RolloutStrategy == nil {
		s.RolloutStrategy = &RolloutStrategy{}
	}
	if s.RolloutStrategy.Type == "" {
		s.RolloutStrategy.Type = RollingUpdateStrategyType
	}
	if s.RolloutStrategy.RollingUpdate == nil {
		s.RolloutStrategy.RollingUpdate = &RollingUpdate{}
	}
	if s.RolloutStrategy.RollingUpdate.MaxSurge == nil {
		s.RolloutStrategy.RollingUpdate.MaxSurge = ptr.To(intstr.FromInt32(1))
	}
}

func validateKThreesControlPlaneSpec(
	s *KThreesControlPlaneSpec,
	pathPrefix *field.Path,
	allowUnsafeZeroSurge bool,
) field.ErrorList {
	if s.RolloutStrategy == nil {
		return nil
	}

	allErrs := field.ErrorList{}
	rolloutStrategyPath := pathPrefix.Child("rolloutStrategy")
	if s.RolloutStrategy.Type != RollingUpdateStrategyType {
		allErrs = append(allErrs, field.Invalid(
			rolloutStrategyPath.Child("type"),
			s.RolloutStrategy.Type,
			"only RollingUpdate is supported",
		))
	}

	maxSurgePath := rolloutStrategyPath.Child("rollingUpdate", "maxSurge")
	var maxSurgeValue *intstr.IntOrString
	if s.RolloutStrategy.RollingUpdate != nil {
		maxSurgeValue = s.RolloutStrategy.RollingUpdate.MaxSurge
	}
	maxSurge, err := parseMaxSurge(maxSurgeValue)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(maxSurgePath, maxSurgeValue, err.Error()))
		return allErrs
	}

	replicas := ptr.Deref(s.Replicas, 1)
	if maxSurge == 0 && replicas < 3 && !feature.Gates.Enabled(feature.InPlaceUpdates) && !allowUnsafeZeroSurge {
		allErrs = append(allErrs, field.Forbidden(
			rolloutStrategyPath.Child("rollingUpdate"),
			"when KThreesControlPlane is configured with maxSurge 0, replica count needs to be at least 3 unless InPlaceUpdates is enabled",
		))
	}
	return allErrs
}

func retainsUnsafeZeroSurgeConfiguration(
	oldSpec *KThreesControlPlaneSpec,
	newSpec *KThreesControlPlaneSpec,
) bool {
	oldReplicas := ptr.Deref(oldSpec.Replicas, int32(1))
	newReplicas := ptr.Deref(newSpec.Replicas, int32(1))

	var oldMaxSurgeValue, newMaxSurgeValue *intstr.IntOrString
	if oldSpec.RolloutStrategy != nil && oldSpec.RolloutStrategy.RollingUpdate != nil {
		oldMaxSurgeValue = oldSpec.RolloutStrategy.RollingUpdate.MaxSurge
	}
	if newSpec.RolloutStrategy != nil && newSpec.RolloutStrategy.RollingUpdate != nil {
		newMaxSurgeValue = newSpec.RolloutStrategy.RollingUpdate.MaxSurge
	}

	oldMaxSurge, oldErr := parseMaxSurge(oldMaxSurgeValue)
	newMaxSurge, newErr := parseMaxSurge(newMaxSurgeValue)
	return oldErr == nil &&
		newErr == nil &&
		oldMaxSurge == 0 &&
		newMaxSurge == 0 &&
		oldReplicas == newReplicas &&
		newReplicas < 3
}

func parseMaxSurge(value *intstr.IntOrString) (int32, error) {
	if value == nil {
		return 1, nil
	}
	switch value.Type {
	case intstr.Int:
		if value.IntVal == 0 || value.IntVal == 1 {
			return value.IntVal, nil
		}
	case intstr.String:
		switch value.StrVal {
		case "0":
			return 0, nil
		case "1":
			return 1, nil
		}
	}
	return 0, fmt.Errorf("maxSurge must be 0 or 1")
}
