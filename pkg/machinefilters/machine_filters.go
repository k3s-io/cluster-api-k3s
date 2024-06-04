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
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

type Func = collections.Func

// MatchesKCPConfiguration returns a filter to find all machines that matches with KCP config and do not require any rollout.
// Kubernetes version, infrastructure template, and KThreesConfig field need to be equivalent.
func MatchesKCPConfiguration(infraConfigs map[string]*unstructured.Unstructured, machineConfigs map[string]*bootstrapv1.KThreesConfig, kcp *controlplanev1.KThreesControlPlane) func(machine *clusterv1.Machine) bool {
	return collections.And(
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
		if clonedFromName != kcp.Spec.MachineTemplate.InfrastructureRef.Name ||
			clonedFromGroupKind != kcp.Spec.MachineTemplate.InfrastructureRef.GroupVersionKind().GroupKind().String() {
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
		if machine == nil {
			return false
		}

		bootstrapRef := machine.Spec.Bootstrap.ConfigRef
		if bootstrapRef == nil {
			// Missing bootstrap reference should not be considered as unmatching.
			// This is a safety precaution to avoid selecting machines that are broken, which in the future should be remediated separately.
			return true
		}

		machineConfig, found := machineConfigs[machine.Name]
		if !found {
			// Return true here because failing to get KThreesConfig should not be considered as unmatching.
			// This is a safety precaution to avoid rolling out machines if the client or the api-server is misbehaving.
			return true
		}

		kcpConfig := kcp.Spec.KThreesConfigSpec.DeepCopy()

		// KCP version check is handled elsewhere
		kcpConfig.Version = ""
		machineConfig.Spec.Version = ""

		return reflect.DeepEqual(&machineConfig.Spec, kcpConfig)
	}
}

// AgentHealthy returns a filter to find all machines that have an AgentHealthy
// set to true.
func AgentHealthy() Func {
	return func(machine *clusterv1.Machine) bool {
		if machine == nil {
			return false
		}
		return conditions.IsTrue(machine, controlplanev1.MachineAgentHealthyCondition)
	}
}
