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
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	ctrlconversion "sigs.k8s.io/controller-runtime/pkg/conversion"

	bootstrapv1beta2 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
)

// ConvertTo converts the v1beta1 KThreesConfig receiver to a v1beta2 KThreesConfig.
func (c *KThreesConfig) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1beta2.KThreesConfig)
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
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfig receiver from a v1beta2 KThreesConfig.
func (c *KThreesConfig) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1beta2.KThreesConfig)
	if err := Convert_v1beta2_KThreesConfig_To_v1beta1_KThreesConfig(src, c, nil); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta1 from v1beta2: %w", err)
	}

	if err := utilconversion.MarshalData(src, c); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta1 KThreesConfigList receiver to a v1beta2 KThreesConfigList.
func (c *KThreesConfigList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1beta2.KThreesConfigList)
	if err := autoConvert_v1beta1_KThreesConfigList_To_v1beta2_KThreesConfigList(c, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigList v1beta1 to v1beta2: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfigList receiver from a v1beta2 KThreesConfigList.
func (c *KThreesConfigList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1beta2.KThreesConfigList)
	if err := autoConvert_v1beta2_KThreesConfigList_To_v1beta1_KThreesConfigList(src, c, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigList v1beta1 from v1beta2: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta1 KThreesConfigTemplate receiver to a v1beta2 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1beta2.KThreesConfigTemplate)
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
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfigTemplate receiver from a v1beta2 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1beta2.KThreesConfigTemplate)
	if err := autoConvert_v1beta2_KThreesConfigTemplate_To_v1beta1_KThreesConfigTemplate(src, r, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta1 from v1beta2: %w", err)
	}

	if err := utilconversion.MarshalData(src, r); err != nil {
		return fmt.Errorf("storing conversion data: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta1 KThreesConfigTemplateList receiver to a v1beta2 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) ConvertTo(dstRaw ctrlconversion.Hub) error {
	dst := dstRaw.(*bootstrapv1beta2.KThreesConfigTemplateList)
	if err := autoConvert_v1beta1_KThreesConfigTemplateList_To_v1beta2_KThreesConfigTemplateList(r, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplateList v1beta1 to v1beta2: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfigTemplateList receiver from a v1beta2 KThreesConfigTemplateList.
func (r *KThreesConfigTemplateList) ConvertFrom(srcRaw ctrlconversion.Hub) error {
	src := srcRaw.(*bootstrapv1beta2.KThreesConfigTemplateList)
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
		out.CloudProviderName = "external"
		out.DisableCloudController = true
	} else {
		out.CloudProviderName = ""
		out.DisableCloudController = false
	}

	return nil
}

// Convert_v1beta2_KThreesServerConfig_To_v1beta1_KThreesServerConfig converts KThreesServerConfig v1beta2 to v1beta1.
func Convert_v1beta2_KThreesServerConfig_To_v1beta1_KThreesServerConfig(in *bootstrapv1beta2.KThreesServerConfig, out *KThreesServerConfig, s conversion.Scope) error { //nolint: stylecheck
	if err := autoConvert_v1beta2_KThreesServerConfig_To_v1beta1_KThreesServerConfig(in, out, s); err != nil {
		return fmt.Errorf("converting KThreesServerConfig v1beta2 to v1beta1: %w", err)
	}

	out.DisableExternalCloudProvider = in.DeprecatedDisableExternalCloudProvider

	return nil
}
