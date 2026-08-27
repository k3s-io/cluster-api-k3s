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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/util/ssa"
)

const (
	managedFieldsCurrentLabel      = "current.test.cluster-api.io/label"
	managedFieldsStaleLabel        = "stale.test.cluster-api.io/label"
	managedFieldsOtherManagerLabel = "other.test.cluster-api.io/label"
	managedFieldsCurrentAnnotation = "current.test.cluster-api.io/annotation"
	managedFieldsStaleAnnotation   = "stale.test.cluster-api.io/annotation"
)

var _ = Describe("related object managedFields migration", func() {
	BeforeEach(func() {
		ensureManagedFieldsTestCRD("machines.cluster.x-k8s.io", clusterv1.GroupVersion.Group, "Machine", "machines")
		ensureManagedFieldsTestCRD("testmachines.infrastructure.cluster.x-k8s.io", "infrastructure.cluster.x-k8s.io", "TestMachine", "testmachines")
		ensureKThreesConfigContractLabel()
	})

	It("migrates old main-manager Apply ownership before syncing related-object metadata", func() {
		ctx := context.Background()
		fixture := createManagedFieldsSyncFixture(ctx, "apply", true)
		defer fixture.cleanup(ctx)

		for _, object := range []client.Object{fixture.infraMachine, fixture.kthreesConfig} {
			oldMainEntry := findManagedField(object, kcpManagerName, metav1.ManagedFieldsOperationApply, "")
			Expect(oldMainEntry).NotTo(BeNil())
			Expect(managedFieldOwns(oldMainEntry, "f:metadata", "f:labels", "f:"+clusterv1.ClusterNameLabel)).To(BeTrue())
		}

		Expect(fixture.reconciler.syncMachines(ctx, fixture.controlPlane)).To(Succeed())

		infraMachine := getInfraMachine(ctx, fixture.infraMachine)
		kthreesConfig := getKThreesConfig(ctx, fixture.kthreesConfig)
		assertMigratedRelatedObject(infraMachine)
		assertMigratedRelatedObject(kthreesConfig)

		fixture.controlPlane.InfraResources[fixture.machine.Name] = infraMachine
		fixture.controlPlane.KthreesConfigs[fixture.machine.Name] = kthreesConfig
		infraResourceVersion := infraMachine.GetResourceVersion()
		configResourceVersion := kthreesConfig.GetResourceVersion()
		infraManagedFields := infraMachine.GetManagedFields()
		configManagedFields := kthreesConfig.GetManagedFields()

		Expect(fixture.reconciler.syncMachines(ctx, fixture.controlPlane)).To(Succeed())

		infraMachine = getInfraMachine(ctx, fixture.infraMachine)
		kthreesConfig = getKThreesConfig(ctx, fixture.kthreesConfig)
		Expect(infraMachine.GetResourceVersion()).To(Equal(infraResourceVersion))
		Expect(kthreesConfig.GetResourceVersion()).To(Equal(configResourceVersion))
		Expect(infraMachine.GetManagedFields()).To(Equal(infraManagedFields))
		Expect(kthreesConfig.GetManagedFields()).To(Equal(configManagedFields))
	})

	It("preserves the CAPI migration entries and is idempotent after the first patch", func() {
		ctx := context.Background()
		fixture := createManagedFieldsSyncFixture(ctx, "migration", true)
		defer fixture.cleanup(ctx)

		Expect(ssa.MigrateManagedFields(ctx, k8sClient, fixture.infraMachine, kcpManagerName, kcpMetadataManagerName)).To(Succeed())

		infraMachine := getInfraMachine(ctx, fixture.infraMachine)
		Expect(findManagedField(infraMachine, kcpManagerName, metav1.ManagedFieldsOperationApply, "")).To(BeNil())
		Expect(findManagedField(infraMachine, "manager", metav1.ManagedFieldsOperationUpdate, "")).To(BeNil())
		Expect(findManagedField(infraMachine, "manager", metav1.ManagedFieldsOperationUpdate, "status")).NotTo(BeNil())
		Expect(findManagedField(infraMachine, "other-manager", metav1.ManagedFieldsOperationApply, "")).NotTo(BeNil())

		metadataEntry := findManagedField(infraMachine, kcpMetadataManagerName, metav1.ManagedFieldsOperationApply, "")
		Expect(metadataEntry).NotTo(BeNil())
		Expect(metadataEntry.APIVersion).To(Equal("infrastructure.cluster.x-k8s.io/v1beta2"))
		Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:name")).To(BeTrue())
		Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:labels", "f:"+clusterv1.ClusterNameLabel)).To(BeTrue())
		Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:labels", "f:"+managedFieldsStaleLabel)).To(BeTrue())
		Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:annotations", "f:"+managedFieldsStaleAnnotation)).To(BeTrue())
		Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:labels", "f:"+managedFieldsOtherManagerLabel)).To(BeFalse())

		resourceVersion := infraMachine.GetResourceVersion()
		managedFields := infraMachine.GetManagedFields()
		Expect(ssa.MigrateManagedFields(ctx, k8sClient, infraMachine, kcpManagerName, kcpMetadataManagerName)).To(Succeed())
		infraMachine = getInfraMachine(ctx, infraMachine)
		Expect(infraMachine.GetResourceVersion()).To(Equal(resourceVersion))
		Expect(infraMachine.GetManagedFields()).To(Equal(managedFields))
	})

	It("keeps classic manager Update compatibility when syncing related-object metadata", func() {
		ctx := context.Background()
		fixture := createManagedFieldsSyncFixture(ctx, "classic", false)
		defer fixture.cleanup(ctx)

		Expect(fixture.reconciler.syncMachines(ctx, fixture.controlPlane)).To(Succeed())

		infraMachine := getInfraMachine(ctx, fixture.infraMachine)
		kthreesConfig := getKThreesConfig(ctx, fixture.kthreesConfig)
		assertClassicRelatedObjectMigrated(infraMachine)
		assertClassicRelatedObjectMigrated(kthreesConfig)
	})
})

type managedFieldsSyncFixture struct {
	reconciler    *KThreesControlPlaneReconciler
	controlPlane  *k3s.ControlPlane
	machine       *clusterv1.Machine
	infraMachine  *unstructured.Unstructured
	kthreesConfig *bootstrapv1.KThreesConfig
}

func createManagedFieldsSyncFixture(ctx context.Context, suffix string, oldApplyOwnership bool) *managedFieldsSyncFixture {
	clusterName := "cluster-" + suffix
	namespace := "default"
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
	}
	kcp := &controlplanev1.KThreesControlPlane{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KThreesControlPlane",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kcp-" + suffix,
			Namespace: namespace,
			UID:       types.UID("kcp-" + suffix + "-uid"),
		},
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Replicas: ptr.To[int32](1),
			Version:  "v1.31.1+k3s1",
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				ObjectMeta: clusterv1beta1.ObjectMeta{
					Labels: map[string]string{
						managedFieldsCurrentLabel: "current",
					},
					Annotations: map[string]string{
						managedFieldsCurrentAnnotation: "current",
					},
				},
			},
		},
	}

	infraGVK := schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta2",
		Kind:    "TestMachine",
	}
	infraMachine := &unstructured.Unstructured{}
	infraMachine.SetGroupVersionKind(infraGVK)
	infraMachine.SetName("infra-" + suffix)
	infraMachine.SetNamespace(namespace)
	infraMachine.SetLabels(oldRelatedObjectLabels(clusterName))
	infraMachine.SetAnnotations(oldRelatedObjectAnnotations())
	Expect(unstructured.SetNestedField(infraMachine.Object, "initial", "spec", "value")).To(Succeed())
	Expect(k8sClient.Create(ctx, infraMachine, client.FieldOwner("manager"))).To(Succeed())

	kthreesConfig := &bootstrapv1.KThreesConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: bootstrapv1.GroupVersion.String(),
			Kind:       "KThreesConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "config-" + suffix,
			Namespace:   namespace,
			Labels:      oldRelatedObjectLabels(clusterName),
			Annotations: oldRelatedObjectAnnotations(),
		},
		Spec: bootstrapv1.KThreesConfigSpec{
			Version: "v1.31.1+k3s1",
		},
	}
	Expect(k8sClient.Create(ctx, kthreesConfig, client.FieldOwner("manager"))).To(Succeed())

	if oldApplyOwnership {
		applyRelatedObjectMetadata(ctx, infraMachine, infraGVK, kcpManagerName, oldRelatedObjectLabels(clusterName), oldRelatedObjectAnnotations())
		applyRelatedObjectMetadata(ctx, kthreesConfig, bootstrapv1.GroupVersion.WithKind("KThreesConfig"), kcpManagerName, oldRelatedObjectLabels(clusterName), oldRelatedObjectAnnotations())
	}

	applyRelatedObjectMetadata(ctx, infraMachine, infraGVK, "other-manager", map[string]string{
		managedFieldsOtherManagerLabel: "preserved",
	}, nil)
	applyRelatedObjectMetadata(ctx, kthreesConfig, bootstrapv1.GroupVersion.WithKind("KThreesConfig"), "other-manager", map[string]string{
		managedFieldsOtherManagerLabel: "preserved",
	}, nil)

	infraMachine = getInfraMachine(ctx, infraMachine)
	infraBeforeStatus := infraMachine.DeepCopy()
	Expect(unstructured.SetNestedField(infraMachine.Object, "ready", "status", "phase")).To(Succeed())
	Expect(k8sClient.Status().Patch(ctx, infraMachine, client.MergeFrom(infraBeforeStatus), client.FieldOwner("manager"))).To(Succeed())

	kthreesConfig = getKThreesConfig(ctx, kthreesConfig)
	configBeforeStatus := kthreesConfig.DeepCopy()
	kthreesConfig.Status.Ready = true
	Expect(k8sClient.Status().Patch(ctx, kthreesConfig, client.MergeFrom(configBeforeStatus), client.FieldOwner("manager"))).To(Succeed())

	machine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-" + suffix,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: clusterName,
			Version:     kcp.Spec.Version,
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infraGVK.Group,
				Kind:     infraGVK.Kind,
				Name:     infraMachine.GetName(),
			},
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: bootstrapv1.GroupVersion.Group,
					Kind:     "KThreesConfig",
					Name:     kthreesConfig.Name,
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, machine, client.FieldOwner("manager"))).To(Succeed())

	infraMachine = getInfraMachine(ctx, infraMachine)
	kthreesConfig = getKThreesConfig(ctx, kthreesConfig)
	return &managedFieldsSyncFixture{
		reconciler: &KThreesControlPlaneReconciler{
			Client:   k8sClient,
			ssaCache: ssa.NewCache(),
		},
		controlPlane: &k3s.ControlPlane{
			KCP:            kcp,
			Cluster:        cluster,
			Machines:       collections.FromMachines(machine),
			InfraResources: map[string]*unstructured.Unstructured{machine.Name: infraMachine},
			KthreesConfigs: map[string]*bootstrapv1.KThreesConfig{machine.Name: kthreesConfig},
		},
		machine:       machine,
		infraMachine:  infraMachine,
		kthreesConfig: kthreesConfig,
	}
}

func (f *managedFieldsSyncFixture) cleanup(ctx context.Context) {
	for _, object := range []client.Object{f.machine, f.infraMachine, f.kthreesConfig} {
		if err := k8sClient.Delete(ctx, object); err != nil {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	}
}

func oldRelatedObjectLabels(clusterName string) map[string]string {
	return map[string]string{
		clusterv1.ClusterNameLabel:             clusterName,
		clusterv1.MachineControlPlaneLabel:     "",
		clusterv1.MachineControlPlaneNameLabel: "",
		managedFieldsCurrentLabel:              "current",
		managedFieldsStaleLabel:                "stale",
	}
}

func oldRelatedObjectAnnotations() map[string]string {
	return map[string]string{
		managedFieldsCurrentAnnotation: "current",
		managedFieldsStaleAnnotation:   "stale",
	}
}

func applyRelatedObjectMetadata(ctx context.Context, object client.Object, gvk schema.GroupVersionKind, manager string, labels, annotations map[string]string) {
	intent := &unstructured.Unstructured{}
	intent.SetGroupVersionKind(gvk)
	intent.SetNamespace(object.GetNamespace())
	intent.SetName(object.GetName())
	intent.SetUID(object.GetUID())
	intent.SetLabels(labels)
	intent.SetAnnotations(annotations)
	Expect(ssa.Patch(ctx, k8sClient, manager, intent)).To(Succeed())
}

func assertMigratedRelatedObject(object client.Object) {
	Expect(findManagedField(object, kcpManagerName, metav1.ManagedFieldsOperationApply, "")).To(BeNil())
	metadataEntry := findManagedField(object, kcpMetadataManagerName, metav1.ManagedFieldsOperationApply, "")
	Expect(metadataEntry).NotTo(BeNil())
	Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:labels", "f:"+managedFieldsCurrentLabel)).To(BeTrue())
	Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:annotations", "f:"+managedFieldsCurrentAnnotation)).To(BeTrue())
	Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:labels", "f:"+managedFieldsStaleLabel)).To(BeFalse())
	Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:annotations", "f:"+managedFieldsStaleAnnotation)).To(BeFalse())

	otherManagerEntry := findManagedField(object, "other-manager", metav1.ManagedFieldsOperationApply, "")
	Expect(otherManagerEntry).NotTo(BeNil())
	Expect(managedFieldOwns(otherManagerEntry, "f:metadata", "f:labels", "f:"+managedFieldsOtherManagerLabel)).To(BeTrue())
	Expect(findManagedField(object, "manager", metav1.ManagedFieldsOperationUpdate, "status")).NotTo(BeNil())

	Expect(object.GetLabels()).To(HaveKeyWithValue(managedFieldsCurrentLabel, "current"))
	Expect(object.GetAnnotations()).To(HaveKeyWithValue(managedFieldsCurrentAnnotation, "current"))
	Expect(object.GetLabels()).NotTo(HaveKey(managedFieldsStaleLabel))
	Expect(object.GetAnnotations()).NotTo(HaveKey(managedFieldsStaleAnnotation))
	Expect(object.GetLabels()).To(HaveKeyWithValue(managedFieldsOtherManagerLabel, "preserved"))
}

func assertClassicRelatedObjectMigrated(object client.Object) {
	Expect(object.GetLabels()).To(HaveKeyWithValue(managedFieldsCurrentLabel, "current"))
	Expect(object.GetAnnotations()).To(HaveKeyWithValue(managedFieldsCurrentAnnotation, "current"))
	Expect(object.GetLabels()).To(HaveKeyWithValue(managedFieldsOtherManagerLabel, "preserved"))

	Expect(findManagedField(object, kcpManagerName, metav1.ManagedFieldsOperationApply, "")).To(BeNil())
	metadataEntry := findManagedField(object, kcpMetadataManagerName, metav1.ManagedFieldsOperationApply, "")
	Expect(metadataEntry).NotTo(BeNil())
	Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:labels", "f:"+managedFieldsCurrentLabel)).To(BeTrue())
	Expect(managedFieldOwns(metadataEntry, "f:metadata", "f:annotations", "f:"+managedFieldsCurrentAnnotation)).To(BeTrue())

	otherManagerEntry := findManagedField(object, "other-manager", metav1.ManagedFieldsOperationApply, "")
	Expect(otherManagerEntry).NotTo(BeNil())
	Expect(managedFieldOwns(otherManagerEntry, "f:metadata", "f:labels", "f:"+managedFieldsOtherManagerLabel)).To(BeTrue())
	Expect(findManagedField(object, "manager", metav1.ManagedFieldsOperationUpdate, "status")).NotTo(BeNil())

	classicEntry := findManagedField(object, "manager", metav1.ManagedFieldsOperationUpdate, "")
	Expect(classicEntry).NotTo(BeNil())
	Expect(managedFieldOwns(classicEntry, "f:metadata", "f:labels")).To(BeFalse())
	Expect(managedFieldOwns(classicEntry, "f:metadata", "f:annotations")).To(BeFalse())
}

func findManagedField(object client.Object, manager string, operation metav1.ManagedFieldsOperationType, subresource string) *metav1.ManagedFieldsEntry {
	managedFields := object.GetManagedFields()
	for i := range managedFields {
		entry := &managedFields[i]
		if entry.Manager == manager && entry.Operation == operation && entry.Subresource == subresource {
			return entry
		}
	}
	return nil
}

func managedFieldOwns(entry *metav1.ManagedFieldsEntry, path ...string) bool {
	fields := map[string]interface{}{}
	Expect(json.Unmarshal(entry.FieldsV1.Raw, &fields)).To(Succeed())
	_, found, err := unstructured.NestedFieldNoCopy(fields, path...)
	Expect(err).NotTo(HaveOccurred())
	return found
}

func getInfraMachine(ctx context.Context, object client.Object) *unstructured.Unstructured {
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(object.GetObjectKind().GroupVersionKind())
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(object), current)).To(Succeed())
	return current
}

func getKThreesConfig(ctx context.Context, object client.Object) *bootstrapv1.KThreesConfig {
	current := &bootstrapv1.KThreesConfig{}
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(object), current)).To(Succeed())
	current.SetGroupVersionKind(bootstrapv1.GroupVersion.WithKind("KThreesConfig"))
	return current
}

func ensureManagedFieldsTestCRD(name, group, kind, plural string) {
	ctx := context.Background()
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: plural,
				Kind:   kind,
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1beta2",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: ptr.To(true),
					},
				},
				Subresources: &apiextensionsv1.CustomResourceSubresources{
					Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
				},
			}},
		},
	}
	err := k8sClient.Create(ctx, crd)
	if err != nil {
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
	}
	Eventually(func() bool {
		current := &apiextensionsv1.CustomResourceDefinition{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, current); err != nil {
			return false
		}
		for _, condition := range current.Status.Conditions {
			if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
				return true
			}
		}
		return false
	}).Should(BeTrue())
}

func ensureKThreesConfigContractLabel() {
	ctx := context.Background()
	crd := &apiextensionsv1.CustomResourceDefinition{}
	key := client.ObjectKey{Name: "kthreesconfigs.bootstrap.cluster.x-k8s.io"}
	Expect(k8sClient.Get(ctx, key, crd)).To(Succeed())
	if crd.Labels[clusterv1.GroupVersion.String()] == bootstrapv1.GroupVersion.Version {
		return
	}
	base := crd.DeepCopy()
	if crd.Labels == nil {
		crd.Labels = map[string]string{}
	}
	crd.Labels[clusterv1.GroupVersion.String()] = bootstrapv1.GroupVersion.Version
	Expect(k8sClient.Patch(ctx, crd, client.MergeFrom(base))).To(Succeed())
}
