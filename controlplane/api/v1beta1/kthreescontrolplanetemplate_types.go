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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KThreesControlPlaneTemplateSpec defines the desired state of KThreesControlPlaneTemplateSpec.
type KThreesControlPlaneTemplateSpec struct {
	Template KThreesControlPlaneTemplateResource `json:"template"`
}

type KThreesControlPlaneTemplateResource struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta metav1.ObjectMeta       `json:"metadata,omitempty"`
	Spec       KThreesControlPlaneSpec `json:"spec"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KThreesControlPlaneTemplate is the Schema for the kthreescontrolplanetemplate API.
type KThreesControlPlaneTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KThreesControlPlaneTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// KThreesControlPlaneTemplateList contains a list of KThreesControlPlaneTemplate.
type KThreesControlPlaneTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KThreesControlPlaneTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KThreesControlPlaneTemplate{}, &KThreesControlPlaneTemplateList{})
}
