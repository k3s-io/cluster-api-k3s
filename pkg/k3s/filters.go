/*
Copyright 2026 The Kubernetes Authors.

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

package k3s

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/capi/compare"
	"github.com/k3s-io/cluster-api-k3s/pkg/capi/inplace"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s/desiredstate"
)

// UpToDateResult is the result of evaluating a Machine against the desired control-plane state.
type UpToDateResult struct {
	LogMessages              []string
	ConditionMessages        []string
	EligibleForInPlaceUpdate bool
	DesiredMachine           *clusterv1.Machine
	CurrentInfraMachine      *unstructured.Unstructured
	DesiredInfraMachine      *unstructured.Unstructured
	CurrentKThreesConfig     *bootstrapv1.KThreesConfig
	DesiredKThreesConfig     *bootstrapv1.KThreesConfig
}

// UpToDate determines whether a Machine and its related objects match the desired control-plane state.
func UpToDate(
	ctx context.Context,
	c client.Client,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine,
	kcp *controlplanev1.KThreesControlPlane,
	reconciliationTime *metav1.Time,
	infraMachines map[string]*unstructured.Unstructured,
	kthreesConfigs map[string]*bootstrapv1.KThreesConfig,
) (bool, *UpToDateResult, error) {
	result := &UpToDateResult{EligibleForInPlaceUpdate: true}

	if _, ok := machine.Annotations[clusterv1.DeleteMachineAnnotation]; ok {
		result.EligibleForInPlaceUpdate = false
	}
	if _, ok := machine.Annotations[clusterv1.RemediateMachineAnnotation]; ok {
		result.EligibleForInPlaceUpdate = false
	}
	if kcp.Spec.RolloutAfter != nil &&
		collections.ShouldRolloutAfter(reconciliationTime, *kcp.Spec.RolloutAfter)(machine) {
		result.LogMessages = append(result.LogMessages, "rolloutAfter expired")
		result.ConditionMessages = append(result.ConditionMessages, "KThreesControlPlane spec.rolloutAfter expired")
		result.EligibleForInPlaceUpdate = false
	}

	desiredMachine, err := desiredstate.ComputeDesiredMachine(
		kcp, cluster, machine.Spec.InfrastructureRef, machine.Spec.Bootstrap.ConfigRef, machine.Spec.FailureDomain, machine,
	)
	if err != nil {
		return false, nil, errors.Wrapf(err, "failed to determine if Machine %s is up-to-date", machine.Name)
	}
	desiredMachine.Spec.Version = kcp.Spec.Version
	result.DesiredMachine = desiredMachine

	machineMatches, _, err := compare.Diff(
		inplace.CleanupMachineSpecForDiff(&machine.Spec),
		inplace.CleanupMachineSpecForDiff(&desiredMachine.Spec),
	)
	if err != nil {
		return false, nil, errors.Wrapf(err, "failed to compare Machine %s", machine.Name)
	}
	if !machineMatches {
		if machine.Spec.Version != desiredMachine.Spec.Version {
			result.LogMessages = append(result.LogMessages,
				fmt.Sprintf("Machine version %q is not equal to KCP version %q", machine.Spec.Version, desiredMachine.Spec.Version))
			result.ConditionMessages = append(result.ConditionMessages,
				fmt.Sprintf("Version %s, %s required", machine.Spec.Version, desiredMachine.Spec.Version))
		} else {
			result.LogMessages = append(result.LogMessages, "Machine spec is not up-to-date")
			result.ConditionMessages = append(result.ConditionMessages, "Machine spec is not up-to-date")
		}
	}

	currentConfig, configFound := kthreesConfigs[machine.Name]
	if !configFound {
		result.EligibleForInPlaceUpdate = false
	} else {
		result.CurrentKThreesConfig = currentConfig
		desiredConfig, err := desiredstate.ComputeDesiredKThreesConfig(kcp, cluster, currentConfig.Name, currentConfig)
		if err != nil {
			return false, nil, errors.Wrapf(err, "failed to compute desired KThreesConfig for Machine %s", machine.Name)
		}
		result.DesiredKThreesConfig = desiredConfig
		currentConfigForDiff, desiredConfigForDiff := prepareKThreesConfigsForDiff(currentConfig, desiredConfig)
		configMatches, _, err := compare.Diff(&currentConfigForDiff.Spec, &desiredConfigForDiff.Spec)
		if err != nil {
			return false, nil, errors.Wrapf(err, "failed to compare KThreesConfig for Machine %s", machine.Name)
		}
		if !configMatches {
			result.LogMessages = append(result.LogMessages, "KThreesConfig spec is not up-to-date")
			result.ConditionMessages = append(result.ConditionMessages, "KThreesConfig is not up-to-date")
		}
	}

	currentInfra, infraFound := infraMachines[machine.Name]
	if !infraFound {
		result.EligibleForInPlaceUpdate = false
	} else {
		result.CurrentInfraMachine = currentInfra
		desiredInfra, err := desiredstate.ComputeDesiredInfraMachine(ctx, c, kcp, cluster, currentInfra.GetName(), currentInfra)
		if err != nil {
			if !kcp.DeletionTimestamp.IsZero() && apierrors.IsNotFound(err) {
				result.EligibleForInPlaceUpdate = false
			} else {
				return false, nil, errors.Wrapf(err, "failed to compute desired InfraMachine for Machine %s", machine.Name)
			}
		} else {
			result.DesiredInfraMachine = desiredInfra
			annotations := currentInfra.GetAnnotations()
			clonedFromName, hasName := annotations[clusterv1.TemplateClonedFromNameAnnotation]
			clonedFromGroupKind, hasGroupKind := annotations[clusterv1.TemplateClonedFromGroupKindAnnotation]
			desiredTemplateRef := kcp.Spec.MachineTemplate.InfrastructureRef
			if hasName && hasGroupKind &&
				(clonedFromName != desiredTemplateRef.Name ||
					clonedFromGroupKind != desiredTemplateRef.GroupVersionKind().GroupKind().String()) {
				result.LogMessages = append(result.LogMessages, fmt.Sprintf(
					"Infrastructure template rotated from %s %s to %s %s",
					clonedFromGroupKind, clonedFromName,
					desiredTemplateRef.GroupVersionKind().GroupKind().String(), desiredTemplateRef.Name,
				))
				result.ConditionMessages = append(result.ConditionMessages, fmt.Sprintf("%s is not up-to-date", machine.Spec.InfrastructureRef.Kind))
			}
		}
	}

	if len(result.LogMessages) > 0 || len(result.ConditionMessages) > 0 {
		return false, result, nil
	}
	result.EligibleForInPlaceUpdate = false
	return true, result, nil
}

// PrepareKThreesConfigsForDiff normalizes bootstrap version because Machine.spec.version is authoritative.
func PrepareKThreesConfigsForDiff(
	current, desired *bootstrapv1.KThreesConfig,
) (*bootstrapv1.KThreesConfig, *bootstrapv1.KThreesConfig) {
	current = current.DeepCopy()
	desired = desired.DeepCopy()
	current.Spec.Version = ""
	desired.Spec.Version = ""
	return current, desired
}

func prepareKThreesConfigsForDiff(
	current, desired *bootstrapv1.KThreesConfig,
) (*bootstrapv1.KThreesConfig, *bootstrapv1.KThreesConfig) {
	return PrepareKThreesConfigsForDiff(current, desired)
}
