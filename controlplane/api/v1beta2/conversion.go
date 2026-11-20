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
	"errors"
	"fmt"
	"maps"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	ctrlconversion "sigs.k8s.io/controller-runtime/pkg/conversion"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta3"
)

var apiVersionGetter = func(_ schema.GroupKind) (string, error) {
	return "", errors.New("apiVersionGetter not set")
}

// SetAPIVersionGetter sets the function used to get the apiVersion for a GroupKind.
// It is used when converting a ContractVersionedObjectReference (v1beta3) to an ObjectReference (v1beta2).
func SetAPIVersionGetter(f func(gk schema.GroupKind) (string, error)) {
	apiVersionGetter = f
}

// ConvertTo converts the v1beta2 KThreesControlPlane receiver to a v1beta3 KThreesControlPlane.
func (in *KThreesControlPlane) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*controlplanev1.KThreesControlPlane)
	if err := Convert_v1beta2_KThreesControlPlane_To_v1beta3_KThreesControlPlane(in, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlane v1beta2 to v1beta3: %w", err)
	}

	infraRef, err := convertToContractVersionedObjectReference(&in.Spec.MachineTemplate.InfrastructureRef)
	if err != nil {
		return err
	}
	dst.Spec.MachineTemplate.Spec.InfrastructureRef = *infraRef

	// Manually restore data.
	restored := &controlplanev1.KThreesControlPlane{}
	ok, err := utilconversion.UnmarshalData(in, restored)
	if err != nil {
		return fmt.Errorf("unmarshalling stored conversion data: %w", err)
	}

	// Recover intent for bool values converted to *bool.
	initialization := controlplanev1.KThreesControlPlaneInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(in.Status.Initialized, ok, restored.Status.Initialization.ControlPlaneInitialized, &initialization.ControlPlaneInitialized)
	if !reflect.DeepEqual(initialization, controlplanev1.KThreesControlPlaneInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}

	return nil
}

// ConvertFrom converts the v1beta2 KThreesControlPlane receiver from a v1beta3 KThreesControlPlane.
func (in *KThreesControlPlane) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*controlplanev1.KThreesControlPlane)
	if err := Convert_v1beta3_KThreesControlPlane_To_v1beta2_KThreesControlPlane(src, in, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlane v1beta2 from v1beta3: %w", err)
	}

	if src.Spec.MachineTemplate.Spec.InfrastructureRef.IsDefined() {
		infraRef, err := convertToObjectReference(&src.Spec.MachineTemplate.Spec.InfrastructureRef, src.Namespace)
		if err != nil {
			return err
		}
		in.Spec.MachineTemplate.InfrastructureRef = *infraRef
	}

	// Preserve Hub data on down-conversion except for metadata.
	// NOTE: The annotations map is cloned because the generated conversion copies ObjectMeta by value,
	// and thus the map would otherwise be shared with (and mutated on) the source object.
	in.SetAnnotations(maps.Clone(in.GetAnnotations()))
	if err := utilconversion.MarshalData(src, in); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta2 KThreesControlPlaneList receiver to a v1beta3 KThreesControlPlaneList.
func (in *KThreesControlPlaneList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*controlplanev1.KThreesControlPlaneList)
	if err := Convert_v1beta2_KThreesControlPlaneList_To_v1beta3_KThreesControlPlaneList(in, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneList v1beta2 to v1beta3: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta2 KThreesControlPlaneList receiver from a v1beta3 KThreesControlPlaneList.
func (in *KThreesControlPlaneList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*controlplanev1.KThreesControlPlaneList)
	if err := Convert_v1beta3_KThreesControlPlaneList_To_v1beta2_KThreesControlPlaneList(src, in, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneList v1beta2 from v1beta3: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta2 KThreesControlPlaneTemplate receiver to a v1beta3 KThreesControlPlaneTemplate.
func (in *KThreesControlPlaneTemplate) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*controlplanev1.KThreesControlPlaneTemplate)
	if err := Convert_v1beta2_KThreesControlPlaneTemplate_To_v1beta3_KThreesControlPlaneTemplate(in, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneTemplate v1beta2 to v1beta3: %w", err)
	}

	infraRef, err := convertToContractVersionedObjectReference(&in.Spec.Template.Spec.MachineTemplate.InfrastructureRef)
	if err != nil {
		return err
	}
	dst.Spec.Template.Spec.MachineTemplate.Spec.InfrastructureRef = *infraRef

	// Manually restore data.
	restored := &controlplanev1.KThreesControlPlaneTemplate{}
	if _, err := utilconversion.UnmarshalData(in, restored); err != nil {
		return fmt.Errorf("unmarshalling stored conversion data: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta2 KThreesControlPlaneTemplate receiver from a v1beta3 KThreesControlPlaneTemplate.
func (in *KThreesControlPlaneTemplate) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*controlplanev1.KThreesControlPlaneTemplate)
	if err := Convert_v1beta3_KThreesControlPlaneTemplate_To_v1beta2_KThreesControlPlaneTemplate(src, in, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneTemplate v1beta2 from v1beta3: %w", err)
	}

	if src.Spec.Template.Spec.MachineTemplate.Spec.InfrastructureRef.IsDefined() {
		infraRef, err := convertToObjectReference(&src.Spec.Template.Spec.MachineTemplate.Spec.InfrastructureRef, src.Namespace)
		if err != nil {
			return err
		}
		in.Spec.Template.Spec.MachineTemplate.InfrastructureRef = *infraRef
	}

	// Preserve Hub data on down-conversion except for metadata.
	// NOTE: The annotations map is cloned because the generated conversion copies ObjectMeta by value,
	// and thus the map would otherwise be shared with (and mutated on) the source object.
	in.SetAnnotations(maps.Clone(in.GetAnnotations()))
	if err := utilconversion.MarshalData(src, in); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta2 KThreesControlPlaneTemplateList receiver to a v1beta3 KThreesControlPlaneTemplateList.
func (in *KThreesControlPlaneTemplateList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*controlplanev1.KThreesControlPlaneTemplateList)
	if err := Convert_v1beta2_KThreesControlPlaneTemplateList_To_v1beta3_KThreesControlPlaneTemplateList(in, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneTemplateList v1beta2 to v1beta3: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta2 KThreesControlPlaneTemplateList receiver from a v1beta3 KThreesControlPlaneTemplateList.
func (in *KThreesControlPlaneTemplateList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*controlplanev1.KThreesControlPlaneTemplateList)
	if err := Convert_v1beta3_KThreesControlPlaneTemplateList_To_v1beta2_KThreesControlPlaneTemplateList(src, in, nil); err != nil {
		return fmt.Errorf("converting KThreesControlPlaneTemplateList v1beta2 from v1beta3: %w", err)
	}
	return nil
}

// Convert_v1beta2_KThreesControlPlaneSpec_To_v1beta3_KThreesControlPlaneSpec converts KThreesControlPlaneSpec v1beta2 to v1beta3.
func Convert_v1beta2_KThreesControlPlaneSpec_To_v1beta3_KThreesControlPlaneSpec(in *KThreesControlPlaneSpec, out *controlplanev1.KThreesControlPlaneSpec, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta2_KThreesControlPlaneSpec_To_v1beta3_KThreesControlPlaneSpec(in, out, s); err != nil {
		return err
	}
	if in.RolloutAfter != nil && !reflect.DeepEqual(in.RolloutAfter, &metav1.Time{}) {
		out.Rollout.After = *in.RolloutAfter
	}
	return nil
}

// Convert_v1beta3_KThreesControlPlaneSpec_To_v1beta2_KThreesControlPlaneSpec converts KThreesControlPlaneSpec v1beta3 to v1beta2.
func Convert_v1beta3_KThreesControlPlaneSpec_To_v1beta2_KThreesControlPlaneSpec(in *controlplanev1.KThreesControlPlaneSpec, out *KThreesControlPlaneSpec, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta3_KThreesControlPlaneSpec_To_v1beta2_KThreesControlPlaneSpec(in, out, s); err != nil {
		return err
	}
	if !reflect.DeepEqual(in.Rollout.After, metav1.Time{}) {
		out.RolloutAfter = ptr.To(in.Rollout.After)
	}
	return nil
}

// Convert_v1beta2_KThreesControlPlaneTemplateResourceSpec_To_v1beta3_KThreesControlPlaneTemplateResourceSpec converts
// KThreesControlPlaneTemplateResourceSpec v1beta2 to v1beta3.
func Convert_v1beta2_KThreesControlPlaneTemplateResourceSpec_To_v1beta3_KThreesControlPlaneTemplateResourceSpec(in *KThreesControlPlaneTemplateResourceSpec, out *controlplanev1.KThreesControlPlaneTemplateResourceSpec, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta2_KThreesControlPlaneTemplateResourceSpec_To_v1beta3_KThreesControlPlaneTemplateResourceSpec(in, out, s); err != nil {
		return err
	}
	if in.RolloutAfter != nil && !reflect.DeepEqual(in.RolloutAfter, &metav1.Time{}) {
		out.Rollout.After = *in.RolloutAfter
	}
	return nil
}

// Convert_v1beta3_KThreesControlPlaneTemplateResourceSpec_To_v1beta2_KThreesControlPlaneTemplateResourceSpec converts
// KThreesControlPlaneTemplateResourceSpec v1beta3 to v1beta2.
func Convert_v1beta3_KThreesControlPlaneTemplateResourceSpec_To_v1beta2_KThreesControlPlaneTemplateResourceSpec(in *controlplanev1.KThreesControlPlaneTemplateResourceSpec, out *KThreesControlPlaneTemplateResourceSpec, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta3_KThreesControlPlaneTemplateResourceSpec_To_v1beta2_KThreesControlPlaneTemplateResourceSpec(in, out, s); err != nil {
		return err
	}
	if !reflect.DeepEqual(in.Rollout.After, metav1.Time{}) {
		out.RolloutAfter = ptr.To(in.Rollout.After)
	}
	return nil
}

// Convert_v1beta2_KThreesControlPlaneMachineTemplate_To_v1beta3_KThreesControlPlaneMachineTemplate converts
// KThreesControlPlaneMachineTemplate v1beta2 to v1beta3.
// NOTE: InfrastructureRef is converted in ConvertTo, as it requires the apiVersion to be resolved.
func Convert_v1beta2_KThreesControlPlaneMachineTemplate_To_v1beta3_KThreesControlPlaneMachineTemplate(in *KThreesControlPlaneMachineTemplate, out *controlplanev1.KThreesControlPlaneMachineTemplate, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta2_KThreesControlPlaneMachineTemplate_To_v1beta3_KThreesControlPlaneMachineTemplate(in, out, s); err != nil {
		return err
	}
	out.Spec.Deletion.NodeDrainTimeoutSeconds = clusterv1.ConvertToSeconds(in.NodeDrainTimeout)
	out.Spec.Deletion.NodeVolumeDetachTimeoutSeconds = clusterv1.ConvertToSeconds(in.NodeVolumeDetachTimeout)
	out.Spec.Deletion.NodeDeletionTimeoutSeconds = clusterv1.ConvertToSeconds(in.NodeDeletionTimeout)
	return nil
}

// Convert_v1beta3_KThreesControlPlaneMachineTemplate_To_v1beta2_KThreesControlPlaneMachineTemplate converts
// KThreesControlPlaneMachineTemplate v1beta3 to v1beta2.
// NOTE: InfrastructureRef is converted in ConvertFrom, as it requires the apiVersion to be resolved.
func Convert_v1beta3_KThreesControlPlaneMachineTemplate_To_v1beta2_KThreesControlPlaneMachineTemplate(in *controlplanev1.KThreesControlPlaneMachineTemplate, out *KThreesControlPlaneMachineTemplate, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta3_KThreesControlPlaneMachineTemplate_To_v1beta2_KThreesControlPlaneMachineTemplate(in, out, s); err != nil {
		return err
	}
	out.NodeDrainTimeout = clusterv1.ConvertFromSeconds(in.Spec.Deletion.NodeDrainTimeoutSeconds)
	out.NodeVolumeDetachTimeout = clusterv1.ConvertFromSeconds(in.Spec.Deletion.NodeVolumeDetachTimeoutSeconds)
	out.NodeDeletionTimeout = clusterv1.ConvertFromSeconds(in.Spec.Deletion.NodeDeletionTimeoutSeconds)
	return nil
}

// Convert_v1beta2_KThreesControlPlaneStatus_To_v1beta3_KThreesControlPlaneStatus converts KThreesControlPlaneStatus v1beta2 to v1beta3.
func Convert_v1beta2_KThreesControlPlaneStatus_To_v1beta3_KThreesControlPlaneStatus(in *KThreesControlPlaneStatus, out *controlplanev1.KThreesControlPlaneStatus, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta2_KThreesControlPlaneStatus_To_v1beta3_KThreesControlPlaneStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions.
	// NOTE: legacy conditions should not be automatically converted into metav1 conditions.
	out.Conditions = nil

	// Reset replica counters from autogenerated conversions.
	// NOTE: old replica counters should not be automatically converted into replica counters with a new semantic.
	out.ReadyReplicas = nil

	// Retrieve new conditions and replica counters from the v1beta2 field.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
		out.ReadyReplicas = in.V1Beta2.ReadyReplicas
		out.AvailableReplicas = in.V1Beta2.AvailableReplicas
		out.UpToDateReplicas = in.V1Beta2.UpToDateReplicas
	}

	// Move legacy conditions, failureReason, failureMessage and replica counters to the deprecated field.
	out.Deprecated = &controlplanev1.KThreesControlPlaneDeprecatedStatus{
		V1Beta1: &controlplanev1.KThreesControlPlaneV1Beta1DeprecatedStatus{
			FailureReason:       in.FailureReason,
			FailureMessage:      in.FailureMessage,
			UpdatedReplicas:     in.UpdatedReplicas,
			ReadyReplicas:       in.ReadyReplicas,
			UnavailableReplicas: in.UnavailableReplicas,
		},
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}

	// NOTE: status.initialized is converted to status.initialization.controlPlaneInitialized in ConvertTo.
	return nil
}

// Convert_v1beta3_KThreesControlPlaneStatus_To_v1beta2_KThreesControlPlaneStatus converts KThreesControlPlaneStatus v1beta3 to v1beta2.
func Convert_v1beta3_KThreesControlPlaneStatus_To_v1beta2_KThreesControlPlaneStatus(in *controlplanev1.KThreesControlPlaneStatus, out *KThreesControlPlaneStatus, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta3_KThreesControlPlaneStatus_To_v1beta2_KThreesControlPlaneStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions.
	// NOTE: metav1 conditions should not be automatically converted into legacy conditions.
	out.Conditions = nil

	// Reset replica counters from autogenerated conversions.
	// NOTE: replica counters with a new semantic should not be automatically converted into old replica counters.
	out.ReadyReplicas = 0

	// Retrieve legacy conditions, failureReason, failureMessage and replica counters from the deprecated field.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		if in.Deprecated.V1Beta1.Conditions != nil {
			clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		}
		out.FailureReason = in.Deprecated.V1Beta1.FailureReason
		out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
		out.UpdatedReplicas = in.Deprecated.V1Beta1.UpdatedReplicas
		out.ReadyReplicas = in.Deprecated.V1Beta1.ReadyReplicas
		out.UnavailableReplicas = in.Deprecated.V1Beta1.UnavailableReplicas
	}

	out.Version = nil
	if in.Version != "" {
		out.Version = ptr.To(in.Version)
	}

	// Move controlPlaneInitialized to initialized, rebuild ready.
	out.Initialized = ptr.Deref(in.Initialization.ControlPlaneInitialized, false)
	out.Ready = out.ReadyReplicas > 0

	// Move new conditions and replica counters to the v1beta2 field.
	if in.Conditions == nil && in.ReadyReplicas == nil && in.AvailableReplicas == nil && in.UpToDateReplicas == nil {
		return nil
	}
	out.V1Beta2 = &KThreesControlPlaneV1Beta2Status{
		Conditions:        in.Conditions,
		ReadyReplicas:     in.ReadyReplicas,
		AvailableReplicas: in.AvailableReplicas,
		UpToDateReplicas:  in.UpToDateReplicas,
	}
	return nil
}

// Convert_v1_ObjectMeta_To_v1beta2_ObjectMeta converts a metav1.ObjectMeta to a Cluster API ObjectMeta.
// Only labels and annotations are preserved.
func Convert_v1_ObjectMeta_To_v1beta2_ObjectMeta(in *metav1.ObjectMeta, out *clusterv1.ObjectMeta, _ apimachineryconversion.Scope) error { //nolint: stylecheck
	out.Labels = in.Labels
	out.Annotations = in.Annotations
	return nil
}

// Convert_v1beta2_ObjectMeta_To_v1_ObjectMeta converts a Cluster API ObjectMeta to a metav1.ObjectMeta.
func Convert_v1beta2_ObjectMeta_To_v1_ObjectMeta(in *clusterv1.ObjectMeta, out *metav1.ObjectMeta, _ apimachineryconversion.Scope) error { //nolint: stylecheck
	out.Labels = in.Labels
	out.Annotations = in.Annotations
	return nil
}

// Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta converts a Cluster API v1beta1 ObjectMeta to a v1beta2 ObjectMeta.
func Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in *clusterv1beta1.ObjectMeta, out *clusterv1.ObjectMeta, s apimachineryconversion.Scope) error { //nolint: stylecheck
	return clusterv1beta1.Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in, out, s)
}

// Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta converts a Cluster API v1beta2 ObjectMeta to a v1beta1 ObjectMeta.
func Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in *clusterv1.ObjectMeta, out *clusterv1beta1.ObjectMeta, s apimachineryconversion.Scope) error { //nolint: stylecheck
	return clusterv1beta1.Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in, out, s)
}

// Convert_v1_Condition_To_v1beta1_Condition converts a metav1 condition to a Cluster API v1beta1 condition.
func Convert_v1_Condition_To_v1beta1_Condition(in *metav1.Condition, out *clusterv1beta1.Condition, s apimachineryconversion.Scope) error { //nolint: stylecheck
	return clusterv1beta1.Convert_v1_Condition_To_v1beta1_Condition(in, out, s)
}

// Convert_v1beta1_Condition_To_v1_Condition converts a Cluster API v1beta1 condition to a metav1 condition.
func Convert_v1beta1_Condition_To_v1_Condition(in *clusterv1beta1.Condition, out *metav1.Condition, s apimachineryconversion.Scope) error { //nolint: stylecheck
	return clusterv1beta1.Convert_v1beta1_Condition_To_v1_Condition(in, out, s)
}

func convertToContractVersionedObjectReference(ref *corev1.ObjectReference) (*clusterv1.ContractVersionedObjectReference, error) {
	var apiGroup string
	if ref.APIVersion != "" {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to convert object: failed to parse apiVersion: %w", err)
		}
		apiGroup = gv.Group
	}
	return &clusterv1.ContractVersionedObjectReference{
		APIGroup: apiGroup,
		Kind:     ref.Kind,
		Name:     ref.Name,
	}, nil
}

func convertToObjectReference(ref *clusterv1.ContractVersionedObjectReference, namespace string) (*corev1.ObjectReference, error) {
	apiVersion, err := apiVersionGetter(schema.GroupKind{
		Group: ref.APIGroup,
		Kind:  ref.Kind,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert object: %w", err)
	}
	return &corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       ref.Kind,
		Namespace:  namespace,
		Name:       ref.Name,
	}, nil
}
