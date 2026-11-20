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
	"fmt"
	"maps"
	"reflect"

	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	ctrlconversion "sigs.k8s.io/controller-runtime/pkg/conversion"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta3"
)

// ConvertTo converts the v1beta2 KThreesConfig receiver to a v1beta3 KThreesConfig.
func (c *KThreesConfig) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1.KThreesConfig)
	if err := Convert_v1beta2_KThreesConfig_To_v1beta3_KThreesConfig(c, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta2 to v1beta3: %w", err)
	}

	// Manually restore data.
	restored := &bootstrapv1.KThreesConfig{}
	ok, err := utilconversion.UnmarshalData(c, restored)
	if err != nil {
		return fmt.Errorf("unmarshalling stored conversion data: %w", err)
	}

	// Recover intent for bool values converted to *bool.
	initialization := bootstrapv1.KThreesConfigInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(c.Status.Ready, ok, restored.Status.Initialization.DataSecretCreated, &initialization.DataSecretCreated)
	if !reflect.DeepEqual(initialization, bootstrapv1.KThreesConfigInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}
	return nil
}

// ConvertFrom converts the v1beta2 KThreesConfig receiver from a v1beta3 KThreesConfig.
func (c *KThreesConfig) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1.KThreesConfig)
	if err := Convert_v1beta3_KThreesConfig_To_v1beta2_KThreesConfig(src, c, nil); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta2 from v1beta3: %w", err)
	}

	// Preserve Hub data on down-conversion except for metadata.
	// NOTE: The annotations map is cloned because the generated conversion copies ObjectMeta by value,
	// and thus the map would otherwise be shared with (and mutated on) the source object.
	c.SetAnnotations(maps.Clone(c.GetAnnotations()))
	if err := utilconversion.MarshalData(src, c); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta2 KThreesConfigList receiver to a v1beta3 KThreesConfigList.
func (c *KThreesConfigList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1.KThreesConfigList)
	if err := Convert_v1beta2_KThreesConfigList_To_v1beta3_KThreesConfigList(c, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigList v1beta2 to v1beta3: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta2 KThreesConfigList receiver from a v1beta3 KThreesConfigList.
func (c *KThreesConfigList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1.KThreesConfigList)
	if err := Convert_v1beta3_KThreesConfigList_To_v1beta2_KThreesConfigList(src, c, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigList v1beta2 from v1beta3: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta2 KThreesConfigTemplate receiver to a v1beta3 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1.KThreesConfigTemplate)
	if err := Convert_v1beta2_KThreesConfigTemplate_To_v1beta3_KThreesConfigTemplate(r, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta2 to v1beta3: %w", err)
	}

	// Drop the conversion data annotation; there are no fields to restore yet.
	if _, err := utilconversion.UnmarshalData(r, &bootstrapv1.KThreesConfigTemplate{}); err != nil {
		return fmt.Errorf("unmarshalling stored conversion data: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta2 KThreesConfigTemplate receiver from a v1beta3 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1.KThreesConfigTemplate)
	if err := Convert_v1beta3_KThreesConfigTemplate_To_v1beta2_KThreesConfigTemplate(src, r, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta2 from v1beta3: %w", err)
	}

	// Preserve Hub data on down-conversion except for metadata.
	// NOTE: The annotations map is cloned because the generated conversion copies ObjectMeta by value,
	// and thus the map would otherwise be shared with (and mutated on) the source object.
	r.SetAnnotations(maps.Clone(r.GetAnnotations()))
	if err := utilconversion.MarshalData(src, r); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta2 KThreesConfigTemplateList receiver to a v1beta3 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1.KThreesConfigTemplateList)
	if err := Convert_v1beta2_KThreesConfigTemplateList_To_v1beta3_KThreesConfigTemplateList(r, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplateList v1beta2 to v1beta3: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta2 KThreesConfigTemplateList receiver from a v1beta3 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1.KThreesConfigTemplateList)
	if err := Convert_v1beta3_KThreesConfigTemplateList_To_v1beta2_KThreesConfigTemplateList(src, r, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplateList v1beta2 from v1beta3: %w", err)
	}
	return nil
}

// Convert_v1beta2_KThreesServerConfig_To_v1beta3_KThreesServerConfig converts KThreesServerConfig v1beta2 to v1beta3.
// DeprecatedDisableExternalCloudProvider does not exist in v1beta3 and is dropped;
// DisableCloudController and CloudProviderName must be used instead.
func Convert_v1beta2_KThreesServerConfig_To_v1beta3_KThreesServerConfig(in *KThreesServerConfig, out *bootstrapv1.KThreesServerConfig, s apimachineryconversion.Scope) error { //nolint: stylecheck
	return autoConvert_v1beta2_KThreesServerConfig_To_v1beta3_KThreesServerConfig(in, out, s)
}

// Convert_v1beta2_KThreesConfigStatus_To_v1beta3_KThreesConfigStatus converts KThreesConfigStatus v1beta2 to v1beta3.
func Convert_v1beta2_KThreesConfigStatus_To_v1beta3_KThreesConfigStatus(in *KThreesConfigStatus, out *bootstrapv1.KThreesConfigStatus, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta2_KThreesConfigStatus_To_v1beta3_KThreesConfigStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions.
	// NOTE: v1beta2 conditions (CAPI v1beta1 condition type) should not be automatically converted into
	// metav1 conditions; they are moved to the deprecated field below.
	out.Conditions = nil

	out.DataSecretName = ptr.Deref(in.DataSecretName, "")

	// Retrieve new conditions from the v1beta2 field.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Move legacy conditions, failureReason and failureMessage to the deprecated field.
	// NOTE: status.ready is converted to status.initialization.dataSecretCreated in ConvertTo.
	if in.Conditions == nil && in.FailureReason == "" && in.FailureMessage == "" {
		return nil
	}
	out.Deprecated = &bootstrapv1.KThreesConfigDeprecatedStatus{
		V1Beta1: &bootstrapv1.KThreesConfigV1Beta1DeprecatedStatus{
			FailureReason:  in.FailureReason,
			FailureMessage: in.FailureMessage,
		},
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

// Convert_v1beta3_KThreesConfigStatus_To_v1beta2_KThreesConfigStatus converts KThreesConfigStatus v1beta3 to v1beta2.
func Convert_v1beta3_KThreesConfigStatus_To_v1beta2_KThreesConfigStatus(in *bootstrapv1.KThreesConfigStatus, out *KThreesConfigStatus, s apimachineryconversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta3_KThreesConfigStatus_To_v1beta2_KThreesConfigStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions.
	// NOTE: metav1 conditions should not be automatically converted into legacy conditions.
	out.Conditions = nil

	out.DataSecretName = nil
	if in.DataSecretName != "" {
		out.DataSecretName = ptr.To(in.DataSecretName)
	}
	out.Ready = ptr.Deref(in.Initialization.DataSecretCreated, false)

	// Retrieve legacy conditions, failureReason and failureMessage from the deprecated field.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		if in.Deprecated.V1Beta1.Conditions != nil {
			clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		}
		out.FailureReason = in.Deprecated.V1Beta1.FailureReason
		out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
	}

	// Move new conditions to the v1beta2 field.
	if in.Conditions != nil {
		out.V1Beta2 = &KThreesConfigV1Beta2Status{
			Conditions: in.Conditions,
		}
	}
	return nil
}

// Convert_v1beta2_KThreesConfigSpec_To_v1beta3_KThreesConfigSpec converts KThreesConfigSpec v1beta2 to v1beta3.
// NOTE: This is defined explicitly so it can be used by conversion-gen in other packages (e.g. controlplane/api/v1beta2).
func Convert_v1beta2_KThreesConfigSpec_To_v1beta3_KThreesConfigSpec(in *KThreesConfigSpec, out *bootstrapv1.KThreesConfigSpec, s apimachineryconversion.Scope) error { //nolint: stylecheck
	return autoConvert_v1beta2_KThreesConfigSpec_To_v1beta3_KThreesConfigSpec(in, out, s)
}

// Convert_v1beta3_KThreesConfigSpec_To_v1beta2_KThreesConfigSpec converts KThreesConfigSpec v1beta3 to v1beta2.
// NOTE: This is defined explicitly so it can be used by conversion-gen in other packages (e.g. controlplane/api/v1beta2).
func Convert_v1beta3_KThreesConfigSpec_To_v1beta2_KThreesConfigSpec(in *bootstrapv1.KThreesConfigSpec, out *KThreesConfigSpec, s apimachineryconversion.Scope) error { //nolint: stylecheck
	return autoConvert_v1beta3_KThreesConfigSpec_To_v1beta2_KThreesConfigSpec(in, out, s)
}
