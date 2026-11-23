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
package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	bootstrapv1beta2 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
)

// convertToV1beta2 converts the v1beta1 KThreesConfig receiver to a v1beta2 KThreesConfig.
func (c *KThreesConfig) convertToV1beta2(dst *bootstrapv1beta2.KThreesConfig) error {
	if err := Convert_v1beta1_KThreesConfig_To_v1beta2_KThreesConfig(c, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta1 to v1beta2: %w", err)
	}

	restored := &bootstrapv1beta2.KThreesConfig{}
	if ok, err := utilconversion.UnmarshalData(c, restored); err != nil {
		return fmt.Errorf("unmarshalling stored conversion data: %w", err)
	} else if !ok {
		// No stored data.
		return nil
	}

	dst.Spec.ServerConfig.CloudProviderName = restored.Spec.ServerConfig.CloudProviderName
	dst.Spec.ServerConfig.DeprecatedDisableExternalCloudProvider = restored.Spec.ServerConfig.DeprecatedDisableExternalCloudProvider
	dst.Spec.ServerConfig.DisableCloudController = restored.Spec.ServerConfig.DisableCloudController
	dst.Spec.ServerConfig.SystemDefaultRegistry = restored.Spec.ServerConfig.SystemDefaultRegistry
	dst.Spec.ServerConfig.EtcdProxyImage = restored.Spec.ServerConfig.EtcdProxyImage
	dst.Spec.AgentConfig.AirGappedInstallScriptPath = restored.Spec.AgentConfig.AirGappedInstallScriptPath
	dst.Status.V1Beta2 = restored.Status.V1Beta2
	return nil
}

// convertFromV1beta2 converts the v1beta1 KThreesConfig receiver from a v1beta2 KThreesConfig.
func (c *KThreesConfig) convertFromV1beta2(src *bootstrapv1beta2.KThreesConfig) error {
	if err := Convert_v1beta2_KThreesConfig_To_v1beta1_KThreesConfig(src, c, nil); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta1 from v1beta2: %w", err)
	}

	if err := utilconversion.MarshalDataUnsafeNoCopy(src, c); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// convertToV1beta2 converts the v1beta1 KThreesConfigList receiver to a v1beta2 KThreesConfigList.
func (c *KThreesConfigList) convertToV1beta2(dst *bootstrapv1beta2.KThreesConfigList) error {
	if err := autoConvert_v1beta1_KThreesConfigList_To_v1beta2_KThreesConfigList(c, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigList v1beta1 to v1beta2: %w", err)
	}
	return nil
}

// convertFromV1beta2 converts the v1beta1 KThreesConfigList receiver from a v1beta2 KThreesConfigList.
func (c *KThreesConfigList) convertFromV1beta2(src *bootstrapv1beta2.KThreesConfigList) error {
	if err := autoConvert_v1beta2_KThreesConfigList_To_v1beta1_KThreesConfigList(src, c, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigList v1beta1 from v1beta2: %w", err)
	}
	return nil
}

// convertToV1beta2 converts the v1beta1 KThreesConfigTemplate receiver to a v1beta2 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) convertToV1beta2(dst *bootstrapv1beta2.KThreesConfigTemplate) error {
	if err := autoConvert_v1beta1_KThreesConfigTemplate_To_v1beta2_KThreesConfigTemplate(r, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta1 to v1beta2: %w", err)
	}

	restored := &bootstrapv1beta2.KThreesConfigTemplate{}
	if ok, err := utilconversion.UnmarshalData(r, restored); err != nil {
		return fmt.Errorf("unmarshalling stored conversion data: %w", err)
	} else if !ok {
		// No stored data.
		return nil
	}

	dst.Spec.Template.Spec.ServerConfig.CloudProviderName = restored.Spec.Template.Spec.ServerConfig.CloudProviderName
	dst.Spec.Template.Spec.ServerConfig.DeprecatedDisableExternalCloudProvider = restored.Spec.Template.Spec.ServerConfig.DeprecatedDisableExternalCloudProvider
	dst.Spec.Template.Spec.ServerConfig.DisableCloudController = restored.Spec.Template.Spec.ServerConfig.DisableCloudController
	dst.Spec.Template.Spec.ServerConfig.SystemDefaultRegistry = restored.Spec.Template.Spec.ServerConfig.SystemDefaultRegistry
	dst.Spec.Template.Spec.ServerConfig.EtcdProxyImage = restored.Spec.Template.Spec.ServerConfig.EtcdProxyImage
	dst.Spec.Template.Spec.AgentConfig.AirGappedInstallScriptPath = restored.Spec.Template.Spec.AgentConfig.AirGappedInstallScriptPath
	return nil
}

// convertFromV1beta2 converts the v1beta1 KThreesConfigTemplate receiver from a v1beta2 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) convertFromV1beta2(src *bootstrapv1beta2.KThreesConfigTemplate) error {
	if err := autoConvert_v1beta2_KThreesConfigTemplate_To_v1beta1_KThreesConfigTemplate(src, r, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta1 from v1beta2: %w", err)
	}

	if err := utilconversion.MarshalDataUnsafeNoCopy(src, r); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// convertToV1beta2 converts the v1beta1 KThreesConfigTemplateList receiver to a v1beta2 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) convertToV1beta2(dst *bootstrapv1beta2.KThreesConfigTemplateList) error {
	if err := autoConvert_v1beta1_KThreesConfigTemplateList_To_v1beta2_KThreesConfigTemplateList(r, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplateList v1beta1 to v1beta2: %w", err)
	}
	return nil
}

// convertFromV1beta2 converts the v1beta1 KThreesConfigTemplateList receiver from a v1beta2 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) convertFromV1beta2(src *bootstrapv1beta2.KThreesConfigTemplateList) error {
	if err := autoConvert_v1beta2_KThreesConfigTemplateList_To_v1beta1_KThreesConfigTemplateList(src, r, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplateList v1beta1 from v1beta2: %w", err)
	}
	return nil
}

// Convert_v1beta1_KThreesServerConfig_To_v1beta2_KThreesServerConfig converts KThreesServerConfig v1beta1 to v1beta2.
func Convert_v1beta1_KThreesServerConfig_To_v1beta2_KThreesServerConfig(in *KThreesServerConfig, out *bootstrapv1beta2.KThreesServerConfig, s conversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta1_KThreesServerConfig_To_v1beta2_KThreesServerConfig(in, out, s); err != nil {
		return fmt.Errorf("converting KThreesServerConfig v1beta1 to v1beta2: %w", err)
	}

	out.DeprecatedDisableExternalCloudProvider = in.DisableExternalCloudProvider

	if !in.DisableExternalCloudProvider {
		out.CloudProviderName = ptr.To("external")
		out.DisableCloudController = ptr.To(true)
	} else {
		out.CloudProviderName = ptr.To("")
		out.DisableCloudController = ptr.To(false)
	}

	return nil
}

// Convert_v1beta2_KThreesServerConfig_To_v1beta1_KThreesServerConfig converts KThreesServerConfig v1beta2 to v1beta1.
func Convert_v1beta2_KThreesServerConfig_To_v1beta1_KThreesServerConfig(in *bootstrapv1beta2.KThreesServerConfig, out *KThreesServerConfig, s conversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta2_KThreesServerConfig_To_v1beta1_KThreesServerConfig(in, out, s); err != nil {
		return fmt.Errorf("converting KThreesServerConfig v1beta2 to v1beta1: %w", err)
	}

	// DisableExternalCloudProvider=true is converted to CloudProviderName="" and DisableCloudController=false
	// when converting to v1beta2 (see Convert_v1beta1_KThreesServerConfig_To_v1beta2_KThreesServerConfig).
	// Newer API versions do not have DeprecatedDisableExternalCloudProvider anymore, so it is also inferred
	// from that combination of values to avoid losing it in round trips through the hub.
	out.DisableExternalCloudProvider = in.DeprecatedDisableExternalCloudProvider ||
		(in.CloudProviderName != nil && *in.CloudProviderName == "" &&
			in.DisableCloudController != nil && !*in.DisableCloudController)

	return nil
}

// Convert_v1beta2_KThreesAgentConfig_To_v1beta1_KThreesAgentConfig is an autogenerated conversion function.
func Convert_v1beta2_KThreesAgentConfig_To_v1beta1_KThreesAgentConfig(in *bootstrapv1beta2.KThreesAgentConfig, out *KThreesAgentConfig, s conversion.Scope) error { //nolint: stylecheck
	return autoConvert_v1beta2_KThreesAgentConfig_To_v1beta1_KThreesAgentConfig(in, out, s)
}

// Convert_v1beta2_KThreesConfigStatus_To_v1beta1_KThreesConfigStatus converts KThreesConfigStatus v1beta2 to v1beta1.
// NOTE: status.v1beta2 does not exist in v1beta1; it is restored from the conversion data in ConvertTo.
func Convert_v1beta2_KThreesConfigStatus_To_v1beta1_KThreesConfigStatus(in *bootstrapv1beta2.KThreesConfigStatus, out *KThreesConfigStatus, s conversion.Scope) error { //nolint: stylecheck
	return autoConvert_v1beta2_KThreesConfigStatus_To_v1beta1_KThreesConfigStatus(in, out, s)
}
