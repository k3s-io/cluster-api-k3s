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
	"github.com/zawachte-msft/cluster-api-k3s/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	cabp3v1 "github.com/zawachte-msft/cluster-api-k3s/bootstrap/api/v1alpha3"
)

const (
	KThreesControlPlaneFinalizer = "kthrees.controlplane.cluster.x-k8s.io"

	// KThreesServerConfigurationAnnotation is a machine annotation that stores the json-marshalled string of KCP ClusterConfiguration.
	// This annotation is used to detect any changes in ClusterConfiguration and trigger machine rollout in KCP.
	KThreesServerConfigurationAnnotation = "controlplane.cluster.x-k8s.io/kthrees-server-configuration"

	// SkipCoreDNSAnnotation annotation explicitly skips reconciling CoreDNS if set
	SkipCoreDNSAnnotation = "controlplane.cluster.x-k8s.io/skip-coredns"
)

// KThreesControlPlaneSpec defines the desired state of KThreesControlPlane
type KThreesControlPlaneSpec struct {
	// Number of desired machines. Defaults to 1. When stacked etcd is used only
	// odd numbers are permitted, as per [etcd best practice](https://etcd.io/docs/v3.3.12/faq/#why-an-odd-number-of-cluster-members).
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Version defines the desired Kubernetes version.
	Version string `json:"version"`

	// InfrastructureTemplate is a required reference to a custom resource
	// offered by an infrastructure provider.
	InfrastructureTemplate corev1.ObjectReference `json:"infrastructureTemplate"`

	// KThreesConfigSpec is a KThreesConfigSpec
	// to use for initializing and joining machines to the control plane.
	KThreesConfigSpec cabp3v1.KThreesConfigSpec `json:"kthreesConfigSpec"`

	// UpgradeAfter is a field to indicate an upgrade should be performed
	// after the specified time even if no changes have been made to the
	// KThreesControlPlane
	// +optional
	UpgradeAfter *metav1.Time `json:"upgradeAfter,omitempty"`

	// NodeDrainTimeout is the total amount of time that the controller will spend on draining a controlplane node
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
	// +optional
	NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`
}

// KThreesControlPlaneStatus defines the observed state of KThreesControlPlane
type KThreesControlPlaneStatus struct {
	// Selector is the label selector in string format to avoid introspection
	// by clients, and is used to provide the CRD-based integration for the
	// scale subresource and additional integrations for things like kubectl
	// describe.. The string will be in the same format as the query-param syntax.
	// More info about label selectors: http://kubernetes.io/docs/user-guide/labels#label-selectors
	// +optional
	Selector string `json:"selector,omitempty"`

	// Total number of non-terminated machines targeted by this control plane
	// (their labels match the selector).
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Total number of non-terminated machines targeted by this control plane
	// that have the desired template spec.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Total number of fully running and ready control plane machines.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Total number of unavailable machines targeted by this control plane.
	// This is the total number of machines that are still required for
	// the deployment to have 100% available capacity. They may either
	// be machines that are running but not yet ready or machines
	// that still have not been created.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`

	// Initialized denotes whether or not the control plane has the
	// uploaded kubeadm-config configmap.
	// +optional
	Initialized bool `json:"initialized"`

	// Ready denotes that the KThreesControlPlane API Server is ready to
	// receive requests.
	// +optional
	Ready bool `json:"ready"`

	// FailureReason indicates that there is a terminal problem reconciling the
	// state, and will be set to a token value suitable for
	// programmatic interpretation.
	// +optional
	FailureReason errors.KThreesControlPlaneStatusError `json:"failureReason,omitempty"`

	// ErrorMessage indicates that there is a terminal problem reconciling the
	// state, and will be set to a descriptive error message.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the KThreesControlPlane.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Initialized",type=boolean,JSONPath=".status.initialized",description="This denotes whether or not the control plane has the uploaded kubeadm-config configmap"
// +kubebuilder:printcolumn:name="API Server Available",type=boolean,JSONPath=".status.ready",description="KThreesControlPlane API Server is ready to receive requests"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.version",description="Kubernetes version associated with this control plane"
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=".status.replicas",description="Total number of non-terminated machines targeted by this control plane"
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=".status.readyReplicas",description="Total number of fully running and ready control plane machines"
// +kubebuilder:printcolumn:name="Updated",type=integer,JSONPath=".status.updatedReplicas",description="Total number of non-terminated machines targeted by this control plane that have the desired template spec"
// +kubebuilder:printcolumn:name="Unavailable",type=integer,JSONPath=".status.unavailableReplicas",description="Total number of unavailable machines targeted by this control plane"

// KThreesControlPlane is the Schema for the kthreescontrolplanes API
type KThreesControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KThreesControlPlaneSpec   `json:"spec,omitempty"`
	Status KThreesControlPlaneStatus `json:"status,omitempty"`
}

func (in *KThreesControlPlane) GetConditions() clusterv1.Conditions {
	return in.Status.Conditions
}

func (in *KThreesControlPlane) SetConditions(conditions clusterv1.Conditions) {
	in.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// KThreesControlPlaneList contains a list of KThreesControlPlane
type KThreesControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KThreesControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KThreesControlPlane{}, &KThreesControlPlaneList{})
}
