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

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	cabp3v1 "github.com/cluster-api-provider-k3s/cluster-api-k3s/bootstrap/api/v1beta2"
)

// ConvertTo converts the v1beta1 KThreesConfig receiver to a v1beta2 KThreesConfig.
func (c *KThreesConfig) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*cabp3v1.KThreesConfig)
	if err := autoConvert_v1beta1_KThreesConfig_To_v1beta2_KThreesConfig(c, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta1 to v1beta2: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfig receiver from a v1beta2 KThreesConfig.
func (c *KThreesConfig) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*cabp3v1.KThreesConfig)
	if err := autoConvert_v1beta2_KThreesConfig_To_v1beta1_KThreesConfig(src, c, nil); err != nil {
		return fmt.Errorf("converting KThreesConfig v1beta1 from v1beta2: %w", err)
	}
	return nil
}

// ConvertTo converts the v1beta1 KThreesConfigTemplate receiver to a v1beta2 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*cabp3v1.KThreesConfigTemplate)
	if err := autoConvert_v1beta1_KThreesConfigTemplate_To_v1beta2_KThreesConfigTemplate(r, dst, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta1 to v1beta2: %w", err)
	}
	return nil
}

// ConvertFrom converts the v1beta1 KThreesConfigTemplate receiver from a v1beta2 KThreesConfigTemplate.
func (r *KThreesConfigTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*cabp3v1.KThreesConfigTemplate)
	if err := autoConvert_v1beta2_KThreesConfigTemplate_To_v1beta1_KThreesConfigTemplate(src, r, nil); err != nil {
		return fmt.Errorf("converting KThreesConfigTemplate v1beta1 from v1beta2: %w", err)
	}
	return nil
}
