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
package controllers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta3"
	k3s "github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

// reconcileMachineUpToDateCondition sets the UpToDate condition on control plane Machines.
// NOTE: Core Cluster API computes the UpToDate condition only for worker Machines; for control plane Machines,
// it is the responsibility of the control plane provider.
func reconcileMachineUpToDateCondition(controlPlane *k3s.ControlPlane) {
	for _, machine := range controlPlane.Machines {
		reasons := controlPlane.MachineNotUpToDateReasons(machine)
		if len(reasons) == 0 {
			conditions.Set(machine, metav1.Condition{
				Type:   clusterv1.MachineUpToDateCondition,
				Status: metav1.ConditionTrue,
				Reason: clusterv1.MachineUpToDateReason,
			})
			continue
		}

		messages := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			messages = append(messages, fmt.Sprintf("* %s", reason))
		}
		conditions.Set(machine, metav1.Condition{
			Type:    clusterv1.MachineUpToDateCondition,
			Status:  metav1.ConditionFalse,
			Reason:  clusterv1.MachineNotUpToDateReason,
			Message: strings.Join(messages, "\n"),
		})
	}
}

// setReplicas sets the replica counters with the semantic introduced by the Cluster API v1beta2 contract:
// they are computed from the Ready, Available and UpToDate conditions of the controlled Machines.
func setReplicas(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	var readyReplicas, availableReplicas, upToDateReplicas int32
	for _, machine := range machines {
		if conditions.IsTrue(machine, clusterv1.MachineReadyCondition) {
			readyReplicas++
		}
		if conditions.IsTrue(machine, clusterv1.MachineAvailableCondition) {
			availableReplicas++
		}
		if conditions.IsTrue(machine, clusterv1.MachineUpToDateCondition) {
			upToDateReplicas++
		}
	}

	kcp.Status.Replicas = ptr.To(int32(len(machines)))
	kcp.Status.ReadyReplicas = ptr.To(readyReplicas)
	kcp.Status.AvailableReplicas = ptr.To(availableReplicas)
	kcp.Status.UpToDateReplicas = ptr.To(upToDateReplicas)
}

// v1beta1DeprecatedStatus returns the struct holding the deprecated v1beta1 status fields, creating it if necessary.
func v1beta1DeprecatedStatus(kcp *controlplanev1.KThreesControlPlane) *controlplanev1.KThreesControlPlaneV1Beta1DeprecatedStatus {
	if kcp.Status.Deprecated == nil {
		kcp.Status.Deprecated = &controlplanev1.KThreesControlPlaneDeprecatedStatus{}
	}
	if kcp.Status.Deprecated.V1Beta1 == nil {
		kcp.Status.Deprecated.V1Beta1 = &controlplanev1.KThreesControlPlaneV1Beta1DeprecatedStatus{}
	}
	return kcp.Status.Deprecated.V1Beta1
}

func setInitializedCondition(kcp *controlplanev1.KThreesControlPlane) {
	if ptr.Deref(kcp.Status.Initialization.ControlPlaneInitialized, false) {
		conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneInitializedCondition,
			Status: metav1.ConditionTrue,
			Reason: controlplanev1.KThreesControlPlaneInitializedReason,
		})
		return
	}

	conditions.Set(kcp, metav1.Condition{
		Type:   controlplanev1.KThreesControlPlaneInitializedCondition,
		Status: metav1.ConditionFalse,
		Reason: controlplanev1.KThreesControlPlaneNotInitializedReason,
	})
}

// setAvailableCondition sets the Available condition.
// The control plane is considered available when it is initialized and at least one control plane Node is ready.
// NOTE: readyNodes is nil if it was not possible to read the status of the workload cluster.
func setAvailableCondition(kcp *controlplanev1.KThreesControlPlane, readyNodes *int32) {
	if !kcp.DeletionTimestamp.IsZero() {
		conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1.KThreesControlPlaneNotAvailableReason,
			Message: "KThreesControlPlane is deleting",
		})
		return
	}

	if !ptr.Deref(kcp.Status.Initialization.ControlPlaneInitialized, false) {
		conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1.KThreesControlPlaneNotAvailableReason,
			Message: "Control plane not yet initialized",
		})
		return
	}

	if readyNodes == nil {
		// Keep the current condition if it was not possible to read the status of the workload cluster,
		// in order to avoid flickering in case of temporary connection issues.
		if !conditions.Has(kcp, controlplanev1.KThreesControlPlaneAvailableCondition) {
			conditions.Set(kcp, metav1.Condition{
				Type:    controlplanev1.KThreesControlPlaneAvailableCondition,
				Status:  metav1.ConditionUnknown,
				Reason:  clusterv1.AvailableUnknownReason,
				Message: "Failed to read the status of the workload cluster",
			})
		}
		return
	}

	if *readyNodes == 0 {
		conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneAvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1.KThreesControlPlaneNotAvailableReason,
			Message: "There are no ready control plane Nodes",
		})
		return
	}

	conditions.Set(kcp, metav1.Condition{
		Type:   controlplanev1.KThreesControlPlaneAvailableCondition,
		Status: metav1.ConditionTrue,
		Reason: controlplanev1.KThreesControlPlaneAvailableReason,
	})
}

func setScalingUpCondition(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	if kcp.Spec.Replicas == nil {
		conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneScalingUpCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneScalingUpWaitingForReplicasSetReason,
			Message: "Waiting for spec.replicas set",
		})
		return
	}

	currentReplicas := int32(len(machines))
	desiredReplicas := *kcp.Spec.Replicas
	if !kcp.DeletionTimestamp.IsZero() {
		desiredReplicas = 0
	}

	if currentReplicas >= desiredReplicas {
		conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneScalingUpCondition,
			Status: metav1.ConditionFalse,
			Reason: controlplanev1.KThreesControlPlaneNotScalingUpReason,
		})
		return
	}

	conditions.Set(kcp, metav1.Condition{
		Type:    controlplanev1.KThreesControlPlaneScalingUpCondition,
		Status:  metav1.ConditionTrue,
		Reason:  controlplanev1.KThreesControlPlaneScalingUpReason,
		Message: fmt.Sprintf("Scaling up from %d to %d replicas", currentReplicas, desiredReplicas),
	})
}

func setScalingDownCondition(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	if kcp.Spec.Replicas == nil {
		conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneScalingDownCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneScalingDownWaitingForReplicasSetReason,
			Message: "Waiting for spec.replicas set",
		})
		return
	}

	currentReplicas := int32(len(machines))
	desiredReplicas := *kcp.Spec.Replicas
	if !kcp.DeletionTimestamp.IsZero() {
		desiredReplicas = 0
	}

	if currentReplicas <= desiredReplicas {
		conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneScalingDownCondition,
			Status: metav1.ConditionFalse,
			Reason: controlplanev1.KThreesControlPlaneNotScalingDownReason,
		})
		return
	}

	message := fmt.Sprintf("Scaling down from %d to %d replicas", currentReplicas, desiredReplicas)
	if staleMessage := aggregateStaleMachines(machines); staleMessage != "" {
		message += fmt.Sprintf(" is blocked because:\n* %s", staleMessage)
	}
	conditions.Set(kcp, metav1.Condition{
		Type:    controlplanev1.KThreesControlPlaneScalingDownCondition,
		Status:  metav1.ConditionTrue,
		Reason:  controlplanev1.KThreesControlPlaneScalingDownReason,
		Message: message,
	})
}

func setMachinesReadyCondition(ctx context.Context, kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	if len(machines) == 0 {
		conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneMachinesReadyCondition,
			Status: metav1.ConditionTrue,
			Reason: controlplanev1.KThreesControlPlaneMachinesReadyNoReplicasReason,
		})
		return
	}

	readyCondition, err := conditions.NewAggregateCondition(
		machines.UnsortedList(), clusterv1.MachineReadyCondition,
		conditions.TargetConditionType(controlplanev1.KThreesControlPlaneMachinesReadyCondition),
		// Using a custom merge strategy to override reasons applied during merge.
		conditions.CustomMergeStrategy{
			MergeStrategy: conditions.DefaultMergeStrategy(
				conditions.ComputeReasonFunc(conditions.GetDefaultComputeMergeReasonFunc(
					controlplanev1.KThreesControlPlaneMachinesNotReadyReason,
					controlplanev1.KThreesControlPlaneMachinesReadyUnknownReason,
					controlplanev1.KThreesControlPlaneMachinesReadyReason,
				)),
			),
		},
	)
	if err != nil {
		conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneMachinesReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneMachinesReadyInternalErrorReason,
			Message: "Please check controller logs for errors",
		})

		ctrl.LoggerFrom(ctx).Error(err, fmt.Sprintf("Failed to aggregate Machine's %s conditions", clusterv1.MachineReadyCondition))
		return
	}

	conditions.Set(kcp, *readyCondition)
}

// setDeletingCondition sets the Deleting condition to false if the KThreesControlPlane is not being deleted.
// NOTE: While the KThreesControlPlane is being deleted, the Deleting condition is set by reconcileDelete.
func setDeletingCondition(kcp *controlplanev1.KThreesControlPlane) {
	if !kcp.DeletionTimestamp.IsZero() {
		return
	}
	conditions.Set(kcp, metav1.Condition{
		Type:   controlplanev1.KThreesControlPlaneDeletingCondition,
		Status: metav1.ConditionFalse,
		Reason: controlplanev1.KThreesControlPlaneNotDeletingReason,
	})
}

// setV1Beta1ResizedCondition sets the deprecated v1beta1 Resized condition.
func setV1Beta1ResizedCondition(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	replicas := int32(len(machines))
	desiredReplicas := ptr.Deref(kcp.Spec.Replicas, 1)

	switch {
	// We are scaling up
	case replicas < desiredReplicas:
		v1beta1conditions.MarkFalse(kcp, controlplanev1.ResizedV1Beta1Condition, controlplanev1.ScalingUpV1Beta1Reason, clusterv1.ConditionSeverityWarning, "Scaling up control plane to %d replicas (actual %d)", desiredReplicas, replicas)
	// We are scaling down
	case replicas > desiredReplicas:
		v1beta1conditions.MarkFalse(kcp, controlplanev1.ResizedV1Beta1Condition, controlplanev1.ScalingDownV1Beta1Reason, clusterv1.ConditionSeverityWarning, "Scaling down control plane to %d replicas (actual %d)", desiredReplicas, replicas)
	default:
		// make sure last resize operation is marked as completed.
		// NOTE: we are checking the number of machines ready so we report resize completed only when the machines
		// are actually provisioned (vs reporting completed immediately after the last machine object is created).
		readyMachines := machines.Filter(collections.IsReady())
		if int32(len(readyMachines)) == replicas {
			v1beta1conditions.MarkTrue(kcp, controlplanev1.ResizedV1Beta1Condition)
		}
	}
}

// setLastRemediation surfaces lastRemediation data in status.
// LastRemediation is the remediation currently in progress, in any, or the
// most recent of the remediation we are keeping track on machines.
func setLastRemediation(controlPlane *k3s.ControlPlane) error {
	var lastRemediation *RemediationData

	if v, ok := controlPlane.KCP.Annotations[controlplanev1.RemediationInProgressAnnotation]; ok {
		remediationData, err := RemediationDataFromAnnotation(v)
		if err != nil {
			return err
		}
		lastRemediation = remediationData
	} else {
		for _, m := range controlPlane.Machines.UnsortedList() {
			if v, ok := m.Annotations[controlplanev1.RemediationForAnnotation]; ok {
				remediationData, err := RemediationDataFromAnnotation(v)
				if err != nil {
					return err
				}
				if lastRemediation == nil || lastRemediation.Timestamp.Time.Before(remediationData.Timestamp.Time) {
					lastRemediation = remediationData
				}
			}
		}
	}

	if lastRemediation != nil {
		controlPlane.KCP.Status.LastRemediation = lastRemediation.ToStatus()
	}
	return nil
}

// aggregateStaleMachines returns a message listing the machines that are deleting since more than 15 minutes.
func aggregateStaleMachines(machines collections.Machines) string {
	staleMachines := machines.Filter(func(machine *clusterv1.Machine) bool {
		return !machine.DeletionTimestamp.IsZero() && time.Since(machine.DeletionTimestamp.Time) > time.Minute*15
	})
	if len(staleMachines) == 0 {
		return ""
	}

	names := staleMachines.Names()
	sort.Strings(names)
	if len(names) > 3 {
		names = append(names[:3], "...")
	}
	if len(staleMachines) == 1 {
		return fmt.Sprintf("Machine %s is in deletion since more than 15m", strings.Join(names, ", "))
	}
	return fmt.Sprintf("Machines %s are in deletion since more than 15m", strings.Join(names, ", "))
}
