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

package k3s_test

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/machinefilters" //nolint:staticcheck // Intentionally pin the deprecated external API.
)

var (
	_ func(*k3s.ControlPlane) collections.Machines                                                               = (*k3s.ControlPlane).MachinesNeedingRollout
	_ func(*k3s.ControlPlane) (collections.Machines, map[string]k3s.UpToDateResult)                              = (*k3s.ControlPlane).MachinesNeedingRolloutWithResults
	_ func(*k3s.ControlPlane) (collections.Machines, map[string]k3s.UpToDateResult)                              = (*k3s.ControlPlane).NotUpToDateMachines
	_ func(*k3s.ControlPlane) collections.Machines                                                               = (*k3s.ControlPlane).UpToDateMachines
	_ func(*k3s.ControlPlane, *bootstrapv1.KThreesConfigSpec) *bootstrapv1.KThreesConfig                         = (*k3s.ControlPlane).GenerateKThreesConfig
	_ func(*k3s.ControlPlane, *corev1.ObjectReference, *corev1.ObjectReference, *string) *clusterv1beta1.Machine = (*k3s.ControlPlane).NewMachine
	_ func(string, controlplanev1.KThreesControlPlaneMachineTemplate) map[string]string                          = k3s.ControlPlaneLabelsForCluster

	_ func(map[string]*unstructured.Unstructured, map[string]*bootstrapv1.KThreesConfig, *controlplanev1.KThreesControlPlane) func(*clusterv1.Machine) bool = machinefilters.MatchesKCPConfiguration
	_ func(map[string]*unstructured.Unstructured, *controlplanev1.KThreesControlPlane) machinefilters.Func                                                  = machinefilters.MatchesTemplateClonedFrom
	_ func(string) machinefilters.Func                                                                                                                      = machinefilters.MatchesKubernetesVersion
	_ func(map[string]*bootstrapv1.KThreesConfig, *controlplanev1.KThreesControlPlane) machinefilters.Func                                                  = machinefilters.MatchesKThreesBootstrapConfig
	_ func() machinefilters.Func                                                                                                                            = machinefilters.AgentHealthy
)

func TestMachinesNeedingRolloutCompositeLiteral(t *testing.T) {
	g := NewWithT(t)
	const (
		desiredVersion = "v1.31.2+k3s1"
		storedVersion  = "v1.30.8+k3s1"
	)
	kcp := &controlplanev1.KThreesControlPlane{
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Version: desiredVersion,
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "TestMachineTemplate",
					Name:       "desired-template",
				},
			},
			KThreesConfigSpec: bootstrapv1.KThreesConfigSpec{
				PreK3sCommands: []string{"desired"},
			},
		},
	}
	bootstrapRef := clusterv1.ContractVersionedObjectReference{
		APIGroup: bootstrapv1.GroupVersion.Group,
		Kind:     "KThreesConfig",
		Name:     "config",
	}
	newMachine := func(name, version string) *clusterv1.Machine {
		return &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: clusterv1.MachineSpec{
				Version:   version,
				Bootstrap: clusterv1.Bootstrap{ConfigRef: bootstrapRef},
			},
		}
	}
	matching := newMachine("matching", desiredVersion)
	versionMismatch := newMachine("version-mismatch", storedVersion)
	bootstrapMismatch := newMachine("bootstrap-mismatch", desiredVersion)
	infraMismatch := newMachine("infra-mismatch", desiredVersion)
	deletingMismatch := newMachine("deleting-mismatch", storedVersion)
	now := metav1.Now()
	deletingMismatch.DeletionTimestamp = &now

	machines := collections.FromMachines(matching, versionMismatch, bootstrapMismatch, infraMismatch, deletingMismatch)
	configs := map[string]*bootstrapv1.KThreesConfig{}
	infraResources := map[string]*unstructured.Unstructured{}
	for name := range machines {
		configs[name] = &bootstrapv1.KThreesConfig{
			Spec: bootstrapv1.KThreesConfigSpec{
				Version:        storedVersion,
				PreK3sCommands: []string{"desired"},
			},
		}
		infraResources[name] = &unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					clusterv1beta1.TemplateClonedFromNameAnnotation:      "desired-template",
					clusterv1beta1.TemplateClonedFromGroupKindAnnotation: "TestMachineTemplate.infrastructure.cluster.x-k8s.io",
				},
			},
		}}
	}
	configs[bootstrapMismatch.Name].Spec.PreK3sCommands = []string{"old"}
	infraResources[infraMismatch.Name].SetAnnotations(map[string]string{
		clusterv1beta1.TemplateClonedFromNameAnnotation:      "old-template",
		clusterv1beta1.TemplateClonedFromGroupKindAnnotation: "TestMachineTemplate.infrastructure.cluster.x-k8s.io",
	})

	controlPlane := &k3s.ControlPlane{
		KCP:            kcp,
		Cluster:        &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster-1"}},
		Machines:       machines,
		KthreesConfigs: configs,
		InfraResources: infraResources,
	}

	g.Expect(controlPlane.MachinesNeedingRollout().Names()).To(ConsistOf(
		versionMismatch.Name,
		bootstrapMismatch.Name,
		infraMismatch.Name,
	))
	g.Expect(controlPlane.UpToDateMachines().Names()).To(ConsistOf(matching.Name))
	notUpToDate, results := controlPlane.NotUpToDateMachines()
	g.Expect(notUpToDate.Names()).To(ConsistOf(
		versionMismatch.Name,
		bootstrapMismatch.Name,
		infraMismatch.Name,
		deletingMismatch.Name,
	))
	g.Expect(results).NotTo(BeNil())
	g.Expect(results).To(BeEmpty())
	g.Expect(configs[matching.Name].Spec.Version).To(Equal(storedVersion))
}

func TestLegacyControlPlaneConstructionAPIs(t *testing.T) {
	g := NewWithT(t)
	failureDomain := "failure-domain-1"
	kcp := &controlplanev1.KThreesControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kcp-1",
			Namespace: "default",
			UID:       "kcp-uid",
		},
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Version: "v1.31.2+k3s1",
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				ObjectMeta: clusterv1beta1.ObjectMeta{
					Labels: map[string]string{"custom": "label"},
				},
			},
		},
	}
	controlPlane := &k3s.ControlPlane{
		KCP:     kcp,
		Cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster-1"}},
	}

	labels := k3s.ControlPlaneLabelsForCluster(controlPlane.Cluster.Name, kcp.Spec.MachineTemplate)
	g.Expect(labels).To(HaveKeyWithValue("custom", "label"))
	g.Expect(labels).To(HaveKeyWithValue(clusterv1beta1.ClusterNameLabel, controlPlane.Cluster.Name))
	g.Expect(labels).To(HaveKey(clusterv1beta1.MachineControlPlaneLabel))
	labels["custom"] = "changed"
	g.Expect(kcp.Spec.MachineTemplate.ObjectMeta.Labels).To(HaveKeyWithValue("custom", "label"))

	configSpec := &bootstrapv1.KThreesConfigSpec{Version: kcp.Spec.Version}
	config := controlPlane.GenerateKThreesConfig(configSpec)
	g.Expect(config.Name).To(HavePrefix(kcp.Name + "-"))
	g.Expect(config.Namespace).To(Equal(kcp.Namespace))
	g.Expect(config.Labels).To(HaveKeyWithValue(clusterv1beta1.ClusterNameLabel, controlPlane.Cluster.Name))
	g.Expect(config.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
		APIVersion: controlplanev1.GroupVersion.String(),
		Kind:       "KThreesControlPlane",
		Name:       kcp.Name,
		UID:        kcp.UID,
	}))
	g.Expect(config.Spec).To(Equal(*configSpec))

	infraRef := &corev1.ObjectReference{APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1", Kind: "TestMachine", Name: "infra-1"}
	bootstrapRef := &corev1.ObjectReference{APIVersion: bootstrapv1.GroupVersion.String(), Kind: "KThreesConfig", Name: config.Name}
	machine := controlPlane.NewMachine(infraRef, bootstrapRef, &failureDomain)
	g.Expect(machine.Name).To(HavePrefix(kcp.Name + "-"))
	g.Expect(machine.Namespace).To(Equal(kcp.Namespace))
	g.Expect(machine.Labels).To(HaveKeyWithValue(clusterv1beta1.ClusterNameLabel, controlPlane.Cluster.Name))
	g.Expect(machine.Spec.ClusterName).To(Equal(controlPlane.Cluster.Name))
	g.Expect(machine.Spec.Version).To(Equal(&kcp.Spec.Version))
	g.Expect(machine.Spec.InfrastructureRef).To(Equal(*infraRef))
	g.Expect(machine.Spec.Bootstrap.ConfigRef).To(Equal(bootstrapRef))
	g.Expect(machine.Spec.FailureDomain).To(Equal(&failureDomain))
	g.Expect(metav1.IsControlledBy(machine, kcp)).To(BeTrue())
}
