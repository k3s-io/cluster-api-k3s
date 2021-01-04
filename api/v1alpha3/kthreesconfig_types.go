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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KThreesConfigSpec defines the desired state of KThreesConfig
type KThreesConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of KThreesConfig. Edit KThreesConfig_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// KThreesConfigStatus defines the observed state of KThreesConfig
type KThreesConfigStatus struct {
	// Ready indicates the BootstrapData field is ready to be consumed
	Ready bool `json:"ready,omitempty"`

	BootstrapData []byte `json:"bootstrapData,omitempty"`

	// DataSecretName is the name of the secret that stores the bootstrap data script.
	// +optional
	DataSecretName *string `json:"dataSecretName,omitempty"`

	// FailureReason will be set on non-retryable errors
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// FailureMessage will be set on non-retryable errors
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the KubeadmConfig.
	// +optional
	//Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KThreesConfig is the Schema for the kthreesconfigs API
type KThreesConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KThreesConfigSpec   `json:"spec,omitempty"`
	Status KThreesConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KThreesConfigList contains a list of KThreesConfig
type KThreesConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KThreesConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KThreesConfig{}, &KThreesConfigList{})
}
