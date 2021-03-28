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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KThreesConfigTemplateSpec defines the desired state of KThreesConfigTemplate
type KThreesConfigTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Template KThreesConfigTemplateResource `json:"template"`
}

// KThreesConfigTemplateResource defines the Template structure
type KThreesConfigTemplateResource struct {
	Spec KThreesConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// KThreesConfigTemplate is the Schema for the kthreesconfigtemplates API
type KThreesConfigTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KThreesConfigTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// KThreesConfigTemplateList contains a list of KThreesConfigTemplate
type KThreesConfigTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KThreesConfigTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KThreesConfigTemplate{}, &KThreesConfigTemplateList{})
}
