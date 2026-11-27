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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
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

		// Note: setRollingOutCondition makes assumptions on the format/content of this message.
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

// setV1Beta2Replicas sets the replica counters in status.v1beta2 with the semantic introduced by the Cluster API v1beta2 contract:
// they are computed from the Ready, Available and UpToDate conditions of the controlled Machines.
func setV1Beta2Replicas(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
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

	if kcp.Status.V1Beta2 == nil {
		kcp.Status.V1Beta2 = &controlplanev1.KThreesControlPlaneV1Beta2Status{}
	}
	kcp.Status.V1Beta2.ReadyReplicas = ptr.To(readyReplicas)
	kcp.Status.V1Beta2.AvailableReplicas = ptr.To(availableReplicas)
	kcp.Status.V1Beta2.UpToDateReplicas = ptr.To(upToDateReplicas)
}

func setInitializedV1Beta2Condition(kcp *controlplanev1.KThreesControlPlane) {
	if kcp.Status.Initialized {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneInitializedV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: controlplanev1.KThreesControlPlaneInitializedV1Beta2Reason,
		})
		return
	}

	v1beta2conditions.Set(kcp, metav1.Condition{
		Type:   controlplanev1.KThreesControlPlaneInitializedV1Beta2Condition,
		Status: metav1.ConditionFalse,
		Reason: controlplanev1.KThreesControlPlaneNotInitializedV1Beta2Reason,
	})
}

// setAvailableV1Beta2Condition sets the Available condition.
// The control plane is considered available when it is initialized and at least one control plane Node is ready.
// NOTE: readyNodes is nil if it was not possible to read the status of the workload cluster.
func setAvailableV1Beta2Condition(kcp *controlplanev1.KThreesControlPlane, readyNodes *int32) {
	if !kcp.DeletionTimestamp.IsZero() {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1.KThreesControlPlaneNotAvailableV1Beta2Reason,
			Message: "KThreesControlPlane is deleting",
		})
		return
	}

	if !kcp.Status.Initialized {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1.KThreesControlPlaneNotAvailableV1Beta2Reason,
			Message: "Control plane not yet initialized",
		})
		return
	}

	if readyNodes == nil {
		// Keep the current condition if it was not possible to read the status of the workload cluster,
		// in order to avoid flickering in case of temporary connection issues.
		if !v1beta2conditions.Has(kcp, controlplanev1.KThreesControlPlaneAvailableV1Beta2Condition) {
			v1beta2conditions.Set(kcp, metav1.Condition{
				Type:    controlplanev1.KThreesControlPlaneAvailableV1Beta2Condition,
				Status:  metav1.ConditionUnknown,
				Reason:  clusterv1.AvailableUnknownReason,
				Message: "Failed to read the status of the workload cluster",
			})
		}
		return
	}

	if *readyNodes == 0 {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneAvailableV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1.KThreesControlPlaneNotAvailableV1Beta2Reason,
			Message: "There are no ready control plane Nodes",
		})
		return
	}

	v1beta2conditions.Set(kcp, metav1.Condition{
		Type:   controlplanev1.KThreesControlPlaneAvailableV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: controlplanev1.KThreesControlPlaneAvailableV1Beta2Reason,
	})
}

func setRollingOutV1Beta2Condition(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	// Count machines rolling out and collect reasons why a rollout is happening.
	// Note: The code below collects all the reasons for which at least a machine is rolling out; under normal circumstances
	// all the machines are rolling out for the same reasons, however, in case of changes to KThreesControlPlane
	// before a previous changes is not fully rolled out, there could be machines rolling out for
	// different reasons.
	rollingOutReplicas := 0
	rolloutReasons := sets.Set[string]{}
	for _, machine := range machines {
		upToDateCondition := conditions.Get(machine, clusterv1.MachineUpToDateCondition)
		if upToDateCondition == nil || upToDateCondition.Status != metav1.ConditionFalse {
			continue
		}
		rollingOutReplicas++
		if upToDateCondition.Message != "" {
			rolloutReasons.Insert(strings.Split(upToDateCondition.Message, "\n")...)
		}
	}

	if rollingOutReplicas == 0 {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneRollingOutV1Beta2Condition,
			Status: metav1.ConditionFalse,
			Reason: controlplanev1.KThreesControlPlaneNotRollingOutV1Beta2Reason,
		})
		return
	}

	// Rolling out.
	message := fmt.Sprintf("Rolling out %d not up-to-date replicas", rollingOutReplicas)
	if rolloutReasons.Len() > 0 {
		// Surface rollout reasons ensuring that if there is a version change, it goes first.
		reasons := rolloutReasons.UnsortedList()
		sort.Slice(reasons, func(i, j int) bool {
			if strings.HasPrefix(reasons[i], "* Version") && !strings.HasPrefix(reasons[j], "* Version") {
				return true
			}
			if !strings.HasPrefix(reasons[i], "* Version") && strings.HasPrefix(reasons[j], "* Version") {
				return false
			}
			return reasons[i] < reasons[j]
		})
		message += fmt.Sprintf("\n%s", strings.Join(reasons, "\n"))
	}
	v1beta2conditions.Set(kcp, metav1.Condition{
		Type:    controlplanev1.KThreesControlPlaneRollingOutV1Beta2Condition,
		Status:  metav1.ConditionTrue,
		Reason:  controlplanev1.KThreesControlPlaneRollingOutV1Beta2Reason,
		Message: message,
	})
}

func setScalingUpV1Beta2Condition(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	if kcp.Spec.Replicas == nil {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneScalingUpV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneScalingUpWaitingForReplicasSetV1Beta2Reason,
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
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneScalingUpV1Beta2Condition,
			Status: metav1.ConditionFalse,
			Reason: controlplanev1.KThreesControlPlaneNotScalingUpV1Beta2Reason,
		})
		return
	}

	v1beta2conditions.Set(kcp, metav1.Condition{
		Type:    controlplanev1.KThreesControlPlaneScalingUpV1Beta2Condition,
		Status:  metav1.ConditionTrue,
		Reason:  controlplanev1.KThreesControlPlaneScalingUpV1Beta2Reason,
		Message: fmt.Sprintf("Scaling up from %d to %d replicas", currentReplicas, desiredReplicas),
	})
}

func setScalingDownV1Beta2Condition(kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	if kcp.Spec.Replicas == nil {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneScalingDownV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneScalingDownWaitingForReplicasSetV1Beta2Reason,
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
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneScalingDownV1Beta2Condition,
			Status: metav1.ConditionFalse,
			Reason: controlplanev1.KThreesControlPlaneNotScalingDownV1Beta2Reason,
		})
		return
	}

	message := fmt.Sprintf("Scaling down from %d to %d replicas", currentReplicas, desiredReplicas)
	if staleMessage := aggregateStaleMachines(machines); staleMessage != "" {
		message += fmt.Sprintf(" is blocked because:\n* %s", staleMessage)
	}
	v1beta2conditions.Set(kcp, metav1.Condition{
		Type:    controlplanev1.KThreesControlPlaneScalingDownV1Beta2Condition,
		Status:  metav1.ConditionTrue,
		Reason:  controlplanev1.KThreesControlPlaneScalingDownV1Beta2Reason,
		Message: message,
	})
}

func setMachinesReadyV1Beta2Condition(ctx context.Context, kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	if len(machines) == 0 {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneMachinesReadyV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: controlplanev1.KThreesControlPlaneMachinesReadyNoReplicasV1Beta2Reason,
		})
		return
	}

	readyCondition, err := conditions.NewAggregateCondition(
		machines.UnsortedList(), clusterv1.MachineReadyCondition,
		conditions.TargetConditionType(controlplanev1.KThreesControlPlaneMachinesReadyV1Beta2Condition),
		// Using a custom merge strategy to override reasons applied during merge.
		conditions.CustomMergeStrategy{
			MergeStrategy: conditions.DefaultMergeStrategy(
				conditions.ComputeReasonFunc(conditions.GetDefaultComputeMergeReasonFunc(
					controlplanev1.KThreesControlPlaneMachinesNotReadyV1Beta2Reason,
					controlplanev1.KThreesControlPlaneMachinesReadyUnknownV1Beta2Reason,
					controlplanev1.KThreesControlPlaneMachinesReadyV1Beta2Reason,
				)),
			),
		},
	)
	if err != nil {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneMachinesReadyV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneMachinesReadyInternalErrorV1Beta2Reason,
			Message: "Please check controller logs for errors",
		})

		ctrl.LoggerFrom(ctx).Error(err, fmt.Sprintf("Failed to aggregate Machine's %s conditions", clusterv1.MachineReadyCondition))
		return
	}

	v1beta2conditions.Set(kcp, *readyCondition)
}

func setMachinesUpToDateV1Beta2Condition(ctx context.Context, kcp *controlplanev1.KThreesControlPlane, machines collections.Machines) {
	// Only consider Machines that have an UpToDate condition or are older than 10s.
	// This is done to ensure the MachinesUpToDate condition doesn't flicker after a new Machine is created,
	// because it can take a bit until the UpToDate condition is set on a new Machine.
	machines = machines.Filter(func(machine *clusterv1.Machine) bool {
		return conditions.Has(machine, clusterv1.MachineUpToDateCondition) || time.Since(machine.CreationTimestamp.Time) > 10*time.Second
	})

	if len(machines) == 0 {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:   controlplanev1.KThreesControlPlaneMachinesUpToDateV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: controlplanev1.KThreesControlPlaneMachinesUpToDateNoReplicasV1Beta2Reason,
		})
		return
	}

	upToDateCondition, err := conditions.NewAggregateCondition(
		machines.UnsortedList(), clusterv1.MachineUpToDateCondition,
		conditions.TargetConditionType(controlplanev1.KThreesControlPlaneMachinesUpToDateV1Beta2Condition),
		// Using a custom merge strategy to override reasons applied during merge.
		conditions.CustomMergeStrategy{
			MergeStrategy: conditions.DefaultMergeStrategy(
				conditions.ComputeReasonFunc(conditions.GetDefaultComputeMergeReasonFunc(
					controlplanev1.KThreesControlPlaneMachinesNotUpToDateV1Beta2Reason,
					controlplanev1.KThreesControlPlaneMachinesUpToDateUnknownV1Beta2Reason,
					controlplanev1.KThreesControlPlaneMachinesUpToDateV1Beta2Reason,
				)),
			),
		},
	)
	if err != nil {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneMachinesUpToDateV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneMachinesUpToDateInternalErrorV1Beta2Reason,
			Message: "Please check controller logs for errors",
		})

		ctrl.LoggerFrom(ctx).Error(err, fmt.Sprintf("Failed to aggregate Machine's %s conditions", clusterv1.MachineUpToDateCondition))
		return
	}

	v1beta2conditions.Set(kcp, *upToDateCondition)
}

func setRemediatingV1Beta2Condition(ctx context.Context, kcp *controlplanev1.KThreesControlPlane, machinesToBeRemediated, unhealthyMachines collections.Machines) {
	if len(machinesToBeRemediated) == 0 {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneRemediatingV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1.KThreesControlPlaneNotRemediatingV1Beta2Reason,
			Message: aggregateUnhealthyMachines(unhealthyMachines),
		})
		return
	}

	remediatingCondition, err := conditions.NewAggregateCondition(
		machinesToBeRemediated.UnsortedList(), clusterv1.MachineOwnerRemediatedCondition,
		conditions.TargetConditionType(controlplanev1.KThreesControlPlaneRemediatingV1Beta2Condition),
		// Note: in case of the remediating conditions it is not required to use a CustomMergeStrategy/ComputeReasonFunc
		// because we are considering only machinesToBeRemediated (and we can pin the reason when we set the condition).
	)
	if err != nil {
		v1beta2conditions.Set(kcp, metav1.Condition{
			Type:    controlplanev1.KThreesControlPlaneRemediatingV1Beta2Condition,
			Status:  metav1.ConditionUnknown,
			Reason:  controlplanev1.KThreesControlPlaneRemediatingInternalErrorV1Beta2Reason,
			Message: "Please check controller logs for errors",
		})

		ctrl.LoggerFrom(ctx).Error(err, fmt.Sprintf("Failed to aggregate Machine's %s conditions", clusterv1.MachineOwnerRemediatedCondition))
		return
	}

	v1beta2conditions.Set(kcp, metav1.Condition{
		Type:    remediatingCondition.Type,
		Status:  metav1.ConditionTrue,
		Reason:  controlplanev1.KThreesControlPlaneRemediatingV1Beta2Reason,
		Message: remediatingCondition.Message,
	})
}

// setDeletingV1Beta2Condition sets the Deleting condition to false if the KThreesControlPlane is not being deleted.
// NOTE: While the KThreesControlPlane is being deleted, the Deleting condition is set by reconcileDelete.
func setDeletingV1Beta2Condition(kcp *controlplanev1.KThreesControlPlane) {
	if !kcp.DeletionTimestamp.IsZero() {
		return
	}
	v1beta2conditions.Set(kcp, metav1.Condition{
		Type:   controlplanev1.KThreesControlPlaneDeletingV1Beta2Condition,
		Status: metav1.ConditionFalse,
		Reason: controlplanev1.KThreesControlPlaneNotDeletingV1Beta2Reason,
	})
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

// aggregateUnhealthyMachines returns a message listing the machines that are unhealthy but not remediated by KThreesControlPlane.
func aggregateUnhealthyMachines(machines collections.Machines) string {
	if len(machines) == 0 {
		return ""
	}

	names := machines.Names()
	sort.Strings(names)
	if len(names) > 3 {
		names = append(names[:3], "...")
	}
	if len(machines) == 1 {
		return fmt.Sprintf("Machine %s is not healthy (not to be remediated by KThreesControlPlane)", strings.Join(names, ", "))
	}
	return fmt.Sprintf("Machines %s are not healthy (not to be remediated by KThreesControlPlane)", strings.Join(names, ", "))
}
