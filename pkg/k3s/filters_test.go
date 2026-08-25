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
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s/desiredstate"
)

func TestUpToDate(t *testing.T) {
	const desiredVersion = "v1.31.2+k3s1"
	reconciliationTime := metav1.NewTime(time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))

	tests := []struct {
		name             string
		mutate           func(*controlplanev1.KThreesControlPlane, *clusterv1.Machine, *bootstrapv1.KThreesConfig, *unstructured.Unstructured)
		removeConfig     bool
		removeInfra      bool
		wantUpToDate     bool
		wantEligible     bool
		wantDesiredInfra bool
		wantDesiredBoot  bool
		wantMessages     []string
	}{
		{
			name:         "matching objects are up to date and ineligible",
			wantUpToDate: true,
		},
		{
			name: "version difference is eligible and carries desired Machine",
			mutate: func(kcp *controlplanev1.KThreesControlPlane, _ *clusterv1.Machine, _ *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				kcp.Spec.Version = desiredVersion
			},
			wantEligible: true,
			wantMessages: []string{"Version v1.31.1+k3s1, v1.31.2+k3s1 required"},
		},
		{
			name: "bootstrap difference is eligible and carries desired config",
			mutate: func(kcp *controlplanev1.KThreesControlPlane, _ *clusterv1.Machine, _ *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				kcp.Spec.KThreesConfigSpec.PostK3sCommands = []string{"new"}
			},
			wantEligible:    true,
			wantDesiredBoot: true,
			wantMessages:    []string{"KThreesConfig is not up-to-date"},
		},
		{
			name: "infrastructure template rotation is eligible and carries desired infra",
			mutate: func(kcp *controlplanev1.KThreesControlPlane, _ *clusterv1.Machine, _ *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				kcp.Spec.MachineTemplate.InfrastructureRef.Name = "template-2"
			},
			wantEligible:     true,
			wantDesiredInfra: true,
			wantMessages:     []string{"TestMachine is not up-to-date"},
		},
		{
			name: "delete annotation makes version difference ineligible",
			mutate: func(kcp *controlplanev1.KThreesControlPlane, machine *clusterv1.Machine, _ *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				kcp.Spec.Version = desiredVersion
				machine.Annotations[clusterv1.DeleteMachineAnnotation] = ""
			},
		},
		{
			name: "remediate annotation makes version difference ineligible",
			mutate: func(kcp *controlplanev1.KThreesControlPlane, machine *clusterv1.Machine, _ *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				kcp.Spec.Version = desiredVersion
				machine.Annotations[clusterv1.RemediateMachineAnnotation] = ""
			},
		},
		{
			name: "expired rolloutAfter is ineligible",
			mutate: func(kcp *controlplanev1.KThreesControlPlane, _ *clusterv1.Machine, _ *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				t := metav1.NewTime(reconciliationTime.Add(-time.Hour))
				kcp.Spec.RolloutAfter = &t
			},
			wantMessages: []string{"KThreesControlPlane spec.rolloutAfter expired"},
		},
		{
			name:         "missing bootstrap prevents in-place",
			removeConfig: true,
			wantUpToDate: true,
		},
		{
			name:         "missing infrastructure prevents in-place",
			removeInfra:  true,
			wantUpToDate: true,
		},
		{
			name: "combined version and infrastructure differences retain both desired objects",
			mutate: func(kcp *controlplanev1.KThreesControlPlane, _ *clusterv1.Machine, _ *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				kcp.Spec.Version = desiredVersion
				kcp.Spec.MachineTemplate.InfrastructureRef.Name = "template-2"
			},
			wantEligible:     true,
			wantDesiredInfra: true,
		},
		{
			name: "bootstrap version is normalized when desired bootstrap version is empty",
			mutate: func(_ *controlplanev1.KThreesControlPlane, _ *clusterv1.Machine, config *bootstrapv1.KThreesConfig, _ *unstructured.Unstructured) {
				config.Spec.Version = "v1.31.1+k3s1"
			},
			wantUpToDate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cluster, kcp, machine, config, infra, c := upToDateFixtures(t)
			if tt.mutate != nil {
				tt.mutate(kcp, machine, config, infra)
			}
			infraMachines := map[string]*unstructured.Unstructured{machine.Name: infra}
			configs := map[string]*bootstrapv1.KThreesConfig{machine.Name: config}
			if tt.removeInfra {
				delete(infraMachines, machine.Name)
			}
			if tt.removeConfig {
				delete(configs, machine.Name)
			}

			upToDate, result, err := UpToDate(
				context.Background(), c, cluster, machine, kcp, &reconciliationTime, infraMachines, configs,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(upToDate).To(Equal(tt.wantUpToDate))
			g.Expect(result.EligibleForInPlaceUpdate).To(Equal(tt.wantEligible))
			g.Expect(result.ConditionMessages).To(ContainElements(tt.wantMessages))
			g.Expect(result.DesiredMachine).NotTo(BeNil())
			g.Expect(result.DesiredMachine.Spec.Version).To(Equal(kcp.Spec.Version))
			if tt.wantDesiredInfra {
				g.Expect(result.DesiredInfraMachine).NotTo(BeNil())
			}
			if tt.wantDesiredBoot {
				g.Expect(result.DesiredKThreesConfig).NotTo(BeNil())
			}
			if tt.removeInfra {
				g.Expect(result.CurrentInfraMachine).To(BeNil())
				g.Expect(result.DesiredInfraMachine).To(BeNil())
			}
			if tt.removeConfig {
				g.Expect(result.CurrentKThreesConfig).To(BeNil())
				g.Expect(result.DesiredKThreesConfig).To(BeNil())
			}
		})
	}
}

func upToDateFixtures(t *testing.T) (
	*clusterv1.Cluster,
	*controlplanev1.KThreesControlPlane,
	*clusterv1.Machine,
	*bootstrapv1.KThreesConfig,
	*unstructured.Unstructured,
	*synchronizedClient,
) {
	t.Helper()
	g := NewWithT(t)
	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster-1", Namespace: "default"}}
	kcp := &controlplanev1.KThreesControlPlane{
		TypeMeta: metav1.TypeMeta{APIVersion: controlplanev1.GroupVersion.String(), Kind: "KThreesControlPlane"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kcp-1", Namespace: "default", UID: types.UID("kcp-uid"),
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
			KThreesConfigSpec: bootstrapv1.KThreesConfigSpec{
				PreK3sCommands: []string{"same"},
			},
		},
	}
	bootstrapRef := clusterv1.ContractVersionedObjectReference{
		APIGroup: bootstrapv1.GroupVersion.Group,
		Kind:     "KThreesConfig",
		Name:     "bootstrap-1",
	}
	infraRef := clusterv1.ContractVersionedObjectReference{
		APIGroup: "infrastructure.cluster.x-k8s.io",
		Kind:     "TestMachine",
		Name:     "infra-1",
	}
	machine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "machine-1",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)),
			Annotations:       map[string]string{},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName:       cluster.Name,
			Version:           kcp.Spec.Version,
			FailureDomain:     "fd-1",
			InfrastructureRef: infraRef,
			Bootstrap:         clusterv1.Bootstrap{ConfigRef: bootstrapRef},
		},
	}
	config, err := desiredstate.ComputeDesiredKThreesConfig(kcp, cluster, bootstrapRef.Name, nil)
	g.Expect(err).NotTo(HaveOccurred())
	infra := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       "TestMachine",
		"metadata": map[string]interface{}{
			"name":      infraRef.Name,
			"namespace": machine.Namespace,
			"annotations": map[string]interface{}{
				clusterv1.TemplateClonedFromNameAnnotation:      "template-1",
				clusterv1.TemplateClonedFromGroupKindAnnotation: "TestMachineTemplate.infrastructure.cluster.x-k8s.io",
			},
		},
		"spec": map[string]interface{}{"value": "old"},
	}}

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	crd := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
		Name: "testmachinetemplates.infrastructure.cluster.x-k8s.io",
		Labels: map[string]string{
			clusterv1.GroupVersion.String(): "v1beta1",
		},
	}}
	template1 := testInfraTemplate("default", "template-1", "old")
	template2 := testInfraTemplate("default", "template-2", "new")
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crd, template1, template2).Build()
	return cluster, kcp, machine, config, infra, &synchronizedClient{Client: client}
}

type synchronizedClient struct {
	client.Client
}

func testInfraTemplate(namespace, name, value string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
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
}
