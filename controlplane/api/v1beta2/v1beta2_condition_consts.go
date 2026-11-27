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

import clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

// Conditions that will be used for the KThreesControlPlane object in status.v1beta2.conditions.
// NOTE: These conditions follow the Cluster API v1beta2 conventions (metav1.Condition), and they will
// become the main conditions when moving to the next API version.

// KThreesControlPlane's Available condition and corresponding reasons.
const (
	// KThreesControlPlaneAvailableV1Beta2Condition is true if KThreesControlPlane is not deleted, it is initialized
	// and at least one control plane Machine reports a Ready Node.
	KThreesControlPlaneAvailableV1Beta2Condition = clusterv1.AvailableCondition

	// KThreesControlPlaneAvailableV1Beta2Reason surfaces when the KThreesControlPlane is available.
	KThreesControlPlaneAvailableV1Beta2Reason = clusterv1.AvailableReason

	// KThreesControlPlaneNotAvailableV1Beta2Reason surfaces when the KThreesControlPlane is not available.
	KThreesControlPlaneNotAvailableV1Beta2Reason = clusterv1.NotAvailableReason
)

// KThreesControlPlane's Initialized condition and corresponding reasons.
const (
	// KThreesControlPlaneInitializedV1Beta2Condition is true when the control plane is functional enough to accept
	// requests. This information is usually used as a signal for starting all the provisioning operations
	// that depend on a functional API server, but do not require a full HA control plane to exist.
	KThreesControlPlaneInitializedV1Beta2Condition = "Initialized"

	// KThreesControlPlaneInitializedV1Beta2Reason surfaces when the control plane is initialized.
	KThreesControlPlaneInitializedV1Beta2Reason = "Initialized"

	// KThreesControlPlaneNotInitializedV1Beta2Reason surfaces when the control plane is not initialized.
	KThreesControlPlaneNotInitializedV1Beta2Reason = "NotInitialized"
)

// KThreesControlPlane's CertificatesAvailable condition and corresponding reasons.
const (
	// KThreesControlPlaneCertificatesAvailableV1Beta2Condition is true if KThreesControlPlane certificates exist.
	KThreesControlPlaneCertificatesAvailableV1Beta2Condition = "CertificatesAvailable"

	// KThreesControlPlaneCertificatesAvailableV1Beta2Reason surfaces when the KThreesControlPlane certificates exist.
	KThreesControlPlaneCertificatesAvailableV1Beta2Reason = clusterv1.AvailableReason

	// KThreesControlPlaneCertificatesInternalErrorV1Beta2Reason surfaces unexpected failures when reconciling
	// the KThreesControlPlane certificates.
	KThreesControlPlaneCertificatesInternalErrorV1Beta2Reason = clusterv1.InternalErrorReason
)

// KThreesControlPlane's TokenAvailable condition and corresponding reasons.
const (
	// KThreesControlPlaneTokenAvailableV1Beta2Condition is true if the token required for nodes to join the cluster exists.
	KThreesControlPlaneTokenAvailableV1Beta2Condition = "TokenAvailable"

	// KThreesControlPlaneTokenAvailableV1Beta2Reason surfaces when the token required for nodes to join the cluster exists.
	KThreesControlPlaneTokenAvailableV1Beta2Reason = clusterv1.AvailableReason

	// KThreesControlPlaneTokenInternalErrorV1Beta2Reason surfaces unexpected failures when reconciling
	// the token required for nodes to join the cluster.
	KThreesControlPlaneTokenInternalErrorV1Beta2Reason = clusterv1.InternalErrorReason
)

// KThreesControlPlane's EtcdClusterHealthy condition and corresponding reasons.
const (
	// KThreesControlPlaneEtcdClusterHealthyV1Beta2Condition surfaces issues to etcd cluster hosted on machines managed by this object.
	// It is computed as aggregation of Machine's EtcdMemberHealthy conditions plus additional checks validating
	// potential issues to etcd quorum.
	KThreesControlPlaneEtcdClusterHealthyV1Beta2Condition = "EtcdClusterHealthy"

	// KThreesControlPlaneEtcdClusterInspectionFailedV1Beta2Reason documents a failure when inspecting the status of the
	// etcd cluster hosted on KThreesControlPlane controlled machines.
	KThreesControlPlaneEtcdClusterInspectionFailedV1Beta2Reason = clusterv1.InspectionFailedReason

	// KThreesControlPlaneEtcdClusterHealthyV1Beta2Reason surfaces when the etcd cluster hosted on KThreesControlPlane
	// machines is healthy.
	KThreesControlPlaneEtcdClusterHealthyV1Beta2Reason = "Healthy"

	// KThreesControlPlaneEtcdClusterNotHealthyV1Beta2Reason surfaces when the etcd cluster hosted on KThreesControlPlane
	// machines is not healthy.
	KThreesControlPlaneEtcdClusterNotHealthyV1Beta2Reason = "NotHealthy"

	// KThreesControlPlaneEtcdClusterHealthUnknownV1Beta2Reason surfaces when the health status of the etcd cluster hosted
	// on KThreesControlPlane machines is unknown.
	KThreesControlPlaneEtcdClusterHealthUnknownV1Beta2Reason = "HealthUnknown"
)

// KThreesControlPlane's ControlPlaneComponentsHealthy condition and corresponding reasons.
const (
	// KThreesControlPlaneControlPlaneComponentsHealthyV1Beta2Condition surfaces issues to control plane components
	// hosted on machines managed by this object. It is computed as aggregation of Machine's AgentHealthy conditions.
	KThreesControlPlaneControlPlaneComponentsHealthyV1Beta2Condition = "ControlPlaneComponentsHealthy"

	// KThreesControlPlaneControlPlaneComponentsInspectionFailedV1Beta2Reason documents a failure when inspecting the status of the
	// control plane components hosted on KThreesControlPlane controlled machines.
	KThreesControlPlaneControlPlaneComponentsInspectionFailedV1Beta2Reason = clusterv1.InspectionFailedReason

	// KThreesControlPlaneControlPlaneComponentsHealthyV1Beta2Reason surfaces when the control plane components hosted on
	// KThreesControlPlane machines are healthy.
	KThreesControlPlaneControlPlaneComponentsHealthyV1Beta2Reason = "Healthy"

	// KThreesControlPlaneControlPlaneComponentsNotHealthyV1Beta2Reason surfaces when the control plane components hosted on
	// KThreesControlPlane machines are not healthy.
	KThreesControlPlaneControlPlaneComponentsNotHealthyV1Beta2Reason = "NotHealthy"

	// KThreesControlPlaneControlPlaneComponentsHealthUnknownV1Beta2Reason surfaces when the health status of the control
	// plane components hosted on KThreesControlPlane machines is unknown.
	KThreesControlPlaneControlPlaneComponentsHealthUnknownV1Beta2Reason = "HealthUnknown"
)

// KThreesControlPlane's MachinesReady condition and corresponding reasons.
const (
	// KThreesControlPlaneMachinesReadyV1Beta2Condition surfaces detail of issues on the controlled machines, if any.
	KThreesControlPlaneMachinesReadyV1Beta2Condition = clusterv1.MachinesReadyCondition

	// KThreesControlPlaneMachinesReadyV1Beta2Reason surfaces when all the controlled machine's Ready conditions are true.
	KThreesControlPlaneMachinesReadyV1Beta2Reason = clusterv1.ReadyReason

	// KThreesControlPlaneMachinesNotReadyV1Beta2Reason surfaces when at least one of the controlled machine's Ready conditions is false.
	KThreesControlPlaneMachinesNotReadyV1Beta2Reason = clusterv1.NotReadyReason

	// KThreesControlPlaneMachinesReadyUnknownV1Beta2Reason surfaces when at least one of the controlled machine's Ready conditions is unknown
	// and none of the controlled machine's Ready conditions is false.
	KThreesControlPlaneMachinesReadyUnknownV1Beta2Reason = clusterv1.ReadyUnknownReason

	// KThreesControlPlaneMachinesReadyNoReplicasV1Beta2Reason surfaces when no machines exist for the KThreesControlPlane.
	KThreesControlPlaneMachinesReadyNoReplicasV1Beta2Reason = clusterv1.NoReplicasReason

	// KThreesControlPlaneMachinesReadyInternalErrorV1Beta2Reason surfaces unexpected failures when computing the MachinesReady condition.
	KThreesControlPlaneMachinesReadyInternalErrorV1Beta2Reason = clusterv1.InternalErrorReason
)

// KThreesControlPlane's MachinesUpToDate condition and corresponding reasons.
const (
	// KThreesControlPlaneMachinesUpToDateV1Beta2Condition surfaces details of controlled machines not up to date, if any.
	KThreesControlPlaneMachinesUpToDateV1Beta2Condition = clusterv1.MachinesUpToDateCondition

	// KThreesControlPlaneMachinesUpToDateV1Beta2Reason surfaces when all the controlled machine's UpToDate conditions are true.
	KThreesControlPlaneMachinesUpToDateV1Beta2Reason = clusterv1.UpToDateReason

	// KThreesControlPlaneMachinesNotUpToDateV1Beta2Reason surfaces when at least one of the controlled machine's UpToDate conditions is false.
	KThreesControlPlaneMachinesNotUpToDateV1Beta2Reason = clusterv1.NotUpToDateReason

	// KThreesControlPlaneMachinesUpToDateUnknownV1Beta2Reason surfaces when at least one of the controlled machine's UpToDate conditions is unknown
	// and none of the controlled machine's UpToDate conditions is false.
	KThreesControlPlaneMachinesUpToDateUnknownV1Beta2Reason = clusterv1.UpToDateUnknownReason

	// KThreesControlPlaneMachinesUpToDateNoReplicasV1Beta2Reason surfaces when no machines exist for the KThreesControlPlane.
	KThreesControlPlaneMachinesUpToDateNoReplicasV1Beta2Reason = clusterv1.NoReplicasReason

	// KThreesControlPlaneMachinesUpToDateInternalErrorV1Beta2Reason surfaces unexpected failures when computing the MachinesUpToDate condition.
	KThreesControlPlaneMachinesUpToDateInternalErrorV1Beta2Reason = clusterv1.InternalErrorReason
)

// KThreesControlPlane's RollingOut condition and corresponding reasons.
const (
	// KThreesControlPlaneRollingOutV1Beta2Condition is true if there is at least one machine not up-to-date.
	KThreesControlPlaneRollingOutV1Beta2Condition = clusterv1.RollingOutCondition

	// KThreesControlPlaneRollingOutV1Beta2Reason surfaces when there is at least one machine not up-to-date.
	KThreesControlPlaneRollingOutV1Beta2Reason = clusterv1.RollingOutReason

	// KThreesControlPlaneNotRollingOutV1Beta2Reason surfaces when all the machines are up-to-date.
	KThreesControlPlaneNotRollingOutV1Beta2Reason = clusterv1.NotRollingOutReason
)

// KThreesControlPlane's ScalingUp condition and corresponding reasons.
const (
	// KThreesControlPlaneScalingUpV1Beta2Condition is true if actual replicas < desired replicas.
	KThreesControlPlaneScalingUpV1Beta2Condition = clusterv1.ScalingUpCondition

	// KThreesControlPlaneScalingUpV1Beta2Reason surfaces when actual replicas < desired replicas.
	KThreesControlPlaneScalingUpV1Beta2Reason = clusterv1.ScalingUpReason

	// KThreesControlPlaneNotScalingUpV1Beta2Reason surfaces when actual replicas >= desired replicas.
	KThreesControlPlaneNotScalingUpV1Beta2Reason = clusterv1.NotScalingUpReason

	// KThreesControlPlaneScalingUpWaitingForReplicasSetV1Beta2Reason surfaces when the .spec.replicas
	// field of the KThreesControlPlane is not set.
	KThreesControlPlaneScalingUpWaitingForReplicasSetV1Beta2Reason = clusterv1.WaitingForReplicasSetReason
)

// KThreesControlPlane's ScalingDown condition and corresponding reasons.
const (
	// KThreesControlPlaneScalingDownV1Beta2Condition is true if actual replicas > desired replicas.
	KThreesControlPlaneScalingDownV1Beta2Condition = clusterv1.ScalingDownCondition

	// KThreesControlPlaneScalingDownV1Beta2Reason surfaces when actual replicas > desired replicas.
	KThreesControlPlaneScalingDownV1Beta2Reason = clusterv1.ScalingDownReason

	// KThreesControlPlaneNotScalingDownV1Beta2Reason surfaces when actual replicas <= desired replicas.
	KThreesControlPlaneNotScalingDownV1Beta2Reason = clusterv1.NotScalingDownReason

	// KThreesControlPlaneScalingDownWaitingForReplicasSetV1Beta2Reason surfaces when the .spec.replicas
	// field of the KThreesControlPlane is not set.
	KThreesControlPlaneScalingDownWaitingForReplicasSetV1Beta2Reason = clusterv1.WaitingForReplicasSetReason
)

// KThreesControlPlane's Remediating condition and corresponding reasons.
const (
	// KThreesControlPlaneRemediatingV1Beta2Condition surfaces details about ongoing remediation of the controlled machines, if any.
	KThreesControlPlaneRemediatingV1Beta2Condition = clusterv1.RemediatingCondition

	// KThreesControlPlaneRemediatingV1Beta2Reason surfaces when the KThreesControlPlane has at least one machine with HealthCheckSucceeded set to false
	// and with the OwnerRemediated condition set to false.
	KThreesControlPlaneRemediatingV1Beta2Reason = clusterv1.RemediatingReason

	// KThreesControlPlaneNotRemediatingV1Beta2Reason surfaces when the KThreesControlPlane does not have any machine with HealthCheckSucceeded set to false
	// and with the OwnerRemediated condition set to false.
	KThreesControlPlaneNotRemediatingV1Beta2Reason = clusterv1.NotRemediatingReason

	// KThreesControlPlaneRemediatingInternalErrorV1Beta2Reason surfaces unexpected failures when computing the Remediating condition.
	KThreesControlPlaneRemediatingInternalErrorV1Beta2Reason = clusterv1.InternalErrorReason
)

// KThreesControlPlane's Deleting condition and corresponding reasons.
const (
	// KThreesControlPlaneDeletingV1Beta2Condition surfaces details about ongoing deletion of the controlled machines.
	KThreesControlPlaneDeletingV1Beta2Condition = clusterv1.DeletingCondition

	// KThreesControlPlaneNotDeletingV1Beta2Reason surfaces when the KThreesControlPlane is not deleting because the
	// DeletionTimestamp is not set.
	KThreesControlPlaneNotDeletingV1Beta2Reason = clusterv1.NotDeletingReason

	// KThreesControlPlaneDeletingWaitingForWorkersDeletionV1Beta2Reason surfaces when the KThreesControlPlane deletion
	// waits for the workers to be deleted.
	KThreesControlPlaneDeletingWaitingForWorkersDeletionV1Beta2Reason = "WaitingForWorkersDeletion"

	// KThreesControlPlaneDeletingWaitingForMachineDeletionV1Beta2Reason surfaces when the KThreesControlPlane deletion
	// waits for the control plane Machines to be deleted.
	KThreesControlPlaneDeletingWaitingForMachineDeletionV1Beta2Reason = "WaitingForMachineDeletion"

	// KThreesControlPlaneDeletingDeletionCompletedV1Beta2Reason surfaces when the KThreesControlPlane deletion has been completed.
	// This reason is set right after the corresponding finalizer is removed.
	// This means that the object will go away (i.e. be removed from etcd), except if there are other
	// finalizers on the KThreesControlPlane object.
	KThreesControlPlaneDeletingDeletionCompletedV1Beta2Reason = clusterv1.DeletionCompletedReason

	// KThreesControlPlaneDeletingInternalErrorV1Beta2Reason surfaces unexpected failures when deleting a KThreesControlPlane object.
	KThreesControlPlaneDeletingInternalErrorV1Beta2Reason = clusterv1.InternalErrorReason
)
