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

package v1beta3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	bootstrapv1beta3 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta3"
)

// KThreesControlPlaneTemplateSpec defines the desired state of KThreesControlPlaneTemplateSpec.
type KThreesControlPlaneTemplateSpec struct {
	Template KThreesControlPlaneTemplateResource `json:"template"`
}

// KThreesControlPlaneTemplateResource describes the data needed to create a KThreesControlPlane from a template.
type KThreesControlPlaneTemplateResource struct {
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec is the desired state of KThreesControlPlaneTemplateResource.
	// +required
	Spec KThreesControlPlaneTemplateResourceSpec `json:"spec,omitempty,omitzero"`
}

// KThreesControlPlaneTemplateResourceSpec defines the desired state of KThreesControlPlane.
// NOTE: KThreesControlPlaneTemplateResourceSpec is similar to KThreesControlPlaneSpec but
// omits Replicas and Version fields. These fields do not make sense on the KThreesControlPlaneTemplate,
// because they are calculated by the Cluster topology reconciler during reconciliation and thus cannot
// be configured on the KThreesControlPlaneTemplate.
type KThreesControlPlaneTemplateResourceSpec struct {
	// machineTemplate contains information about how machines
	// should be shaped when creating or updating a control plane.
	// +optional
	MachineTemplate KThreesControlPlaneMachineTemplate `json:"machineTemplate,omitempty,omitzero"`

	// kthreesConfigSpec is a KThreesConfigSpec
	// to use for initializing and joining machines to the control plane.
	// +optional
	KThreesConfigSpec bootstrapv1beta3.KThreesConfigSpec `json:"kthreesConfigSpec,omitempty,omitzero"`

	// rollout allows you to configure the behaviour of rolling updates to the control plane Machines.
	// +optional
	Rollout KThreesControlPlaneRolloutSpec `json:"rollout,omitempty,omitzero"`

	// The RemediationStrategy that controls how control plane machine remediation happens.
	// +optional
	RemediationStrategy *RemediationStrategy `json:"remediationStrategy,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// KThreesControlPlaneTemplate is the Schema for the kthreescontrolplanetemplate API.
type KThreesControlPlaneTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KThreesControlPlaneTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// KThreesControlPlaneTemplateList contains a list of KThreesControlPlaneTemplate.
type KThreesControlPlaneTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KThreesControlPlaneTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &KThreesControlPlaneTemplate{}, &KThreesControlPlaneTemplateList{})
}
