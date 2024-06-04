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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	bootstrapv1beta2 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/errors"
)

const (
	KThreesControlPlaneFinalizer = "kthrees.controlplane.cluster.x-k8s.io"

	// KThreesServerConfigurationAnnotation is a machine annotation that stores the json-marshalled string of KCP ClusterConfiguration.
	// This annotation is used to detect any changes in ClusterConfiguration and trigger machine rollout in KCP.
	KThreesServerConfigurationAnnotation = "controlplane.cluster.x-k8s.io/kthrees-server-configuration"

	// SkipCoreDNSAnnotation annotation explicitly skips reconciling CoreDNS if set.
	SkipCoreDNSAnnotation = "controlplane.cluster.x-k8s.io/skip-coredns"

	// RemediationInProgressAnnotation is used to keep track that a KCP remediation is in progress, and more
	// specifically it tracks that the system is in between having deleted an unhealthy machine and recreating its replacement.
	// NOTE: if something external to CAPI removes this annotation the system cannot detect the above situation; this can lead to
	// failures in updating remediation retry or remediation count (both counters restart from zero).
	RemediationInProgressAnnotation = "controlplane.cluster.x-k8s.io/remediation-in-progress"

	// RemediationForAnnotation is used to link a new machine to the unhealthy machine it is replacing;
	// please note that in case of retry, when also the remediating machine fails, the system keeps track of
	// the first machine of the sequence only.
	// NOTE: if something external to CAPI removes this annotation the system this can lead to
	// failures in updating remediation retry (the counter restarts from zero).
	RemediationForAnnotation = "controlplane.cluster.x-k8s.io/remediation-for"

	// DefaultMinHealthyPeriod defines the default minimum period before we consider a remediation on a
	// machine unrelated from the previous remediation.
	DefaultMinHealthyPeriod = 1 * time.Hour
)

// KThreesControlPlaneSpec defines the desired state of KThreesControlPlane.
type KThreesControlPlaneSpec struct {
	// Number of desired machines. Defaults to 1. When stacked etcd is used only
	// odd numbers are permitted, as per [etcd best practice](https://etcd.io/docs/v3.3.12/faq/#why-an-odd-number-of-cluster-members).
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Version defines the desired Kubernetes version.
	Version string `json:"version"`

	// KThreesConfigSpec is a KThreesConfigSpec
	// to use for initializing and joining machines to the control plane.
	// +optional
	KThreesConfigSpec bootstrapv1beta2.KThreesConfigSpec `json:"kthreesConfigSpec,omitempty"`

	// RolloutAfter is a field to indicate a rollout should be performed
	// after the specified time even if no changes have been made to the
	// KThreesControlPlane
	// +optional
	RolloutAfter *metav1.Time `json:"rolloutAfter,omitempty"`

	// MachineTemplate contains information about how machines should be shaped
	// when creating or updating a control plane.
	MachineTemplate KThreesControlPlaneMachineTemplate `json:"machineTemplate,omitempty"`

	// The RemediationStrategy that controls how control plane machine remediation happens.
	// +optional
	RemediationStrategy *RemediationStrategy `json:"remediationStrategy,omitempty"`
}

// MachineTemplate contains information about how machines should be shaped
// when creating or updating a control plane.
type KThreesControlPlaneMachineTemplate struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`
	// InfrastructureRef is a required reference to a custom resource
	// offered by an infrastructure provider.
	InfrastructureRef corev1.ObjectReference `json:"infrastructureRef"`
	// NodeDrainTimeout is the total amount of time that the controller will spend on draining a controlplane node
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
	// +optional
	NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`
	// NodeVolumeDetachTimeout is the total amount of time that the controller will spend on waiting for all volumes
	// to be detached. The default value is 0, meaning that the volumes can be detached without any time limitations.
	// +optional
	NodeVolumeDetachTimeout *metav1.Duration `json:"nodeVolumeDetachTimeout,omitempty"`
	// NodeDeletionTimeout defines how long the machine controller will attempt to delete the Node that the Machine
	// hosts after the Machine is marked for deletion. A duration of 0 will retry deletion indefinitely.
	// If no value is provided, the default value for this property of the Machine resource will be used.
	// +optional
	NodeDeletionTimeout *metav1.Duration `json:"nodeDeletionTimeout,omitempty"`
}

// RemediationStrategy allows to define how control plane machine remediation happens.
type RemediationStrategy struct {
	// MaxRetry is the Max number of retries while attempting to remediate an unhealthy machine.
	// A retry happens when a machine that was created as a replacement for an unhealthy machine also fails.
	// For example, given a control plane with three machines M1, M2, M3:
	//
	//	M1 become unhealthy; remediation happens, and M1-1 is created as a replacement.
	//	If M1-1 (replacement of M1) has problems while bootstrapping it will become unhealthy, and then be
	//	remediated; such operation is considered a retry, remediation-retry #1.
	//	If M1-2 (replacement of M1-1) becomes unhealthy, remediation-retry #2 will happen, etc.
	//
	// A retry could happen only after RetryPeriod from the previous retry.
	// If a machine is marked as unhealthy after MinHealthyPeriod from the previous remediation expired,
	// this is not considered a retry anymore because the new issue is assumed unrelated from the previous one.
	//
	// If not set, the remedation will be retried infinitely.
	// +optional
	MaxRetry *int32 `json:"maxRetry,omitempty"`

	// RetryPeriod is the duration that KCP should wait before remediating a machine being created as a replacement
	// for an unhealthy machine (a retry).
	//
	// If not set, a retry will happen immediately.
	// +optional
	RetryPeriod metav1.Duration `json:"retryPeriod,omitempty"`

	// MinHealthyPeriod defines the duration after which KCP will consider any failure to a machine unrelated
	// from the previous one. In this case the remediation is not considered a retry anymore, and thus the retry
	// counter restarts from 0. For example, assuming MinHealthyPeriod is set to 1h (default)
	//
	//	M1 become unhealthy; remediation happens, and M1-1 is created as a replacement.
	//	If M1-1 (replacement of M1) has problems within the 1hr after the creation, also
	//	this machine will be remediated and this operation is considered a retry - a problem related
	//	to the original issue happened to M1 -.
	//
	//	If instead the problem on M1-1 is happening after MinHealthyPeriod expired, e.g. four days after
	//	m1-1 has been created as a remediation of M1, the problem on M1-1 is considered unrelated to
	//	the original issue happened to M1.
	//
	// If not set, this value is defaulted to 1h.
	// +optional
	MinHealthyPeriod *metav1.Duration `json:"minHealthyPeriod,omitempty"`
}

// KThreesControlPlaneStatus defines the observed state of KThreesControlPlane.
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

	// Version represents the minimum Kubernetes version for the control plane machines
	// in the cluster.
	// +optional
	Version *string `json:"version,omitempty"`

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

	// Initialized denotes whether or not the k3s server is initialized.
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

	// LastRemediation stores info about last remediation performed.
	// +optional
	LastRemediation *LastRemediationStatus `json:"lastRemediation,omitempty"`
}

// LastRemediationStatus  stores info about last remediation performed.
// NOTE: if for any reason information about last remediation are lost, RetryCount is going to restart from 0 and thus
// more remediations than expected might happen.
type LastRemediationStatus struct {
	// Machine is the machine name of the latest machine being remediated.
	Machine string `json:"machine"`

	// Timestamp is when last remediation happened. It is represented in RFC3339 form and is in UTC.
	Timestamp metav1.Time `json:"timestamp"`

	// RetryCount used to keep track of remediation retry for the last remediated machine.
	// A retry happens when a machine that was created as a replacement for an unhealthy machine also fails.
	RetryCount int32 `json:"retryCount"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Initialized",type=boolean,JSONPath=".status.initialized",description="This denotes whether or not the control plane has completed the k3s server initialization"
// +kubebuilder:printcolumn:name="API Server Available",type=boolean,JSONPath=".status.ready",description="KThreesControlPlane API Server is ready to receive requests"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.version",description="Kubernetes version associated with this control plane"
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=".status.replicas",description="Total number of non-terminated machines targeted by this control plane"
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=".status.readyReplicas",description="Total number of fully running and ready control plane machines"
// +kubebuilder:printcolumn:name="Updated",type=integer,JSONPath=".status.updatedReplicas",description="Total number of non-terminated machines targeted by this control plane that have the desired template spec"
// +kubebuilder:printcolumn:name="Unavailable",type=integer,JSONPath=".status.unavailableReplicas",description="Total number of unavailable machines targeted by this control plane"

// KThreesControlPlane is the Schema for the kthreescontrolplanes API.
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

// KThreesControlPlaneList contains a list of KThreesControlPlane.
type KThreesControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KThreesControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KThreesControlPlane{}, &KThreesControlPlaneList{})
}
