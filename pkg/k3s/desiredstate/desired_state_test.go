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

package desiredstate

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

func TestComputeDesiredMachine(t *testing.T) {
	cluster, kcp := desiredStateFixtures()
	infraRef := clusterv1.ContractVersionedObjectReference{
		APIGroup: "infrastructure.cluster.x-k8s.io",
		Kind:     "TestMachine",
		Name:     "infra-1",
	}
	bootstrapRef := clusterv1.ContractVersionedObjectReference{
		APIGroup: bootstrapv1.GroupVersion.Group,
		Kind:     "KThreesConfig",
		Name:     "bootstrap-1",
	}

	tests := []struct {
		name     string
		existing *clusterv1.Machine
		check    func(*WithT, *clusterv1.Machine)
	}{
		{
			name: "new Machine receives desired identity and references",
			check: func(g *WithT, machine *clusterv1.Machine) {
				g.Expect(machine.Name).To(HavePrefix(kcp.Name + "-"))
				g.Expect(machine.Namespace).To(Equal(kcp.Namespace))
				g.Expect(machine.Spec.Version).To(Equal(kcp.Spec.Version))
				g.Expect(machine.Spec.InfrastructureRef).To(Equal(infraRef))
				g.Expect(machine.Spec.Bootstrap.ConfigRef).To(Equal(bootstrapRef))
				g.Expect(machine.Spec.FailureDomain).To(Equal("fd-1"))
				g.Expect(machine.Labels).To(HaveKeyWithValue(clusterv1beta1.ClusterNameLabel, cluster.Name))
				g.Expect(machine.Labels).To(HaveKeyWithValue("template-label", "desired"))
				g.Expect(machine.Annotations).To(HaveKeyWithValue("template-annotation", "desired"))
				g.Expect(machine.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(kcp, controlplanev1.GroupVersion.WithKind("KThreesControlPlane"))))
			},
		},
		{
			name: "existing Machine preserves immutable identity references failure domain and current version",
			existing: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "machine-1",
					Namespace:   "default",
					UID:         types.UID("machine-uid"),
					Annotations: map[string]string{controlplanev1.RemediationForAnnotation: "machine-old"},
				},
				Spec: clusterv1.MachineSpec{
					Version:           "v1.30.4+k3s1",
					FailureDomain:     "fd-existing",
					InfrastructureRef: infraRef,
					Bootstrap:         clusterv1.Bootstrap{ConfigRef: bootstrapRef},
				},
			},
			check: func(g *WithT, machine *clusterv1.Machine) {
				g.Expect(machine.Name).To(Equal("machine-1"))
				g.Expect(machine.UID).To(Equal(types.UID("machine-uid")))
				g.Expect(machine.Spec.Version).To(Equal("v1.30.4+k3s1"))
				g.Expect(machine.Spec.InfrastructureRef).To(Equal(infraRef))
				g.Expect(machine.Spec.Bootstrap.ConfigRef).To(Equal(bootstrapRef))
				g.Expect(machine.Spec.FailureDomain).To(Equal("fd-existing"))
				g.Expect(machine.Annotations).To(HaveKeyWithValue(controlplanev1.RemediationForAnnotation, "machine-old"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			failureDomain := "fd-1"
			if tt.existing != nil {
				failureDomain = tt.existing.Spec.FailureDomain
			}
			machine, err := ComputeDesiredMachine(kcp, cluster, infraRef, bootstrapRef, failureDomain, tt.existing)
			g.Expect(err).NotTo(HaveOccurred())
			tt.check(g, machine)
		})
	}
}

func TestComputeDesiredKThreesConfig(t *testing.T) {
	cluster, kcp := desiredStateFixtures()
	kcp.Spec.KThreesConfigSpec = bootstrapv1.KThreesConfigSpec{
		Version:         "",
		PreK3sCommands:  []string{"desired-pre"},
		PostK3sCommands: []string{"desired-post"},
	}

	tests := []struct {
		name     string
		existing *bootstrapv1.KThreesConfig
		check    func(*WithT, *bootstrapv1.KThreesConfig)
	}{
		{
			name: "new config receives desired metadata and spec",
			check: func(g *WithT, config *bootstrapv1.KThreesConfig) {
				g.Expect(config.Name).To(Equal("bootstrap-1"))
				g.Expect(config.Namespace).To(Equal(kcp.Namespace))
				g.Expect(config.Spec.PreK3sCommands).To(Equal([]string{"desired-pre"}))
				g.Expect(config.Labels).To(HaveKeyWithValue("template-label", "desired"))
				g.Expect(config.Annotations).To(HaveKeyWithValue("template-annotation", "desired"))
				g.Expect(config.OwnerReferences).To(HaveLen(1))
				g.Expect(config.OwnerReferences[0].Kind).To(Equal("KThreesControlPlane"))
			},
		},
		{
			name: "existing config preserves identity owner and current version while receiving desired fields",
			existing: &bootstrapv1.KThreesConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bootstrap-existing",
					Namespace: "default",
					UID:       types.UID("bootstrap-uid"),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       "machine-1",
						UID:        types.UID("machine-uid"),
					}},
				},
				Spec: bootstrapv1.KThreesConfigSpec{
					Version:        "v1.30.4+k3s1",
					PreK3sCommands: []string{"old"},
				},
			},
			check: func(g *WithT, config *bootstrapv1.KThreesConfig) {
				g.Expect(config.Name).To(Equal("bootstrap-existing"))
				g.Expect(config.UID).To(Equal(types.UID("bootstrap-uid")))
				g.Expect(config.OwnerReferences).To(BeEmpty())
				g.Expect(config.Spec.Version).To(Equal("v1.30.4+k3s1"))
				g.Expect(config.Spec.PreK3sCommands).To(Equal([]string{"desired-pre"}))
				g.Expect(config.Spec.PostK3sCommands).To(Equal([]string{"desired-post"}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			config, err := ComputeDesiredKThreesConfig(kcp, cluster, "bootstrap-1", tt.existing)
			g.Expect(err).NotTo(HaveOccurred())
			tt.check(g, config)
		})
	}
}

func TestComputeDesiredInfraMachine(t *testing.T) {
	g := NewWithT(t)
	cluster, kcp := desiredStateFixtures()
	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testmachinetemplates.infrastructure.cluster.x-k8s.io",
			Labels: map[string]string{
				clusterv1.GroupVersion.String(): "v1beta1",
			},
		},
	}
	template := infraTemplate(kcp.Namespace, "template-1", "old")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crd, template).Build()

	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1beta1")
	existing.SetKind("TestMachine")
	existing.SetNamespace(kcp.Namespace)
	existing.SetName("infra-existing")
	existing.SetUID(types.UID("infra-uid"))

	desired, err := ComputeDesiredInfraMachine(context.Background(), c, kcp, cluster, "infra-new", existing)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(desired.GetName()).To(Equal("infra-existing"))
	g.Expect(desired.GetUID()).To(Equal(types.UID("infra-uid")))
	g.Expect(desired.Object).To(HaveKeyWithValue("spec", map[string]interface{}{"value": "old"}))
	g.Expect(desired.GetAnnotations()).To(HaveKeyWithValue(clusterv1.TemplateClonedFromNameAnnotation, "template-1"))

	updatedTemplate := infraTemplate(kcp.Namespace, "template-2", "new")
	g.Expect(c.Create(context.Background(), updatedTemplate)).To(Succeed())
	kcp.Spec.MachineTemplate.InfrastructureRef.Name = "template-2"

	desired, err = ComputeDesiredInfraMachine(context.Background(), c, kcp, cluster, "infra-new", existing)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(desired.Object).To(HaveKeyWithValue("spec", map[string]interface{}{"value": "new"}))
	g.Expect(desired.GetAnnotations()).To(HaveKeyWithValue(clusterv1.TemplateClonedFromNameAnnotation, "template-2"))
}

func TestDesiredStateFiltersReservedMachineTemplateAnnotations(t *testing.T) {
	g := NewWithT(t)
	cluster, kcp := desiredStateFixtures()
	for _, annotation := range []string{
		clusterv1.TemplateClonedFromNameAnnotation,
		clusterv1.TemplateClonedFromGroupKindAnnotation,
		clusterv1.UpdateInProgressAnnotation,
	} {
		kcp.Spec.MachineTemplate.ObjectMeta.Annotations[annotation] = "spoofed"
	}

	infraRef := clusterv1.ContractVersionedObjectReference{
		APIGroup: "infrastructure.cluster.x-k8s.io",
		Kind:     "TestMachine",
		Name:     "infra-1",
	}
	bootstrapRef := clusterv1.ContractVersionedObjectReference{
		APIGroup: bootstrapv1.GroupVersion.Group,
		Kind:     "KThreesConfig",
		Name:     "bootstrap-1",
	}
	machine, err := ComputeDesiredMachine(kcp, cluster, infraRef, bootstrapRef, "fd-1", nil)
	g.Expect(err).NotTo(HaveOccurred())
	config, err := ComputeDesiredKThreesConfig(kcp, cluster, "bootstrap-1", nil)
	g.Expect(err).NotTo(HaveOccurred())

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "testmachinetemplates.infrastructure.cluster.x-k8s.io",
			Labels: map[string]string{clusterv1.GroupVersion.String(): "v1beta1"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		crd,
		infraTemplate(kcp.Namespace, "template-1", "desired"),
	).Build()
	infraMachine, err := ComputeDesiredInfraMachine(context.Background(), c, kcp, cluster, "infra-1", nil)
	g.Expect(err).NotTo(HaveOccurred())

	for name, annotations := range map[string]map[string]string{
		"Machine":        machine.GetAnnotations(),
		"KThreesConfig":  config.GetAnnotations(),
		"Infrastructure": infraMachine.GetAnnotations(),
	} {
		g.Expect(annotations).To(HaveKeyWithValue("template-annotation", "desired"), name)
		g.Expect(annotations).NotTo(HaveKey(clusterv1.UpdateInProgressAnnotation), name)
	}
	g.Expect(machine.GetAnnotations()).NotTo(HaveKey(clusterv1.TemplateClonedFromNameAnnotation))
	g.Expect(machine.GetAnnotations()).NotTo(HaveKey(clusterv1.TemplateClonedFromGroupKindAnnotation))
	g.Expect(config.GetAnnotations()).NotTo(HaveKey(clusterv1.TemplateClonedFromNameAnnotation))
	g.Expect(config.GetAnnotations()).NotTo(HaveKey(clusterv1.TemplateClonedFromGroupKindAnnotation))
	g.Expect(infraMachine.GetAnnotations()).To(HaveKeyWithValue(
		clusterv1.TemplateClonedFromNameAnnotation,
		"template-1",
	))
	g.Expect(infraMachine.GetAnnotations()).To(HaveKeyWithValue(
		clusterv1.TemplateClonedFromGroupKindAnnotation,
		"TestMachineTemplate.infrastructure.cluster.x-k8s.io",
	))
}

func desiredStateFixtures() (*clusterv1.Cluster, *controlplanev1.KThreesControlPlane) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-1",
			Namespace: "default",
		},
	}
	kcp := &controlplanev1.KThreesControlPlane{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KThreesControlPlane",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kcp-1",
			Namespace: "default",
			UID:       types.UID("kcp-uid"),
		},
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Version: "v1.31.1+k3s1",
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				ObjectMeta: clusterv1beta1.ObjectMeta{
					Labels:      map[string]string{"template-label": "desired"},
					Annotations: map[string]string{"template-annotation": "desired"},
				},
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "TestMachineTemplate",
					Name:       "template-1",
				},
			},
		},
	}
	return cluster, kcp
}

func infraTemplate(namespace, name, value string) *unstructured.Unstructured {
	template := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       "TestMachineTemplate",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{"value": value},
			},
		},
	}}
	return template
}
