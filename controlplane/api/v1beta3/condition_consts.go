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

import clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

// KThreesControlPlane's Available condition and corresponding reasons.
const (
	// KThreesControlPlaneAvailableCondition is true if KThreesControlPlane is not deleted, it is initialized
	// and at least one control plane Machine reports a Ready Node.
	KThreesControlPlaneAvailableCondition = clusterv1.AvailableCondition

	// KThreesControlPlaneAvailableReason surfaces when the KThreesControlPlane is available.
	KThreesControlPlaneAvailableReason = clusterv1.AvailableReason

	// KThreesControlPlaneNotAvailableReason surfaces when the KThreesControlPlane is not available.
	KThreesControlPlaneNotAvailableReason = clusterv1.NotAvailableReason
)

// KThreesControlPlane's Initialized condition and corresponding reasons.
const (
	// KThreesControlPlaneInitializedCondition is true when the control plane is functional enough to accept
	// requests. This information is usually used as a signal for starting all the provisioning operations
	// that depend on a functional API server, but do not require a full HA control plane to exist.
	KThreesControlPlaneInitializedCondition = "Initialized"

	// KThreesControlPlaneInitializedReason surfaces when the control plane is initialized.
	KThreesControlPlaneInitializedReason = "Initialized"

	// KThreesControlPlaneNotInitializedReason surfaces when the control plane is not initialized.
	KThreesControlPlaneNotInitializedReason = "NotInitialized"
)

// KThreesControlPlane's CertificatesAvailable condition and corresponding reasons.
const (
	// KThreesControlPlaneCertificatesAvailableCondition is true if KThreesControlPlane certificates exist.
	KThreesControlPlaneCertificatesAvailableCondition = "CertificatesAvailable"

	// KThreesControlPlaneCertificatesAvailableReason surfaces when the KThreesControlPlane certificates exist.
	KThreesControlPlaneCertificatesAvailableReason = clusterv1.AvailableReason

	// KThreesControlPlaneCertificatesInternalErrorReason surfaces unexpected failures when reconciling
	// the KThreesControlPlane certificates.
	KThreesControlPlaneCertificatesInternalErrorReason = clusterv1.InternalErrorReason
)

// KThreesControlPlane's EtcdClusterHealthy condition and corresponding reasons.
const (
	// KThreesControlPlaneEtcdClusterHealthyCondition surfaces issues to etcd cluster hosted on machines managed by this object.
	// It is computed as aggregation of Machine's EtcdMemberHealthy conditions plus additional checks validating
	// potential issues to etcd quorum.
	KThreesControlPlaneEtcdClusterHealthyCondition = "EtcdClusterHealthy"

	// KThreesControlPlaneEtcdClusterInspectionFailedReason documents a failure when inspecting the status of the
	// etcd cluster hosted on KThreesControlPlane controlled machines.
	KThreesControlPlaneEtcdClusterInspectionFailedReason = clusterv1.InspectionFailedReason

	// KThreesControlPlaneEtcdClusterHealthyReason surfaces when the etcd cluster hosted on KThreesControlPlane
	// machines is healthy.
	KThreesControlPlaneEtcdClusterHealthyReason = "Healthy"

	// KThreesControlPlaneEtcdClusterNotHealthyReason surfaces when the etcd cluster hosted on KThreesControlPlane
	// machines is not healthy.
	KThreesControlPlaneEtcdClusterNotHealthyReason = "NotHealthy"

	// KThreesControlPlaneEtcdClusterHealthUnknownReason surfaces when the health status of the etcd cluster hosted
	// on KThreesControlPlane machines is unknown.
	KThreesControlPlaneEtcdClusterHealthUnknownReason = "HealthUnknown"
)

// KThreesControlPlane's ControlPlaneComponentsHealthy condition and corresponding reasons.
const (
	// KThreesControlPlaneControlPlaneComponentsHealthyCondition surfaces issues to control plane components
	// hosted on machines managed by this object. It is computed as aggregation of Machine's AgentHealthy conditions.
	KThreesControlPlaneControlPlaneComponentsHealthyCondition = "ControlPlaneComponentsHealthy"

	// KThreesControlPlaneControlPlaneComponentsInspectionFailedReason documents a failure when inspecting the status of the
	// control plane components hosted on KThreesControlPlane controlled machines.
	KThreesControlPlaneControlPlaneComponentsInspectionFailedReason = clusterv1.InspectionFailedReason

	// KThreesControlPlaneControlPlaneComponentsHealthyReason surfaces when the control plane components hosted on
	// KThreesControlPlane machines are healthy.
	KThreesControlPlaneControlPlaneComponentsHealthyReason = "Healthy"

	// KThreesControlPlaneControlPlaneComponentsNotHealthyReason surfaces when the control plane components hosted on
	// KThreesControlPlane machines are not healthy.
	KThreesControlPlaneControlPlaneComponentsNotHealthyReason = "NotHealthy"

	// KThreesControlPlaneControlPlaneComponentsHealthUnknownReason surfaces when the health status of the control
	// plane components hosted on KThreesControlPlane machines is unknown.
	KThreesControlPlaneControlPlaneComponentsHealthUnknownReason = "HealthUnknown"
)

// KThreesControlPlane's MachinesReady condition and corresponding reasons.
const (
	// KThreesControlPlaneMachinesReadyCondition surfaces detail of issues on the controlled machines, if any.
	KThreesControlPlaneMachinesReadyCondition = clusterv1.MachinesReadyCondition

	// KThreesControlPlaneMachinesReadyReason surfaces when all the controlled machine's Ready conditions are true.
	KThreesControlPlaneMachinesReadyReason = clusterv1.ReadyReason

	// KThreesControlPlaneMachinesNotReadyReason surfaces when at least one of the controlled machine's Ready conditions is false.
	KThreesControlPlaneMachinesNotReadyReason = clusterv1.NotReadyReason

	// KThreesControlPlaneMachinesReadyUnknownReason surfaces when at least one of the controlled machine's Ready conditions is unknown
	// and none of the controlled machine's Ready conditions is false.
	KThreesControlPlaneMachinesReadyUnknownReason = clusterv1.ReadyUnknownReason

	// KThreesControlPlaneMachinesReadyNoReplicasReason surfaces when no machines exist for the KThreesControlPlane.
	KThreesControlPlaneMachinesReadyNoReplicasReason = clusterv1.NoReplicasReason

	// KThreesControlPlaneMachinesReadyInternalErrorReason surfaces unexpected failures when computing the MachinesReady condition.
	KThreesControlPlaneMachinesReadyInternalErrorReason = clusterv1.InternalErrorReason
)

// KThreesControlPlane's RollingOut condition and corresponding reasons.
const (
	// KThreesControlPlaneRollingOutCondition is true if there is at least one machine not up-to-date.
	KThreesControlPlaneRollingOutCondition = clusterv1.RollingOutCondition

	// KThreesControlPlaneRollingOutReason surfaces when there is at least one machine not up-to-date.
	KThreesControlPlaneRollingOutReason = clusterv1.RollingOutReason

	// KThreesControlPlaneNotRollingOutReason surfaces when all the machines are up-to-date.
	KThreesControlPlaneNotRollingOutReason = clusterv1.NotRollingOutReason
)

// KThreesControlPlane's ScalingUp condition and corresponding reasons.
const (
	// KThreesControlPlaneScalingUpCondition is true if actual replicas < desired replicas.
	KThreesControlPlaneScalingUpCondition = clusterv1.ScalingUpCondition

	// KThreesControlPlaneScalingUpReason surfaces when actual replicas < desired replicas.
	KThreesControlPlaneScalingUpReason = clusterv1.ScalingUpReason

	// KThreesControlPlaneNotScalingUpReason surfaces when actual replicas >= desired replicas.
	KThreesControlPlaneNotScalingUpReason = clusterv1.NotScalingUpReason

	// KThreesControlPlaneScalingUpWaitingForReplicasSetReason surfaces when the .spec.replicas
	// field of the KThreesControlPlane is not set.
	KThreesControlPlaneScalingUpWaitingForReplicasSetReason = clusterv1.WaitingForReplicasSetReason
)

// KThreesControlPlane's ScalingDown condition and corresponding reasons.
const (
	// KThreesControlPlaneScalingDownCondition is true if actual replicas > desired replicas.
	KThreesControlPlaneScalingDownCondition = clusterv1.ScalingDownCondition

	// KThreesControlPlaneScalingDownReason surfaces when actual replicas > desired replicas.
	KThreesControlPlaneScalingDownReason = clusterv1.ScalingDownReason

	// KThreesControlPlaneNotScalingDownReason surfaces when actual replicas <= desired replicas.
	KThreesControlPlaneNotScalingDownReason = clusterv1.NotScalingDownReason

	// KThreesControlPlaneScalingDownWaitingForReplicasSetReason surfaces when the .spec.replicas
	// field of the KThreesControlPlane is not set.
	KThreesControlPlaneScalingDownWaitingForReplicasSetReason = clusterv1.WaitingForReplicasSetReason
)

// Reasons that will be used for the OwnerRemediated condition set by MachineHealthCheck on KThreesControlPlane controlled machines
// being remediated in v1beta2 API version.
const (
	// KThreesControlPlaneMachineRemediationInternalErrorReason surfaces unexpected failures while remediating a control plane machine.
	KThreesControlPlaneMachineRemediationInternalErrorReason = clusterv1.InternalErrorReason

	// KThreesControlPlaneMachineCannotBeRemediatedReason surfaces when remediation of a control plane machine can't be started.
	KThreesControlPlaneMachineCannotBeRemediatedReason = "CannotBeRemediated"

	// KThreesControlPlaneMachineRemediationDeferredReason surfaces when remediation of a control plane machine must be deferred.
	KThreesControlPlaneMachineRemediationDeferredReason = "RemediationDeferred"

	// KThreesControlPlaneMachineRemediationMachineDeletingReason surfaces when remediation of a control plane machine
	// has been completed by deleting the unhealthy machine.
	// Note: After an unhealthy machine is deleted, a new one is created by the KThreesControlPlane as part of the
	// regular reconcile loop that ensures the correct number of replicas exist.
	KThreesControlPlaneMachineRemediationMachineDeletingReason = "MachineDeleting"
)

// KThreesControlPlane's Deleting condition and corresponding reasons.
const (
	// KThreesControlPlaneDeletingCondition surfaces details about ongoing deletion of the controlled machines.
	KThreesControlPlaneDeletingCondition = clusterv1.DeletingCondition

	// KThreesControlPlaneNotDeletingReason surfaces when the KThreesControlPlane is not deleting because the
	// DeletionTimestamp is not set.
	KThreesControlPlaneNotDeletingReason = clusterv1.NotDeletingReason

	// KThreesControlPlaneDeletingWaitingForWorkersDeletionReason surfaces when the KThreesControlPlane deletion
	// waits for the workers to be deleted.
	KThreesControlPlaneDeletingWaitingForWorkersDeletionReason = "WaitingForWorkersDeletion"

	// KThreesControlPlaneDeletingWaitingForMachineDeletionReason surfaces when the KThreesControlPlane deletion
	// waits for the control plane Machines to be deleted.
	KThreesControlPlaneDeletingWaitingForMachineDeletionReason = "WaitingForMachineDeletion"

	// KThreesControlPlaneDeletingDeletionCompletedReason surfaces when the KThreesControlPlane deletion has been completed.
	// This reason is set right after the corresponding finalizer is removed.
	// This means that the object will go away (i.e. be removed from etcd), except if there are other
	// finalizers on the KThreesControlPlane object.
	KThreesControlPlaneDeletingDeletionCompletedReason = clusterv1.DeletionCompletedReason

	// KThreesControlPlaneDeletingInternalErrorReason surfaces unexpected failures when deleting a KThreesControlPlane object.
	KThreesControlPlaneDeletingInternalErrorReason = clusterv1.InternalErrorReason
)

// Conditions that will be used for the KThreesControlPlane controlled machines.
const (
	// MachineAgentHealthyCondition reports a machine's k3s agent's operational status,
	// based on the Ready condition of the corresponding Node.
	MachineAgentHealthyCondition = "AgentHealthy"

	// NodeReadyReason documents that the node hosting this machine is in Ready state.
	NodeReadyReason = "NodeReady"

	// PodProvisioningReason documents a pod waiting to be provisioned i.e., Pod is in "Pending" phase.
	PodProvisioningReason = "PodProvisioning"

	// PodMissingReason documents a pod does not exist.
	PodMissingReason = "PodMissing"

	// PodFailedReason documents if a pod failed during provisioning i.e., e.g CrashLoopbackOff, ImagePullBackOff
	// or if all the containers in a pod have terminated.
	PodFailedReason = "PodFailed"

	// PodInspectionFailedReason documents a failure in inspecting the pod status.
	PodInspectionFailedReason = "PodInspectionFailed"

	// MachineEtcdMemberHealthyCondition reports the machine's etcd member's health status.
	// NOTE: This conditions exists only if a stacked etcd cluster is used.
	MachineEtcdMemberHealthyCondition = "EtcdMemberHealthy"

	// EtcdMemberInspectionFailedReason documents a failure in inspecting the etcd member status.
	EtcdMemberInspectionFailedReason = "MemberInspectionFailed"

	// EtcdMemberUnhealthyReason documents a Machine's etcd member is unhealthy.
	EtcdMemberUnhealthyReason = "EtcdMemberUnhealthy"

	// EtcdMemberHealthyReason documents that the etcd member on this machine is healthy.
	EtcdMemberHealthyReason = "EtcdMemberHealthy"
)

// Conditions and condition Reasons for the KThreesControlPlane object, stored in status.deprecated.v1beta1.conditions.
//
// Deprecated: These conditions are only kept for v1beta1 compatibility and will be removed together with
// status.deprecated.v1beta1.

const (
	// MachinesReadyV1Beta1Condition reports an aggregate of current status of the machines controlled by the KThreesControlPlane.
	MachinesReadyV1Beta1Condition clusterv1.ConditionType = "MachinesReady"
)

const (
	// CertificatesAvailableV1Beta1Condition documents that cluster certificates were generated as part of the
	// processing of a KThreesControlPlane object.
	CertificatesAvailableV1Beta1Condition clusterv1.ConditionType = "CertificatesAvailable"

	// CertificatesGenerationFailedV1Beta1Reason (Severity=Warning) documents a KThreesControlPlane controller detecting
	// an error while generating certificates; those kind of errors are usually temporary and the controller
	// automatically recover from them.
	CertificatesGenerationFailedV1Beta1Reason = "CertificatesGenerationFailed"
)

const (
	// AvailableV1Beta1Condition documents that the first control plane instance has completed the server init operation
	// and so the control plane is available and an API server instance is ready for processing requests.
	AvailableV1Beta1Condition clusterv1.ConditionType = "Available"

	// WaitingForKthreesServerV1Beta1Reason (Severity=Info) documents a KThreesControlPlane object waiting for the first
	// control plane instance to complete the k3s server operation.
	WaitingForKthreesServerV1Beta1Reason = "WaitingForKthreesServer"
)

const (
	// MachinesSpecUpToDateV1Beta1Condition documents that the spec of the machines controlled by the KThreesControlPlane
	// is up to date. Whe this condition is false, the KThreesControlPlane is executing a rolling upgrade.
	MachinesSpecUpToDateV1Beta1Condition clusterv1.ConditionType = "MachinesSpecUpToDate"

	// RollingUpdateInProgressV1Beta1Reason (Severity=Warning) documents a KThreesControlPlane object executing a
	// rolling upgrade for aligning the machines spec to the desired state.
	RollingUpdateInProgressV1Beta1Reason = "RollingUpdateInProgress"
)

const (
	// ResizedV1Beta1Condition documents a KThreesControlPlane that is resizing the set of controlled machines.
	ResizedV1Beta1Condition clusterv1.ConditionType = "Resized"

	// ScalingUpV1Beta1Reason (Severity=Info) documents a KThreesControlPlane that is increasing the number of replicas.
	ScalingUpV1Beta1Reason = "ScalingUp"

	// ScalingDownV1Beta1Reason (Severity=Info) documents a KThreesControlPlane that is decreasing the number of replicas.
	ScalingDownV1Beta1Reason = "ScalingDown"
)

const (
	// ControlPlaneComponentsHealthyV1Beta1Condition reports the overall status of the k3s server.
	ControlPlaneComponentsHealthyV1Beta1Condition clusterv1.ConditionType = "ControlPlaneComponentsHealthy"

	// ControlPlaneComponentsUnhealthyV1Beta1Reason (Severity=Error) documents a control plane component not healthy.
	ControlPlaneComponentsUnhealthyV1Beta1Reason = "ControlPlaneComponentsUnhealthy"

	// ControlPlaneComponentsUnknownV1Beta1Reason reports a control plane component in unknown status.
	ControlPlaneComponentsUnknownV1Beta1Reason = "ControlPlaneComponentsUnknown"

	// ControlPlaneComponentsInspectionFailedV1Beta1Reason documents a failure in inspecting the control plane component status.
	ControlPlaneComponentsInspectionFailedV1Beta1Reason = "ControlPlaneComponentsInspectionFailed"
)

const (
	// EtcdClusterHealthyV1Beta1Condition documents the overall etcd cluster's health.
	EtcdClusterHealthyV1Beta1Condition clusterv1.ConditionType = "EtcdClusterHealthyCondition"

	// EtcdClusterInspectionFailedV1Beta1Reason documents a failure in inspecting the etcd cluster status.
	EtcdClusterInspectionFailedV1Beta1Reason = "EtcdClusterInspectionFailed"

	// EtcdClusterUnknownV1Beta1Reason reports an etcd cluster in unknown status.
	EtcdClusterUnknownV1Beta1Reason = "EtcdClusterUnknown"

	// EtcdClusterUnhealthyV1Beta1Reason (Severity=Error) is set when the etcd cluster is unhealthy.
	EtcdClusterUnhealthyV1Beta1Reason = "EtcdClusterUnhealthy"
)

const (
	// TokenAvailableV1Beta1Condition documents whether the token required for nodes to join the cluster is available.
	TokenAvailableV1Beta1Condition clusterv1.ConditionType = "TokenAvailable"

	// TokenGenerationFailedV1Beta1Reason documents that the token required for nodes to join the cluster could not be generated.
	TokenGenerationFailedV1Beta1Reason = "TokenGenerationFailed"
)
