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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	bootstrapv1beta3 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta3"
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
	// replicas is the number of desired machines. Defaults to 1. When stacked etcd is used only
	// odd numbers are permitted, as per [etcd best practice](https://etcd.io/docs/v3.3.12/faq/#why-an-odd-number-of-cluster-members).
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// version defines the desired Kubernetes version.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Version string `json:"version,omitempty"`

	// machineTemplate contains information about how machines
	// should be shaped when creating or updating a control plane.
	// +required
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

// KThreesControlPlaneMachineTemplate defines the template for Machines
// in a KThreesControlPlane object.
type KThreesControlPlaneMachineTemplate struct {
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the spec for Machines
	// in a KThreesControlPlane object.
	// +required
	Spec KThreesControlPlaneMachineTemplateSpec `json:"spec,omitempty,omitzero"`
}

// KThreesControlPlaneMachineTemplateSpec defines the spec for Machines
// in a KThreesControlPlane object.
type KThreesControlPlaneMachineTemplateSpec struct {
	// infrastructureRef is a required reference to a custom resource
	// offered by an infrastructure provider.
	// +required
	InfrastructureRef clusterv1.ContractVersionedObjectReference `json:"infrastructureRef,omitempty,omitzero"`

	// deletion contains configuration options for Machine deletion.
	// +optional
	Deletion KThreesControlPlaneMachineTemplateDeletionSpec `json:"deletion,omitempty,omitzero"`
}

// KThreesControlPlaneMachineTemplateDeletionSpec contains configuration options for Machine deletion.
// +kubebuilder:validation:MinProperties=1
type KThreesControlPlaneMachineTemplateDeletionSpec struct {
	// nodeDrainTimeoutSeconds is the total amount of time that the controller will spend on draining a controlplane node
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: nodeDrainTimeoutSeconds is different from `kubectl drain --timeout`
	// +optional
	// +kubebuilder:validation:Minimum=0
	NodeDrainTimeoutSeconds *int32 `json:"nodeDrainTimeoutSeconds,omitempty"`

	// nodeVolumeDetachTimeoutSeconds is the total amount of time that the controller will spend on waiting for all volumes
	// to be detached. The default value is 0, meaning that the volumes can be detached without any time limitations.
	// +optional
	// +kubebuilder:validation:Minimum=0
	NodeVolumeDetachTimeoutSeconds *int32 `json:"nodeVolumeDetachTimeoutSeconds,omitempty"`

	// nodeDeletionTimeoutSeconds defines how long the machine controller will attempt to delete the Node that the Machine
	// hosts after the Machine is marked for deletion. A duration of 0 will retry deletion indefinitely.
	// If no value is provided, the default value for this property of the Machine resource will be used.
	// +optional
	// +kubebuilder:validation:Minimum=0
	NodeDeletionTimeoutSeconds *int32 `json:"nodeDeletionTimeoutSeconds,omitempty"`
}

// KThreesControlPlaneRolloutSpec allows you to configure the behaviour of rolling updates to the control plane Machines.
// +kubebuilder:validation:MinProperties=1
type KThreesControlPlaneRolloutSpec struct {
	// after is a field to indicate a rollout should be performed
	// after the specified time even if no changes have been made to the
	// KThreesControlPlane.
	// Example: In the YAML the time can be specified in the RFC3339 format.
	// To specify the rollout.after target as March 9, 2023, at 9 am UTC
	// use "2023-03-09T09:00:00Z".
	// +optional
	After metav1.Time `json:"after,omitempty,omitzero"`
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
// +kubebuilder:validation:MinProperties=1
type KThreesControlPlaneStatus struct {
	// conditions represents the observations of a KThreesControlPlane's current state.
	// Known condition types are Available, Initialized, CertificatesAvailable, EtcdClusterHealthy,
	// ControlPlaneComponentsHealthy, MachinesReady, ScalingUp, ScalingDown, Deleting.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// initialization provides observations of the KThreesControlPlane initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization KThreesControlPlaneInitializationStatus `json:"initialization,omitempty,omitzero"`

	// selector is the label selector in string format to avoid introspection
	// by clients, and is used to provide the CRD-based integration for the
	// scale subresource and additional integrations for things like kubectl
	// describe.. The string will be in the same format as the query-param syntax.
	// More info about label selectors: http://kubernetes.io/docs/user-guide/labels#label-selectors
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Selector string `json:"selector,omitempty"`

	// replicas is the total number of non-terminated machines targeted by this control plane
	// (their labels match the selector).
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// readyReplicas is the number of ready replicas for this KThreesControlPlane. A machine is considered ready when Machine's Ready condition is true.
	// +optional
	ReadyReplicas *int32 `json:"readyReplicas,omitempty"`

	// availableReplicas is the number of available replicas targeted by this KThreesControlPlane. A machine is considered available when Machine's Available condition is true.
	// +optional
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`

	// upToDateReplicas is the number of up-to-date replicas targeted by this KThreesControlPlane. A machine is considered up-to-date when Machine's UpToDate condition is true.
	// +optional
	UpToDateReplicas *int32 `json:"upToDateReplicas,omitempty"`

	// version represents the minimum Kubernetes version for the control plane machines
	// in the cluster.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Version string `json:"version,omitempty"`

	// observedGeneration is the latest generation observed by the controller.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastRemediation stores info about last remediation performed.
	// +optional
	LastRemediation *LastRemediationStatus `json:"lastRemediation,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.
	// +optional
	Deprecated *KThreesControlPlaneDeprecatedStatus `json:"deprecated,omitempty"`
}

// KThreesControlPlaneInitializationStatus provides observations of the KThreesControlPlane initialization process.
// +kubebuilder:validation:MinProperties=1
type KThreesControlPlaneInitializationStatus struct {
	// controlPlaneInitialized is true when the KThreesControlPlane provider reports that the Kubernetes control plane is initialized;
	// A control plane is considered initialized when it can accept requests, no matter if this happens before
	// the control plane is fully provisioned or not.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// +optional
	ControlPlaneInitialized *bool `json:"controlPlaneInitialized,omitempty"`
}

// KThreesControlPlaneDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type KThreesControlPlaneDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *KThreesControlPlaneV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// KThreesControlPlaneV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
type KThreesControlPlaneV1Beta1DeprecatedStatus struct {
	// conditions defines current service state of the KThreesControlPlane.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// failureReason indicates that there is a terminal problem reconciling the
	// state, and will be set to a token value suitable for
	// programmatic interpretation.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	FailureReason errors.KThreesControlPlaneStatusError `json:"failureReason,omitempty"`

	// failureMessage indicates that there is a terminal problem reconciling the
	// state, and will be set to a descriptive error message.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=10240
	FailureMessage *string `json:"failureMessage,omitempty"`

	// updatedReplicas is the total number of non-terminated machines targeted by this control plane
	// that have the desired template spec.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// readyReplicas is the total number of fully running and ready control plane machines.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	ReadyReplicas int32 `json:"readyReplicas"`

	// unavailableReplicas is the total number of unavailable machines targeted by this control plane.
	// This is the total number of machines that are still required for
	// the deployment to have 100% available capacity. They may either
	// be machines that are running but not yet ready or machines
	// that still have not been created.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas"`
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
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster"
// +kubebuilder:printcolumn:name="Available",type="string",JSONPath=`.status.conditions[?(@.type=="Available")].status`,description="Cluster pass all availability checks"
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=".spec.replicas",description="The desired number of machines"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.replicas",description="The number of machines"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="The number of machines with Ready condition true"
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=".status.availableReplicas",description="The number of machines with Available condition true"
// +kubebuilder:printcolumn:name="Up-to-date",type=integer,JSONPath=".status.upToDateReplicas",description="The number of machines with UpToDate condition true"
// +kubebuilder:printcolumn:name="Initialized",type=boolean,JSONPath=".status.initialization.controlPlaneInitialized",description="This denotes whether or not the control plane can accept requests"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of KThreesControlPlane"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.version",description="Kubernetes version associated with this control plane"

// KThreesControlPlane is the Schema for the kthreescontrolplanes API.
type KThreesControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KThreesControlPlaneSpec   `json:"spec,omitempty"`
	Status KThreesControlPlaneStatus `json:"status,omitempty,omitzero"`
}

// GetV1Beta1Conditions returns the set of conditions for this object.
func (in *KThreesControlPlane) GetV1Beta1Conditions() clusterv1.Conditions {
	if in.Status.Deprecated == nil || in.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return in.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the conditions on this object.
func (in *KThreesControlPlane) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if in.Status.Deprecated == nil {
		in.Status.Deprecated = &KThreesControlPlaneDeprecatedStatus{}
	}
	if in.Status.Deprecated.V1Beta1 == nil {
		in.Status.Deprecated.V1Beta1 = &KThreesControlPlaneV1Beta1DeprecatedStatus{}
	}
	in.Status.Deprecated.V1Beta1.Conditions = conditions
}

// GetConditions returns the set of conditions for this object.
func (in *KThreesControlPlane) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets conditions for an API object.
func (in *KThreesControlPlane) SetConditions(conditions []metav1.Condition) {
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
