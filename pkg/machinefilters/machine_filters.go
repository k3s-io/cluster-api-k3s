/*
Copyright 2020 The Kubernetes Authors.

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

package machinefilters

import (
	bootstrapv1 "github.com/zawachte-msft/cluster-api-k3s/bootstrap/api/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	controlplanev1 "github.com/zawachte-msft/cluster-api-k3s/controlplane/api/v1alpha3"

	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Func func(machine *clusterv1.Machine) bool

// And returns a filter that returns true if all of the given filters returns true.
func And(filters ...Func) Func {
	return func(machine *clusterv1.Machine) bool {
		for _, f := range filters {
			if !f(machine) {
				return false
			}
		}
		return true
	}
}

// Or returns a filter that returns true if any of the given filters returns true.
func Or(filters ...Func) Func {
	return func(machine *clusterv1.Machine) bool {
		for _, f := range filters {
			if f(machine) {
				return true
			}
		}
		return false
	}
}

// Not returns a filter that returns the opposite of the given filter.
func Not(mf Func) Func {
	return func(machine *clusterv1.Machine) bool {
		return !mf(machine)
	}
}

// HasControllerRef is a filter that returns true if the machine has a controller ref
func HasControllerRef(machine *clusterv1.Machine) bool {
	if machine == nil {
		return false
	}
	return metav1.GetControllerOf(machine) != nil
}

// InFailureDomains returns a filter to find all machines
// in any of the given failure domains
func InFailureDomains(failureDomains ...*string) Func {
	return func(machine *clusterv1.Machine) bool {
		if machine == nil {
			return false
		}
		for i := range failureDomains {
			fd := failureDomains[i]
			if fd == nil {
				if fd == machine.Spec.FailureDomain {
					return true
				}
				continue
			}
			if machine.Spec.FailureDomain == nil {
				continue
			}
			if *fd == *machine.Spec.FailureDomain {
				return true
			}
		}
		return false
	}
}

// OwnedMachines returns a filter to find all owned control plane machines.
// Usage: managementCluster.GetMachinesForCluster(ctx, cluster, machinefilters.OwnedMachines(controlPlane))
func OwnedMachines(owner controllerutil.Object) func(machine *clusterv1.Machine) bool {
	return func(machine *clusterv1.Machine) bool {
		if machine == nil {
			return false
		}
		return util.IsOwnedByObject(machine, owner)
	}
}

// ControlPlaneMachines returns a filter to find all control plane machines for a cluster, regardless of ownership.
// Usage: managementCluster.GetMachinesForCluster(ctx, cluster, machinefilters.ControlPlaneMachines(cluster.Name))
func ControlPlaneMachines(clusterName string) func(machine *clusterv1.Machine) bool {
	selector := ControlPlaneSelectorForCluster(clusterName)
	return func(machine *clusterv1.Machine) bool {
		if machine == nil {
			return false
		}
		return selector.Matches(labels.Set(machine.Labels))
	}
}

// AdoptableControlPlaneMachines returns a filter to find all un-controlled control plane machines.
// Usage: managementCluster.GetMachinesForCluster(ctx, cluster, AdoptableControlPlaneMachines(cluster.Name, controlPlane))
func AdoptableControlPlaneMachines(clusterName string) func(machine *clusterv1.Machine) bool {
	return And(
		ControlPlaneMachines(clusterName),
		Not(HasControllerRef),
	)
}

// HasDeletionTimestamp returns a filter to find all machines that have a deletion timestamp.
func HasDeletionTimestamp(machine *clusterv1.Machine) bool {
	if machine == nil {
		return false
	}
	return !machine.DeletionTimestamp.IsZero()
}

// HasUnhealthyCondition returns a filter to find all machines that have a MachineHealthCheckSucceeded condition set to False,
// indicating a problem was detected on the machine, and the MachineOwnerRemediated condition set, indicating that KCP is
// responsible of performing remediation as owner of the machine.
func HasUnhealthyCondition(machine *clusterv1.Machine) bool {
	if machine == nil {
		return false
	}
	return conditions.IsFalse(machine, clusterv1.MachineHealthCheckSuccededCondition) && conditions.IsFalse(machine, clusterv1.MachineOwnerRemediatedCondition)
}

// IsReady returns a filter to find all machines with the ReadyCondition equals to True.
func IsReady() Func {
	return func(machine *clusterv1.Machine) bool {
		if machine == nil {
			return false
		}
		return conditions.IsTrue(machine, clusterv1.ReadyCondition)
	}
}

// ShouldRolloutAfter returns a filter to find all machines where
// CreationTimestamp < rolloutAfter < reconciliationTIme
func ShouldRolloutAfter(reconciliationTime, rolloutAfter *metav1.Time) Func {
	return func(machine *clusterv1.Machine) bool {
		if machine == nil {
			return false
		}
		return machine.CreationTimestamp.Before(rolloutAfter) && rolloutAfter.Before(reconciliationTime)
	}
}

// HasAnnotationKey returns a filter to find all machines that have the
// specified Annotation key present
func HasAnnotationKey(key string) Func {
	return func(machine *clusterv1.Machine) bool {
		if machine == nil || machine.Annotations == nil {
			return false
		}
		if _, ok := machine.Annotations[key]; ok {
			return true
		}
		return false
	}
}

// ControlPlaneSelectorForCluster returns the label selector necessary to get control plane machines for a given cluster.
func ControlPlaneSelectorForCluster(clusterName string) labels.Selector {
	must := func(r *labels.Requirement, err error) labels.Requirement {
		if err != nil {
			panic(err)
		}
		return *r
	}
	return labels.NewSelector().Add(
		must(labels.NewRequirement(clusterv1.ClusterLabelName, selection.Equals, []string{clusterName})),
		must(labels.NewRequirement(clusterv1.MachineControlPlaneLabelName, selection.Exists, []string{})),
	)
}

// MatchesKCPConfiguration returns a filter to find all machines that matches with KCP config and do not require any rollout.
// Kubernetes version, infrastructure template, and KThreesConfig field need to be equivalent.
func MatchesKCPConfiguration(infraConfigs map[string]*unstructured.Unstructured, machineConfigs map[string]*bootstrapv1.KThreesConfig, kcp *controlplanev1.KThreesControlPlane) func(machine *clusterv1.Machine) bool {
	return And(
		MatchesKubernetesVersion(kcp.Spec.Version),
		MatchesKThreesBootstrapConfig(machineConfigs, kcp),
		MatchesTemplateClonedFrom(infraConfigs, kcp),
	)
}

// MatchesTemplateClonedFrom returns a filter to find all machines that match a given KCP infra template.
func MatchesTemplateClonedFrom(infraConfigs map[string]*unstructured.Unstructured, kcp *controlplanev1.KThreesControlPlane) Func {
	return func(machine *clusterv1.Machine) bool {

		if machine == nil {
			return false
		}
		infraObj, found := infraConfigs[machine.Name]
		if !found {
			// Return true here because failing to get infrastructure machine should not be considered as unmatching.
			return true
		}

		clonedFromName, ok1 := infraObj.GetAnnotations()[clusterv1.TemplateClonedFromNameAnnotation]
		clonedFromGroupKind, ok2 := infraObj.GetAnnotations()[clusterv1.TemplateClonedFromGroupKindAnnotation]
		if !ok1 || !ok2 {
			// All kcp cloned infra machines should have this annotation.
			// Missing the annotation may be due to older version machines or adopted machines.
			// Should not be considered as mismatch.
			return true
		}

		// Check if the machine's infrastructure reference has been created from the current KCP infrastructure template.
		if clonedFromName != kcp.Spec.InfrastructureTemplate.Name ||
			clonedFromGroupKind != kcp.Spec.InfrastructureTemplate.GroupVersionKind().GroupKind().String() {
			return false
		}
		return true
	}
}

// MatchesKubernetesVersion returns a filter to find all machines that match a given Kubernetes version.
func MatchesKubernetesVersion(kubernetesVersion string) Func {
	return func(machine *clusterv1.Machine) bool {
		if machine == nil {
			return false
		}
		if machine.Spec.Version == nil {
			return false
		}
		return *machine.Spec.Version == kubernetesVersion
	}
}

// MatchesKThreesBootstrapConfig checks if machine's KThreesConfigSpec is equivalent with KCP's KThreesConfigSpec.
func MatchesKThreesBootstrapConfig(machineConfigs map[string]*bootstrapv1.KThreesConfig, kcp *controlplanev1.KThreesControlPlane) Func {
	return func(machine *clusterv1.Machine) bool {
		return true
	}
}

// getAdjustedKcpConfig takes the KThreesConfigSpec from KCP and applies the transformations required
// to allow a comparison with the KThreesConfig referenced from the machine.
// NOTE: The KCP controller applies a set of transformations when creating a KThreesConfig referenced from the machine,
// mostly depending on the fact that the machine was the initial control plane node or a joining control plane node.
// In this function we don't have such information, so we are making the KThreesConfigSpec similar to the KThreesConfig.
func getAdjustedKcpConfig(kcp *controlplanev1.KThreesControlPlane, machineConfig *bootstrapv1.KThreesConfig) *bootstrapv1.KThreesConfigSpec {

	return &kcp.Spec.KThreesConfigSpec
}

// cleanupConfigFields cleanups all the fields that are not relevant for the comparison.
func cleanupConfigFields(kcpConfig *bootstrapv1.KThreesConfigSpec, machineConfig *bootstrapv1.KThreesConfig) {

}
