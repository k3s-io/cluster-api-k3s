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
	"errors"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/util/ssa"
)

const (
	relatedObjectTestGroup        = "infrastructure.cluster.x-k8s.io"
	relatedObjectTestVersion      = "v1beta2"
	relatedObjectTestMachineKind  = "OwnershipTestMachine"
	relatedObjectTestTemplateKind = "OwnershipTestMachineTemplate"
)

var _ = Describe("new related object ownership", func() {
	BeforeEach(func() {
		ensureManagedFieldsTestCRD(
			"ownershiptestmachines."+relatedObjectTestGroup,
			relatedObjectTestGroup,
			relatedObjectTestMachineKind,
			"ownershiptestmachines",
		)
		ensureManagedFieldsTestCRD(
			"ownershiptestmachinetemplates."+relatedObjectTestGroup,
			relatedObjectTestGroup,
			relatedObjectTestTemplateKind,
			"ownershiptestmachinetemplates",
		)
		ensureManagedFieldsTestCRD(
			"machines."+clusterv1.GroupVersion.Group,
			clusterv1.GroupVersion.Group,
			"Machine",
			"machines",
		)
		ensureManagedFieldsTestCRD(
			"testmachines."+relatedObjectTestGroup,
			relatedObjectTestGroup,
			testMachineKind,
			"testmachines",
		)
		ensureRelatedObjectContractLabels()
		ensureKThreesConfigContractLabel()
	})

	It("creates complete related objects with split metadata ownership", func() {
		ctx := context.Background()
		fixture := newRelatedObjectOwnershipFixture(ctx)
		defer fixture.cleanup(ctx)

		reconciler := &KThreesControlPlaneReconciler{
			Client:    k8sClient,
			apiReader: k8sClient,
			ssaCache:  ssa.NewCache(),
		}
		Expect(reconciler.cloneConfigsAndGenerateMachine(
			ctx,
			fixture.cluster,
			fixture.kcp,
			&fixture.kcp.Spec.KThreesConfigSpec,
			"",
		)).To(Succeed())

		machine := fixture.getMachine(ctx)
		Expect(machine).NotTo(BeNil())

		infraMachine := fixture.getInfraMachine(ctx, machine.Spec.InfrastructureRef)
		kthreesConfig := fixture.getKThreesConfig(ctx, machine.Spec.Bootstrap.ConfigRef)
		for _, object := range []client.Object{infraMachine, kthreesConfig} {
			mainEntry := findManagedField(object, kcpManagerName, metav1.ManagedFieldsOperationApply, "")
			Expect(mainEntry).NotTo(BeNil())
			Expect(managedFieldOwns(mainEntry, "f:spec")).To(BeTrue())
			Expect(managedFieldOwns(mainEntry, "f:metadata", "f:labels")).To(BeFalse())
			Expect(managedFieldOwns(mainEntry, "f:metadata", "f:annotations")).To(BeFalse())

			metadataEntry := findManagedField(object, kcpMetadataManagerName, metav1.ManagedFieldsOperationApply, "")
			Expect(metadataEntry).NotTo(BeNil())
			Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:labels")).To(BeTrue())
			Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:annotations")).To(BeTrue())
		}
	})

	It("removes omitted related-object spec fields on a later main-manager apply", func() {
		ctx := context.Background()
		fixture := newRelatedObjectOwnershipFixture(ctx)
		defer fixture.cleanup(ctx)
		reconciler := &KThreesControlPlaneReconciler{
			Client:    k8sClient,
			apiReader: k8sClient,
			ssaCache:  ssa.NewCache(),
		}
		Expect(reconciler.cloneConfigsAndGenerateMachine(
			ctx,
			fixture.cluster,
			fixture.kcp,
			&fixture.kcp.Spec.KThreesConfigSpec,
			"",
		)).To(Succeed())

		machine := fixture.getMachine(ctx)
		infraMachine := fixture.getInfraMachine(ctx, machine.Spec.InfrastructureRef)
		desiredInfraMachine := infraMachine.DeepCopy()
		desiredInfraMachine.SetLabels(nil)
		desiredInfraMachine.SetAnnotations(nil)
		unstructured.RemoveNestedField(desiredInfraMachine.Object, "spec", "settings", "legacy")
		Expect(ssa.Patch(ctx, k8sClient, kcpManagerName, desiredInfraMachine)).To(Succeed())

		kthreesConfig := fixture.getKThreesConfig(ctx, machine.Spec.Bootstrap.ConfigRef)
		desiredKThreesConfig := kthreesConfig.DeepCopy()
		desiredKThreesConfig.Labels = nil
		desiredKThreesConfig.Annotations = nil
		desiredKThreesConfig.Spec.PostK3sCommands = nil
		Expect(ssa.Patch(ctx, k8sClient, kcpManagerName, desiredKThreesConfig)).To(Succeed())

		infraMachine = fixture.getInfraMachine(ctx, machine.Spec.InfrastructureRef)
		_, found, err := unstructured.NestedString(infraMachine.Object, "spec", "settings", "legacy")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		stable, found, err := unstructured.NestedString(infraMachine.Object, "spec", "settings", "stable")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(stable).To(Equal("kept"))

		kthreesConfig = fixture.getKThreesConfig(ctx, machine.Spec.Bootstrap.ConfigRef)
		Expect(kthreesConfig.Spec.PostK3sCommands).To(BeEmpty())
	})

	It("cleans up an InfraMachine when ownership setup fails after its SSA create", func() {
		ctx := context.Background()
		fixture := newRelatedObjectOwnershipFixture(ctx)
		defer fixture.cleanup(ctx)
		failingClient := &relatedObjectSetupFailureClient{
			Client:                   k8sClient,
			failManagedFieldsPatchAt: 1,
		}
		reconciler := &KThreesControlPlaneReconciler{
			Client:    failingClient,
			apiReader: k8sClient,
			ssaCache:  ssa.NewCache(),
		}

		err := reconciler.cloneConfigsAndGenerateMachine(
			ctx,
			fixture.cluster,
			fixture.kcp,
			&fixture.kcp.Spec.KThreesConfigSpec,
			"",
		)
		Expect(err).To(MatchError(ContainSubstring("failed to split managedFields ownership for " + relatedObjectTestMachineKind)))
		fixture.expectInfraMachineCount(ctx, 0)
	})

	It("does not create a Machine when the related-object SSA create fails", func() {
		ctx := context.Background()
		fixture := newRelatedObjectOwnershipFixture(ctx)
		defer fixture.cleanup(ctx)
		failingClient := &relatedObjectSetupFailureClient{
			Client:           k8sClient,
			failApplyPatchAt: 1,
		}
		reconciler := &KThreesControlPlaneReconciler{
			Client:    failingClient,
			apiReader: k8sClient,
			ssaCache:  ssa.NewCache(),
		}

		err := reconciler.cloneConfigsAndGenerateMachine(
			ctx,
			fixture.cluster,
			fixture.kcp,
			&fixture.kcp.Spec.KThreesConfigSpec,
			"",
		)
		Expect(err).To(MatchError(ContainSubstring("failed to create " + relatedObjectTestMachineKind)))
		fixture.expectInfraMachineCount(ctx, 0)
	})

	It("cleans up related objects when metadata ownership establishment fails", func() {
		ctx := context.Background()
		fixture := newRelatedObjectOwnershipFixture(ctx)
		defer fixture.cleanup(ctx)
		failingClient := &relatedObjectSetupFailureClient{
			Client:           k8sClient,
			failApplyPatchAt: 2,
		}
		reconciler := &KThreesControlPlaneReconciler{
			Client:    failingClient,
			apiReader: k8sClient,
			ssaCache:  ssa.NewCache(),
		}

		err := reconciler.cloneConfigsAndGenerateMachine(
			ctx,
			fixture.cluster,
			fixture.kcp,
			&fixture.kcp.Spec.KThreesConfigSpec,
			"",
		)
		Expect(err).To(MatchError(ContainSubstring("failed to establish metadata ownership for " + relatedObjectTestMachineKind)))
		fixture.expectInfraMachineCount(ctx, 0)
	})

	It("cleans up both related objects when KThreesConfig ownership setup fails", func() {
		ctx := context.Background()
		fixture := newRelatedObjectOwnershipFixture(ctx)
		defer fixture.cleanup(ctx)
		failingClient := &relatedObjectSetupFailureClient{
			Client:                   k8sClient,
			failManagedFieldsPatchAt: 2,
		}
		reconciler := &KThreesControlPlaneReconciler{
			Client:    failingClient,
			apiReader: k8sClient,
			ssaCache:  ssa.NewCache(),
		}

		err := reconciler.cloneConfigsAndGenerateMachine(
			ctx,
			fixture.cluster,
			fixture.kcp,
			&fixture.kcp.Spec.KThreesConfigSpec,
			"",
		)
		Expect(err).To(MatchError(ContainSubstring("failed to split managedFields ownership for KThreesConfig")))
		fixture.expectInfraMachineCount(ctx, 0)
	})

	It("retains ownership setup and cleanup failures in the aggregate", func() {
		ctx := context.Background()
		fixture := newRelatedObjectOwnershipFixture(ctx)
		defer fixture.cleanup(ctx)
		failingClient := &relatedObjectSetupFailureClient{
			Client:                   k8sClient,
			failManagedFieldsPatchAt: 2,
			failDeleteKind:           relatedObjectTestMachineKind,
		}
		reconciler := &KThreesControlPlaneReconciler{
			Client:    failingClient,
			apiReader: k8sClient,
			ssaCache:  ssa.NewCache(),
		}

		err := reconciler.cloneConfigsAndGenerateMachine(
			ctx,
			fixture.cluster,
			fixture.kcp,
			&fixture.kcp.Spec.KThreesConfigSpec,
			"",
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to split managedFields ownership for KThreesConfig"))
		Expect(err.Error()).To(ContainSubstring("injected cleanup failure"))
		fixture.expectInfraMachineCount(ctx, 1)
	})

	It("requeues managedFields migration before CanUpdateMachine", func() {
		wasEnabled := feature.Gates.Enabled(feature.InPlaceUpdates)
		Expect(feature.MutableGates.Set("InPlaceUpdates=true")).To(Succeed())
		defer func() {
			Expect(feature.MutableGates.Set(fmt.Sprintf("InPlaceUpdates=%t", wasEnabled))).To(Succeed())
		}()
		ctx := context.Background()
		fixture, reconciler, trackingClient, cleanup := newManagedFieldsReconcileFixture(ctx, "requeue")
		defer cleanup()
		fixture.controlPlane.KCP.Spec.Replicas = ptr.To[int32](1)

		canUpdateCalled := false
		reconciler.overrides = &reconcilerOverrides{
			canUpdateMachine: func(context.Context, *clusterv1.Machine, k3s.UpToDateResult) (bool, error) {
				canUpdateCalled = true
				return true, nil
			},
		}

		result, err := reconciler.reconcile(ctx, fixture.controlPlane.Cluster, fixture.controlPlane.KCP)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))
		Expect(canUpdateCalled).To(BeFalse())
		Expect(trackingClient.machinePatchCount).To(BeZero())
		Expect(trackingClient.machineCreateCount).To(BeZero())
	})

	It("requeues managedFields migration before a pending trigger write", func() {
		ctx := context.Background()
		fixture, reconciler, trackingClient, cleanup := newManagedFieldsReconcileFixture(ctx, "trigger")
		defer cleanup()
		machine := &clusterv1.Machine{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fixture.machine), machine)).To(Succeed())
		if machine.Annotations == nil {
			machine.Annotations = map[string]string{}
		}
		machine.Annotations[clusterv1.UpdateInProgressAnnotation] = ""
		Expect(k8sClient.Update(ctx, machine)).To(Succeed())

		triggerCalled := false
		reconciler.overrides = &reconcilerOverrides{
			triggerInPlaceUpdate: func(context.Context, *clusterv1.Machine, k3s.UpToDateResult) error {
				triggerCalled = true
				return nil
			},
		}

		result, err := reconciler.reconcile(ctx, fixture.controlPlane.Cluster, fixture.controlPlane.KCP)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))
		Expect(triggerCalled).To(BeFalse())
		Expect(trackingClient.machinePatchCount).To(BeZero())
		Expect(trackingClient.machineCreateCount).To(BeZero())
	})

	It("requeues managedFields migration before Machine creation", func() {
		ctx := context.Background()
		fixture, reconciler, trackingClient, cleanup := newManagedFieldsReconcileFixture(ctx, "scale-up")
		defer cleanup()
		fixture.controlPlane.KCP.Spec.Replicas = ptr.To[int32](2)

		scaleUpCalled := false
		reconciler.overrides = &reconcilerOverrides{
			scaleUpControlPlane: func(context.Context, *clusterv1.Cluster, *controlplanev1.KThreesControlPlane, *k3s.ControlPlane) (ctrl.Result, error) {
				scaleUpCalled = true
				return ctrl.Result{}, nil
			},
		}

		result, err := reconciler.reconcile(ctx, fixture.controlPlane.Cluster, fixture.controlPlane.KCP)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))
		Expect(scaleUpCalled).To(BeFalse())
		Expect(trackingClient.machinePatchCount).To(BeZero())
		Expect(trackingClient.machineCreateCount).To(BeZero())
	})
})

func newManagedFieldsReconcileFixture(
	ctx context.Context,
	suffix string,
) (*managedFieldsSyncFixture, *KThreesControlPlaneReconciler, *relatedObjectSyncTrackingClient, func()) {
	fixture := createManagedFieldsSyncFixture(ctx, suffix, true)
	template := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": relatedObjectTestGroup + "/" + relatedObjectTestVersion,
		"kind":       relatedObjectTestTemplateKind,
		"metadata": map[string]interface{}{
			"name":      "migration-template-" + suffix,
			"namespace": fixture.controlPlane.KCP.Namespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"value": "initial",
					"removable": map[string]interface{}{
						"nested": "remove",
					},
				},
			},
		},
	}}
	Expect(k8sClient.Create(ctx, template)).To(Succeed())
	fixture.controlPlane.KCP.Spec.MachineTemplate.InfrastructureRef = corev1.ObjectReference{
		APIVersion: template.GetAPIVersion(),
		Kind:       template.GetKind(),
		Name:       template.GetName(),
		Namespace:  template.GetNamespace(),
	}
	fixture.controlPlane.Cluster.TypeMeta = metav1.TypeMeta{APIVersion: clusterv1.GroupVersion.String(), Kind: "Cluster"}
	fixture.controlPlane.Cluster.UID = types.UID("cluster-" + suffix)
	fixture.controlPlane.Cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{Host: "192.0.2.10", Port: 6443}

	machine := &clusterv1.Machine{}
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fixture.machine), machine)).To(Succeed())
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}
	machine.Labels[clusterv1.MachineControlPlaneLabel] = ""
	machine.Labels[clusterv1.MachineControlPlaneNameLabel] = fixture.controlPlane.KCP.Name
	machine.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(
		fixture.controlPlane.KCP,
		controlplanev1.GroupVersion.WithKind("KThreesControlPlane"),
	)}
	machine.Spec.Version = "v1.31.0+k3s1"
	Expect(k8sClient.Update(ctx, machine)).To(Succeed())
	fixture.machine = machine

	trackingClient := &relatedObjectSyncTrackingClient{Client: k8sClient}
	reconciler := &KThreesControlPlaneReconciler{
		Client:                    trackingClient,
		apiReader:                 k8sClient,
		managementClusterUncached: &k3s.Management{Client: k8sClient},
		ssaCache:                  ssa.NewCache(),
	}
	return fixture, reconciler, trackingClient, func() {
		secrets := &corev1.SecretList{}
		Expect(k8sClient.List(ctx, secrets, client.InNamespace(fixture.controlPlane.Cluster.Namespace))).To(Succeed())
		for i := range secrets.Items {
			if strings.HasPrefix(secrets.Items[i].Name, fixture.controlPlane.Cluster.Name+"-") {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &secrets.Items[i]))).To(Succeed())
			}
		}
		fixture.cleanup(ctx)
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, template))).To(Succeed())
	}
}

type relatedObjectSetupFailureClient struct {
	client.Client
	failManagedFieldsPatchAt int
	managedFieldsPatchCount  int
	failApplyPatchAt         int
	applyPatchCount          int
	failDeleteKind           string
	deleteFailed             bool
}

func (c *relatedObjectSetupFailureClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if patch.Type() == types.ApplyPatchType {
		c.applyPatchCount++
		if c.applyPatchCount == c.failApplyPatchAt {
			return errors.New("injected apply patch failure")
		}
	}
	if patch.Type() == types.MergePatchType {
		c.managedFieldsPatchCount++
		if c.managedFieldsPatchCount == c.failManagedFieldsPatchAt {
			return errors.New("injected managedFields patch failure")
		}
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *relatedObjectSetupFailureClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if !c.deleteFailed && obj.GetObjectKind().GroupVersionKind().Kind == c.failDeleteKind {
		c.deleteFailed = true
		return errors.New("injected cleanup failure")
	}
	return c.Client.Delete(ctx, obj, opts...)
}

type relatedObjectOwnershipFixture struct {
	cluster  *clusterv1.Cluster
	kcp      *controlplanev1.KThreesControlPlane
	template *unstructured.Unstructured
}

func newRelatedObjectOwnershipFixture(ctx context.Context) *relatedObjectOwnershipFixture {
	suffix := string(uuid.NewUUID())
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ownership-cluster-" + suffix,
			Namespace: metav1.NamespaceDefault,
		},
	}
	kcp := &controlplanev1.KThreesControlPlane{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KThreesControlPlane",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ownership-kcp-" + suffix,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID("ownership-kcp-" + suffix),
		},
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Version: "v1.31.1+k3s1",
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				ObjectMeta: clusterv1beta1.ObjectMeta{
					Labels: map[string]string{
						"ownership.test.cluster-api.io/label": "desired",
					},
					Annotations: map[string]string{
						"ownership.test.cluster-api.io/annotation": "desired",
					},
				},
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: relatedObjectTestGroup + "/" + relatedObjectTestVersion,
					Kind:       relatedObjectTestTemplateKind,
					Name:       "ownership-template-" + suffix,
				},
			},
			KThreesConfigSpec: bootstrapv1.KThreesConfigSpec{
				Version:         "v1.31.1+k3s1",
				PostK3sCommands: []string{"echo removable"},
			},
		},
	}
	template := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": relatedObjectTestGroup + "/" + relatedObjectTestVersion,
		"kind":       relatedObjectTestTemplateKind,
		"metadata": map[string]interface{}{
			"name":      kcp.Spec.MachineTemplate.InfrastructureRef.Name,
			"namespace": kcp.Namespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"stable":       "kept",
					"removedLater": "remove-me",
					"settings": map[string]interface{}{
						"legacy": "remove-me",
						"stable": "kept",
					},
				},
			},
		},
	}}
	Expect(k8sClient.Create(ctx, template)).To(Succeed())

	return &relatedObjectOwnershipFixture{
		cluster:  cluster,
		kcp:      kcp,
		template: template,
	}
}

func (f *relatedObjectOwnershipFixture) getMachine(ctx context.Context) *clusterv1.Machine {
	machines := &clusterv1.MachineList{}
	Expect(k8sClient.List(
		ctx,
		machines,
		client.InNamespace(f.kcp.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: f.cluster.Name},
	)).To(Succeed())
	Expect(machines.Items).To(HaveLen(1))
	return &machines.Items[0]
}

func (f *relatedObjectOwnershipFixture) getInfraMachine(ctx context.Context, ref clusterv1.ContractVersionedObjectReference) *unstructured.Unstructured {
	infraMachine := &unstructured.Unstructured{}
	infraMachine.SetGroupVersionKind(schema.FromAPIVersionAndKind(
		relatedObjectTestGroup+"/"+relatedObjectTestVersion,
		ref.Kind,
	))
	Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: f.kcp.Namespace, Name: ref.Name}, infraMachine)).To(Succeed())
	return infraMachine
}

func (f *relatedObjectOwnershipFixture) getKThreesConfig(ctx context.Context, ref clusterv1.ContractVersionedObjectReference) *bootstrapv1.KThreesConfig {
	kthreesConfig := &bootstrapv1.KThreesConfig{}
	Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: f.kcp.Namespace, Name: ref.Name}, kthreesConfig)).To(Succeed())
	kthreesConfig.SetGroupVersionKind(bootstrapv1.GroupVersion.WithKind("KThreesConfig"))
	return kthreesConfig
}

func (f *relatedObjectOwnershipFixture) cleanup(ctx context.Context) {
	machines := &clusterv1.MachineList{}
	Expect(k8sClient.List(
		ctx,
		machines,
		client.InNamespace(f.kcp.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: f.cluster.Name},
	)).To(Succeed())
	for i := range machines.Items {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &machines.Items[i]))).To(Succeed())
	}

	infraMachines := &unstructured.UnstructuredList{}
	infraMachines.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   relatedObjectTestGroup,
		Version: relatedObjectTestVersion,
		Kind:    relatedObjectTestMachineKind + "List",
	})
	Expect(k8sClient.List(
		ctx,
		infraMachines,
		client.InNamespace(f.kcp.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: f.cluster.Name},
	)).To(Succeed())
	for i := range infraMachines.Items {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &infraMachines.Items[i]))).To(Succeed())
	}

	kthreesConfigs := &bootstrapv1.KThreesConfigList{}
	Expect(k8sClient.List(
		ctx,
		kthreesConfigs,
		client.InNamespace(f.kcp.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: f.cluster.Name},
	)).To(Succeed())
	for i := range kthreesConfigs.Items {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &kthreesConfigs.Items[i]))).To(Succeed())
	}

	Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, f.template))).To(Succeed())
}

func (f *relatedObjectOwnershipFixture) expectInfraMachineCount(ctx context.Context, infraMachineCount int) {
	machines := &clusterv1.MachineList{}
	Expect(k8sClient.List(
		ctx,
		machines,
		client.InNamespace(f.kcp.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: f.cluster.Name},
	)).To(Succeed())
	Expect(machines.Items).To(BeEmpty())

	infraMachines := &unstructured.UnstructuredList{}
	infraMachines.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   relatedObjectTestGroup,
		Version: relatedObjectTestVersion,
		Kind:    relatedObjectTestMachineKind + "List",
	})
	Expect(k8sClient.List(
		ctx,
		infraMachines,
		client.InNamespace(f.kcp.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: f.cluster.Name},
	)).To(Succeed())
	Expect(infraMachines.Items).To(HaveLen(infraMachineCount))

	kthreesConfigs := &bootstrapv1.KThreesConfigList{}
	Expect(k8sClient.List(
		ctx,
		kthreesConfigs,
		client.InNamespace(f.kcp.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: f.cluster.Name},
	)).To(Succeed())
	Expect(kthreesConfigs.Items).To(BeEmpty())
}

func ensureRelatedObjectContractLabels() {
	ctx := context.Background()
	for _, name := range []string{
		"ownershiptestmachines." + relatedObjectTestGroup,
		"ownershiptestmachinetemplates." + relatedObjectTestGroup,
		"testmachines." + relatedObjectTestGroup,
	} {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, crd)).To(Succeed())
		if crd.Labels[clusterv1.GroupVersion.String()] == relatedObjectTestVersion {
			continue
		}

		base := crd.DeepCopy()
		if crd.Labels == nil {
			crd.Labels = map[string]string{}
		}
		crd.Labels[clusterv1.GroupVersion.String()] = relatedObjectTestVersion
		Expect(k8sClient.Patch(ctx, crd, client.MergeFrom(base))).To(Succeed())
	}
}
