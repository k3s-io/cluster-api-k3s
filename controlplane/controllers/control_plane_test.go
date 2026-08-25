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

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

func TestReconcileMachineUpToDateCondition(t *testing.T) {
	tests := []struct {
		name       string
		kcpVersion string
		inProgress bool
		wantStatus metav1.ConditionStatus
		wantReason string
		wantText   string
	}{
		{
			name:       "outdated Machine",
			kcpVersion: "v1.31.2+k3s1",
			wantStatus: metav1.ConditionFalse,
			wantReason: clusterv1.MachineNotUpToDateReason,
			wantText:   "Version v1.31.1+k3s1, v1.31.2+k3s1 required",
		},
		{
			name:       "in-place update in progress",
			kcpVersion: "v1.31.1+k3s1",
			inProgress: true,
			wantStatus: metav1.ConditionFalse,
			wantReason: clusterv1.MachineUpToDateUpdatingReason,
			wantText:   "In-place update in progress",
		},
		{
			name:       "in-place update in progress with newly changed desired version",
			kcpVersion: "v1.31.2+k3s1",
			inProgress: true,
			wantStatus: metav1.ConditionFalse,
			wantReason: clusterv1.MachineUpToDateUpdatingReason,
			wantText:   "In-place update in progress",
		},
		{
			name:       "up-to-date Machine",
			kcpVersion: "v1.31.1+k3s1",
			wantStatus: metav1.ConditionTrue,
			wantReason: clusterv1.MachineUpToDateReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			controlPlane := newConditionControlPlane(t, tt.kcpVersion, tt.inProgress)
			machine := controlPlane.Machines["machine-1"]
			conditions.Set(machine, metav1.Condition{
				Type:   controlplanev1.MachineAgentHealthyV1Beta2Condition,
				Status: metav1.ConditionTrue,
				Reason: "Healthy",
			})

			reconcileMachineUpToDateCondition(context.Background(), controlPlane)

			condition := conditions.Get(machine, clusterv1.MachineUpToDateCondition)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(tt.wantStatus))
			g.Expect(condition.Reason).To(Equal(tt.wantReason))
			g.Expect(condition.Message).To(ContainSubstring(tt.wantText))
			g.Expect(conditions.IsTrue(machine, controlplanev1.MachineAgentHealthyV1Beta2Condition)).To(BeTrue())
		})
	}
}

func TestReconcileControlPlaneConditionsPatchesUpToDateBeforeInitialization(t *testing.T) {
	g := NewWithT(t)
	controlPlane, c := newConditionControlPlaneAndClient(t, "v1.31.2+k3s1", false)
	g.Expect(controlPlane.KCP.Status.Initialized).To(BeFalse())

	r := &KThreesControlPlaneReconciler{}
	g.Expect(r.reconcileControlPlaneConditions(context.Background(), controlPlane)).To(Succeed())

	actualMachine := &clusterv1.Machine{}
	g.Expect(c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "machine-1"}, actualMachine)).To(Succeed())
	condition := conditions.Get(actualMachine, clusterv1.MachineUpToDateCondition)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(condition.Reason).To(Equal(clusterv1.MachineNotUpToDateReason))
	g.Expect(condition.Message).To(ContainSubstring("Version v1.31.1+k3s1, v1.31.2+k3s1 required"))
}

func newConditionControlPlane(t *testing.T, kcpVersion string, inProgress bool) *k3s.ControlPlane {
	t.Helper()
	controlPlane, _ := newConditionControlPlaneAndClient(t, kcpVersion, inProgress)
	return controlPlane
}

func newConditionControlPlaneAndClient(t *testing.T, kcpVersion string, inProgress bool) (*k3s.ControlPlane, client.Client) {
	t.Helper()
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())

	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster-1", Namespace: "default"}}
	kcp := &controlplanev1.KThreesControlPlane{
		TypeMeta:   metav1.TypeMeta{APIVersion: controlplanev1.GroupVersion.String(), Kind: "KThreesControlPlane"},
		ObjectMeta: metav1.ObjectMeta{Name: "kcp-1", Namespace: "default"},
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Version: kcpVersion,
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "TestMachineTemplate",
					Name:       "template-1",
				},
			},
		},
	}
	annotations := map[string]string{}
	if inProgress {
		annotations[clusterv1.UpdateInProgressAnnotation] = ""
	}
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: "machine-1", Namespace: "default", Annotations: annotations},
		Spec: clusterv1.MachineSpec{
			ClusterName:       cluster.Name,
			Version:           "v1.31.1+k3s1",
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{APIGroup: "infrastructure.cluster.x-k8s.io", Kind: "TestMachine", Name: "infra-1"},
			Bootstrap:         clusterv1.Bootstrap{ConfigRef: clusterv1.ContractVersionedObjectReference{APIGroup: bootstrapv1.GroupVersion.Group, Kind: "KThreesConfig", Name: "bootstrap-1"}},
		},
	}
	config := &bootstrapv1.KThreesConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-1", Namespace: "default"},
	}
	infra := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       "TestMachine",
		"metadata": map[string]interface{}{
			"name":      "infra-1",
			"namespace": "default",
			"annotations": map[string]interface{}{
				clusterv1.TemplateClonedFromNameAnnotation:      "template-1",
				clusterv1.TemplateClonedFromGroupKindAnnotation: "TestMachineTemplate.infrastructure.cluster.x-k8s.io",
			},
		},
		"spec": map[string]interface{}{"value": "same"},
	}}
	template := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       "TestMachineTemplate",
		"metadata":   map[string]interface{}{"name": "template-1", "namespace": "default"},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{"spec": map[string]interface{}{"value": "same"}},
		},
	}}
	crd := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
		Name: "testmachinetemplates.infrastructure.cluster.x-k8s.io",
		Labels: map[string]string{
			clusterv1.GroupVersion.String(): "v1beta1",
		},
	}}
	machineCRD := &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
		Name: "testmachines.infrastructure.cluster.x-k8s.io",
		Labels: map[string]string{
			clusterv1.GroupVersion.String(): "v1beta1",
		},
	}}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&clusterv1.Machine{}).
		WithObjects(machine, config, infra, template, crd, machineCRD).
		Build()
	controlPlane, err := k3s.NewControlPlane(context.Background(), c, cluster, kcp, collections.FromMachines(machine))
	g.Expect(err).NotTo(HaveOccurred())
	return controlPlane, c
}
