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
	_ func(*k3s.ControlPlane, *bootstrapv1.KThreesConfigSpec) *bootstrapv1.KThreesConfig                         = (*k3s.ControlPlane).GenerateKThreesConfig
	_ func(*k3s.ControlPlane, *corev1.ObjectReference, *corev1.ObjectReference, *string) *clusterv1beta1.Machine = (*k3s.ControlPlane).NewMachine
	_ func(string, controlplanev1.KThreesControlPlaneMachineTemplate) map[string]string                          = k3s.ControlPlaneLabelsForCluster

	_ func(map[string]*unstructured.Unstructured, map[string]*bootstrapv1.KThreesConfig, *controlplanev1.KThreesControlPlane) func(*clusterv1.Machine) bool = machinefilters.MatchesKCPConfiguration
	_ func(map[string]*unstructured.Unstructured, *controlplanev1.KThreesControlPlane) machinefilters.Func                                                  = machinefilters.MatchesTemplateClonedFrom
	_ func(string) machinefilters.Func                                                                                                                      = machinefilters.MatchesKubernetesVersion
	_ func(map[string]*bootstrapv1.KThreesConfig, *controlplanev1.KThreesControlPlane) machinefilters.Func                                                  = machinefilters.MatchesKThreesBootstrapConfig
	_ func() machinefilters.Func                                                                                                                            = machinefilters.AgentHealthy
)

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
