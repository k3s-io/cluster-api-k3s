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

package ssa

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type managedFieldsPatchClient struct {
	client.Client
	patchAttempts                  int
	conflictsRemaining             int
	resourceVersions               []string
	optimisticLockResourceVersions []string
	successResourceVersion         string
}

func (c *managedFieldsPatchClient) Patch(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
	c.patchAttempts++
	c.resourceVersions = append(c.resourceVersions, obj.GetResourceVersion())

	data, err := patch.Data(obj)
	if err != nil {
		return err
	}
	patchMap := map[string]interface{}{}
	if err := json.Unmarshal(data, &patchMap); err != nil {
		return err
	}
	resourceVersion, found, err := unstructured.NestedString(patchMap, "metadata", "resourceVersion")
	if err != nil {
		return err
	}
	if found {
		c.optimisticLockResourceVersions = append(c.optimisticLockResourceVersions, resourceVersion)
	}

	if c.conflictsRemaining > 0 {
		c.conflictsRemaining--
		return apierrors.NewConflict(
			schema.GroupResource{Group: "infrastructure.cluster.x-k8s.io", Resource: "testmachines"},
			obj.GetName(),
			errors.New("injected conflict"),
		)
	}

	resourceVersion = c.successResourceVersion
	if resourceVersion == "" {
		resourceVersion = "2"
	}
	obj.SetResourceVersion(resourceVersion)
	return nil
}

type configMapReader struct {
	client.Reader
	object *corev1.ConfigMap
	gets   int
}

func (r *configMapReader) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	r.gets++
	if key != client.ObjectKeyFromObject(r.object) {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, key.Name)
	}
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return errors.New("expected ConfigMap")
	}
	*configMap = *r.object.DeepCopy()
	return nil
}

func TestRemoveManagedFieldsForLabelsAndAnnotations(t *testing.T) {
	mainApply := metav1.ManagedFieldsEntry{
		Manager:    "capi-kthreescontrolplane",
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
		FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{
			"f:metadata":{
				"f:annotations":{"f:old.example.io/value":{}},
				"f:finalizers":{"v:\"test.finalizer\"":{}},
				"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}},
				"f:ownerReferences":{"k:{\"uid\":\"machine-uid\"}":{".":{},"f:uid":{}}}
			},
			"f:spec":{"f:oldSetting":{}}
		}`)},
	}
	statusApply := metav1.ManagedFieldsEntry{
		Manager:     "capi-kthreescontrolplane",
		Operation:   metav1.ManagedFieldsOperationApply,
		APIVersion:  "infrastructure.cluster.x-k8s.io/v1beta1",
		FieldsType:  "FieldsV1",
		Subresource: "status",
		FieldsV1:    &metav1.FieldsV1{Raw: []byte(`{"f:status":{"f:ready":{}}}`)},
	}
	mainUpdate := metav1.ManagedFieldsEntry{
		Manager:    "capi-kthreescontrolplane",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:labels":{"f:update-owned":{}}}}`)},
	}
	otherApply := metav1.ManagedFieldsEntry{
		Manager:    "other-manager",
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:labels":{"f:other":{}}}}`)},
	}

	tests := []struct {
		name              string
		mainApply         metav1.ManagedFieldsEntry
		wantMainApply     bool
		wantPatchAttempts int
	}{
		{
			name:              "removes only labels and annotations",
			mainApply:         mainApply,
			wantMainApply:     true,
			wantPatchAttempts: 1,
		},
		{
			name: "removes an empty main entry",
			mainApply: metav1.ManagedFieldsEntry{
				Manager:    mainApply.Manager,
				Operation:  mainApply.Operation,
				APIVersion: mainApply.APIVersion,
				FieldsType: mainApply.FieldsType,
				FieldsV1: &metav1.FieldsV1{Raw: []byte(`{
					"f:metadata":{
						"f:annotations":{"f:old.example.io/value":{}},
						"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}
					}
				}`)},
			},
			wantPatchAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

			obj := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "related-object",
					Namespace:       metav1.NamespaceDefault,
					ResourceVersion: "1",
					ManagedFields:   []metav1.ManagedFieldsEntry{tt.mainApply, statusApply, mainUpdate, otherApply},
				},
			}
			c := &managedFieldsPatchClient{
				Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
			}

			g.Expect(RemoveManagedFieldsForLabelsAndAnnotations(
				context.Background(),
				c,
				c,
				obj,
				mainApply.Manager,
			)).To(Succeed())
			g.Expect(c.patchAttempts).To(Equal(tt.wantPatchAttempts))

			var gotMainApply *metav1.ManagedFieldsEntry
			gotManagedFields := obj.GetManagedFields()
			for i := range gotManagedFields {
				entry := &gotManagedFields[i]
				if entry.Manager == mainApply.Manager &&
					entry.Operation == metav1.ManagedFieldsOperationApply &&
					entry.Subresource == "" {
					gotMainApply = entry
				}
			}

			if !tt.wantMainApply {
				g.Expect(gotMainApply).To(BeNil())
			} else {
				g.Expect(gotMainApply).NotTo(BeNil())
				g.Expect(gotMainApply.APIVersion).To(Equal(mainApply.APIVersion))

				fields := map[string]interface{}{}
				g.Expect(json.Unmarshal(gotMainApply.FieldsV1.Raw, &fields)).To(Succeed())
				_, found, err := unstructured.NestedFieldNoCopy(fields, "f:spec", "f:oldSetting")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				_, found, err = unstructured.NestedFieldNoCopy(fields, "f:metadata", "f:ownerReferences")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				_, found, err = unstructured.NestedFieldNoCopy(fields, "f:metadata", "f:finalizers")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				_, found, err = unstructured.NestedFieldNoCopy(fields, "f:metadata", "f:labels")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeFalse())
				_, found, err = unstructured.NestedFieldNoCopy(fields, "f:metadata", "f:annotations")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeFalse())
			}

			g.Expect(obj.GetManagedFields()).To(ContainElement(statusApply))
			g.Expect(obj.GetManagedFields()).To(ContainElement(mainUpdate))
			g.Expect(obj.GetManagedFields()).To(ContainElement(otherApply))
		})
	}
}

func TestRemoveManagedFieldsForLabelsAndAnnotationsSkipsUnchangedObject(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	mainApply := metav1.ManagedFieldsEntry{
		Manager:    "capi-kthreescontrolplane",
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:oldSetting":{}}}`)},
	}
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "related-object",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
			ManagedFields:   []metav1.ManagedFieldsEntry{mainApply},
		},
	}
	c := &managedFieldsPatchClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	g.Expect(RemoveManagedFieldsForLabelsAndAnnotations(
		context.Background(),
		c,
		c,
		obj,
		mainApply.Manager,
	)).To(Succeed())
	g.Expect(c.patchAttempts).To(BeZero())
	g.Expect(obj.GetManagedFields()).To(Equal([]metav1.ManagedFieldsEntry{mainApply}))
}

func TestRemoveManagedFieldsForLabelsAndAnnotationsRetriesConflict(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	mainApply := metav1.ManagedFieldsEntry{
		Manager:    "capi-kthreescontrolplane",
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
		FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{
			"f:metadata":{
				"f:annotations":{"f:old.example.io/value":{}},
				"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}},
				"f:ownerReferences":{"k:{\"uid\":\"machine-uid\"}":{".":{},"f:uid":{}}}
			},
			"f:spec":{"f:oldSetting":{}}
		}`)},
	}
	concurrentEntry := metav1.ManagedFieldsEntry{
		Manager:    "concurrent-manager",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: "v1",
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:data":{"f:concurrent":{}}}`)},
	}
	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "related-object",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
			ManagedFields:   []metav1.ManagedFieldsEntry{mainApply},
			OwnerReferences: []metav1.OwnerReference{{UID: "machine-uid"}},
			Finalizers:      []string{"test.finalizer"},
			Labels:          map[string]string{"preserved-label": "value"},
			Annotations:     map[string]string{"preserved-annotation": "value"},
		},
		Data: map[string]string{"preserved": "value"},
	}
	newer := obj.DeepCopy()
	newer.ResourceVersion = "2"
	newer.ManagedFields = append(newer.ManagedFields, concurrentEntry)
	newer.Data["concurrent"] = "value"

	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &managedFieldsPatchClient{
		Client:                 baseClient,
		conflictsRemaining:     1,
		successResourceVersion: "3",
	}
	apiReader := &configMapReader{
		Reader: baseClient,
		object: newer,
	}

	g.Expect(RemoveManagedFieldsForLabelsAndAnnotations(
		context.Background(),
		c,
		apiReader,
		obj,
		mainApply.Manager,
	)).To(Succeed())

	g.Expect(c.patchAttempts).To(Equal(2))
	g.Expect(c.patchAttempts).To(BeNumerically("<=", retry.DefaultRetry.Steps))
	g.Expect(c.resourceVersions).To(Equal([]string{"1", "2"}))
	g.Expect(c.optimisticLockResourceVersions).To(Equal([]string{"1", "2"}))
	g.Expect(apiReader.gets).To(Equal(1))
	g.Expect(obj.GetResourceVersion()).To(Equal("3"))
	g.Expect(obj.GetManagedFields()).To(ContainElement(concurrentEntry))
	g.Expect(obj.GetOwnerReferences()).To(Equal(newer.GetOwnerReferences()))
	g.Expect(obj.GetFinalizers()).To(Equal(newer.GetFinalizers()))
	g.Expect(obj.GetLabels()).To(Equal(newer.GetLabels()))
	g.Expect(obj.GetAnnotations()).To(Equal(newer.GetAnnotations()))
	g.Expect(obj.Data).To(Equal(newer.Data))
}

func TestRemoveManagedFieldsForLabelsAndAnnotationsLimitsConflictRetries(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "related-object",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
			ManagedFields: []metav1.ManagedFieldsEntry{{
				Manager:    "capi-kthreescontrolplane",
				Operation:  metav1.ManagedFieldsOperationApply,
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				FieldsType: "FieldsV1",
				FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:labels":{"f:old":{}}}}`)},
			}},
		},
	}
	newer := obj.DeepCopy()
	newer.ResourceVersion = "2"

	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &managedFieldsPatchClient{
		Client:             baseClient,
		conflictsRemaining: retry.DefaultRetry.Steps + 1,
	}
	apiReader := &configMapReader{
		Reader: baseClient,
		object: newer,
	}

	err := RemoveManagedFieldsForLabelsAndAnnotations(
		context.Background(),
		c,
		apiReader,
		obj,
		"capi-kthreescontrolplane",
	)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsConflict(err)).To(BeTrue())
	g.Expect(c.patchAttempts).To(Equal(retry.DefaultRetry.Steps))
}

func TestRemoveManagedFieldsForLabelsAndAnnotationsRejectsNilFieldsV1(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-object",
			Namespace: metav1.NamespaceDefault,
			ManagedFields: []metav1.ManagedFieldsEntry{{
				Manager:    "capi-kthreescontrolplane",
				Operation:  metav1.ManagedFieldsOperationApply,
				APIVersion: "v1",
				FieldsType: "FieldsV1",
			}},
		},
	}
	c := &managedFieldsPatchClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	err := RemoveManagedFieldsForLabelsAndAnnotations(
		context.Background(),
		c,
		c,
		obj,
		"capi-kthreescontrolplane",
	)
	g.Expect(err).To(MatchError(ContainSubstring("nil FieldsV1")))
	g.Expect(c.patchAttempts).To(BeZero())
}

func TestMigrateManagedFieldsHistoricalLayouts(t *testing.T) {
	mainManager := "capi-kthreescontrolplane"
	metadataManager := "capi-kthreescontrolplane-metadata"
	apiVersion := "infrastructure.cluster.x-k8s.io/v1beta1"
	mainTime := metav1.Now()
	metadataTime := metav1.Now()
	mainStatus := managedFieldsEntry(mainManager, metav1.ManagedFieldsOperationApply, apiVersion, `{"f:status":{"f:ready":{}}}`)
	mainStatus.Subresource = "status"
	classicStatus := managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersion, `{"f:status":{"f:ready":{}}}`)
	classicStatus.Subresource = "status"
	unrelatedApply := managedFieldsEntry("provider-defaults", metav1.ManagedFieldsOperationApply, apiVersion, `{"f:spec":{"f:providerDefault":{}}}`)

	tests := []struct {
		name                 string
		managedFields        []metav1.ManagedFieldsEntry
		wantOutcome          ManagedFieldsMigrationOutcome
		wantSpecAPIVersion   string
		wantSpecPaths        [][]string
		wantClassicPath      []string
		wantStatusEntries    []metav1.ManagedFieldsEntry
		wantUnrelatedEntries []metav1.ManagedFieldsEntry
		wantMainTime         *metav1.Time
		wantMetadataTime     *metav1.Time
	}{
		{
			name: "classic Update owns spec metadata and unrelated metadata",
			managedFields: []metav1.ManagedFieldsEntry{
				managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersion, `{
					"f:metadata":{
						"f:annotations":{"f:stale.example.io/value":{}},
						"f:finalizers":{"v:\"example.io/finalizer\"":{}},
						"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}
					},
					"f:spec":{"f:diskSize":{}}
				}`),
			},
			wantOutcome:        ManagedFieldsMigrationCompleted,
			wantSpecAPIVersion: apiVersion,
			wantSpecPaths:      [][]string{{"f:spec", "f:diskSize"}},
			wantClassicPath:    []string{"f:metadata", "f:finalizers"},
		},
		{
			name: "main Apply owns metadata and classic Update owns spec",
			managedFields: []metav1.ManagedFieldsEntry{
				managedFieldsEntry(mainManager, metav1.ManagedFieldsOperationApply, "infrastructure.cluster.x-k8s.io/v1beta2", `{
					"f:metadata":{
						"f:annotations":{"f:stale.example.io/value":{}},
						"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}
					}
				}`),
				managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersion, `{"f:spec":{"f:diskSize":{}}}`),
			},
			wantOutcome:        ManagedFieldsMigrationCompleted,
			wantSpecAPIVersion: apiVersion,
			wantSpecPaths:      [][]string{{"f:spec", "f:diskSize"}},
		},
		{
			name: "main Apply already owns spec and metadata manager owns metadata",
			managedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    mainManager,
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: apiVersion,
					Time:       &mainTime,
					FieldsType: "FieldsV1",
					FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:diskSize":{}}}`)},
				},
				{
					Manager:    metadataManager,
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: apiVersion,
					Time:       &metadataTime,
					FieldsType: "FieldsV1",
					FieldsV1: &metav1.FieldsV1{Raw: []byte(`{
						"f:metadata":{
							"f:annotations":{"f:stale.example.io/value":{}},
							"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}
						}
					}`)},
				},
			},
			wantOutcome:        ManagedFieldsMigrationUnchanged,
			wantSpecAPIVersion: apiVersion,
			wantSpecPaths:      [][]string{{"f:spec", "f:diskSize"}},
			wantMainTime:       &mainTime,
			wantMetadataTime:   &metadataTime,
		},
		{
			name: "partially migrated main spec with stale classic metadata",
			managedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    mainManager,
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: apiVersion,
					Time:       &mainTime,
					FieldsType: "FieldsV1",
					FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:diskSize":{}}}`)},
				},
				{
					Manager:    metadataManager,
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: apiVersion,
					Time:       &metadataTime,
					FieldsType: "FieldsV1",
					FieldsV1: &metav1.FieldsV1{Raw: []byte(`{
						"f:metadata":{"f:labels":{"f:current.example.io/value":{}}}
					}`)},
				},
				managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersion, `{
					"f:metadata":{
						"f:annotations":{"f:stale.example.io/value":{}},
						"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}
					}
				}`),
			},
			wantOutcome:        ManagedFieldsMigrationCompleted,
			wantSpecAPIVersion: apiVersion,
			wantSpecPaths:      [][]string{{"f:spec", "f:diskSize"}},
			wantMainTime:       &mainTime,
			wantMetadataTime:   &metadataTime,
		},
		{
			name: "status entries remain unchanged",
			managedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    mainManager,
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: apiVersion,
					Time:       &mainTime,
					FieldsType: "FieldsV1",
					FieldsV1: &metav1.FieldsV1{Raw: []byte(`{
						"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}},
						"f:spec":{"f:existing":{}}
					}`)},
				},
				managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersion, `{
					"f:metadata":{"f:annotations":{"f:stale.example.io/value":{}}},
					"f:spec":{"f:diskSize":{}}
				}`),
				mainStatus,
				classicStatus,
			},
			wantOutcome:        ManagedFieldsMigrationCompleted,
			wantSpecAPIVersion: apiVersion,
			wantSpecPaths:      [][]string{{"f:spec", "f:existing"}, {"f:spec", "f:diskSize"}},
			wantStatusEntries:  []metav1.ManagedFieldsEntry{mainStatus, classicStatus},
			wantMainTime:       &mainTime,
		},
		{
			name: "unrelated Apply manager keeps provider defaulted spec",
			managedFields: []metav1.ManagedFieldsEntry{
				managedFieldsEntry(mainManager, metav1.ManagedFieldsOperationApply, apiVersion, `{
					"f:metadata":{
						"f:annotations":{"f:stale.example.io/value":{}},
						"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}
					}
				}`),
				managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersion, `{"f:spec":{"f:diskSize":{}}}`),
				unrelatedApply,
			},
			wantOutcome:          ManagedFieldsMigrationCompleted,
			wantSpecAPIVersion:   apiVersion,
			wantSpecPaths:        [][]string{{"f:spec", "f:diskSize"}},
			wantUnrelatedEntries: []metav1.ManagedFieldsEntry{unrelatedApply},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			obj := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "related-object",
					Namespace:       metav1.NamespaceDefault,
					ResourceVersion: "1",
					ManagedFields:   tt.managedFields,
				},
			}
			c := &managedFieldsPatchClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

			result, err := MigrateManagedFields(context.Background(), c, c, obj, mainManager, metadataManager)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).To(Equal(ManagedFieldsMigrationResult{Outcome: tt.wantOutcome}))
			if tt.wantOutcome == ManagedFieldsMigrationCompleted {
				g.Expect(c.patchAttempts).To(Equal(1))
			} else {
				g.Expect(c.patchAttempts).To(BeZero())
			}

			mainEntry := findManagedFieldsEntry(obj, mainManager, metav1.ManagedFieldsOperationApply, "")
			g.Expect(mainEntry).NotTo(BeNil())
			g.Expect(mainEntry.APIVersion).To(Equal(tt.wantSpecAPIVersion))
			for _, path := range tt.wantSpecPaths {
				g.Expect(managedFieldsEntryOwns(t, mainEntry, path...)).To(BeTrue())
			}
			g.Expect(managedFieldsEntryOwns(t, mainEntry, "f:metadata", "f:labels")).To(BeFalse())
			g.Expect(managedFieldsEntryOwns(t, mainEntry, "f:metadata", "f:annotations")).To(BeFalse())
			if tt.wantMainTime != nil {
				g.Expect(mainEntry.Time).To(Equal(tt.wantMainTime))
			}

			metadataEntry := findManagedFieldsEntry(obj, metadataManager, metav1.ManagedFieldsOperationApply, "")
			g.Expect(metadataEntry).NotTo(BeNil())
			g.Expect(managedFieldsEntryOwns(t, metadataEntry, "f:metadata", "f:labels", "f:cluster.x-k8s.io/cluster-name")).To(BeTrue())
			g.Expect(managedFieldsEntryOwns(t, metadataEntry, "f:metadata", "f:annotations", "f:stale.example.io/value")).To(BeTrue())
			if tt.wantMetadataTime != nil {
				g.Expect(metadataEntry.Time).To(Equal(tt.wantMetadataTime))
			}

			classicEntry := findManagedFieldsEntry(obj, classicManager, metav1.ManagedFieldsOperationUpdate, "")
			if tt.wantClassicPath == nil {
				g.Expect(classicEntry).To(BeNil())
			} else {
				g.Expect(classicEntry).NotTo(BeNil())
				g.Expect(managedFieldsEntryOwns(t, classicEntry, tt.wantClassicPath...)).To(BeTrue())
				g.Expect(managedFieldsEntryOwns(t, classicEntry, "f:spec")).To(BeFalse())
				g.Expect(managedFieldsEntryOwns(t, classicEntry, "f:metadata", "f:labels")).To(BeFalse())
				g.Expect(managedFieldsEntryOwns(t, classicEntry, "f:metadata", "f:annotations")).To(BeFalse())
			}
			for _, entry := range tt.wantStatusEntries {
				g.Expect(obj.GetManagedFields()).To(ContainElement(entry))
			}
			for _, entry := range tt.wantUnrelatedEntries {
				g.Expect(obj.GetManagedFields()).To(ContainElement(entry))
			}

			resourceVersion := obj.GetResourceVersion()
			managedFields := obj.GetManagedFields()
			patchAttempts := c.patchAttempts
			result, err = MigrateManagedFields(context.Background(), c, c, obj, mainManager, metadataManager)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).To(Equal(ManagedFieldsMigrationResult{Outcome: ManagedFieldsMigrationUnchanged}))
			g.Expect(c.patchAttempts).To(Equal(patchAttempts))
			g.Expect(obj.GetResourceVersion()).To(Equal(resourceVersion))
			g.Expect(obj.GetManagedFields()).To(Equal(managedFields))
		})
	}
}

func TestMigrateManagedFieldsPreservesSpecEntryTimestampWhenEarlierMainEntryBecomesEmpty(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	mainManager := "capi-kthreescontrolplane"
	metadataManager := "capi-kthreescontrolplane-metadata"
	apiVersion := "infrastructure.cluster.x-k8s.io/v1beta1"
	metadataOnlyTime := metav1.NewTime(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
	specTime := metav1.NewTime(time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC))
	metadataOnlyEntry := managedFieldsEntry(
		mainManager,
		metav1.ManagedFieldsOperationApply,
		apiVersion,
		`{"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}}}`,
	)
	metadataOnlyEntry.Time = &metadataOnlyTime
	specEntry := managedFieldsEntry(
		mainManager,
		metav1.ManagedFieldsOperationApply,
		apiVersion,
		`{"f:spec":{"f:diskSize":{}}}`,
	)
	specEntry.Time = &specTime
	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "related-object",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
			ManagedFields:   []metav1.ManagedFieldsEntry{metadataOnlyEntry, specEntry},
		},
	}
	c := &managedFieldsPatchClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	result, err := MigrateManagedFields(context.Background(), c, c, obj, mainManager, metadataManager)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(ManagedFieldsMigrationResult{Outcome: ManagedFieldsMigrationCompleted}))

	mainEntry := findManagedFieldsEntry(obj, mainManager, metav1.ManagedFieldsOperationApply, "")
	g.Expect(mainEntry).NotTo(BeNil())
	g.Expect(mainEntry.Time).To(Equal(&specTime))
	g.Expect(managedFieldsEntryOwns(t, mainEntry, "f:spec", "f:diskSize")).To(BeTrue())
}

func TestMigrateManagedFieldsRejectsMultipleSpecAPIVersions(t *testing.T) {
	tests := []struct {
		name          string
		managedFields []metav1.ManagedFieldsEntry
	}{
		{
			name: "spec ownership spans versions",
			managedFields: []metav1.ManagedFieldsEntry{
				managedFieldsEntry(
					classicManager,
					metav1.ManagedFieldsOperationUpdate,
					"infrastructure.cluster.x-k8s.io/v1beta1",
					`{"f:spec":{"f:diskSize":{}}}`,
				),
				managedFieldsEntry(
					classicManager,
					metav1.ManagedFieldsOperationUpdate,
					"infrastructure.cluster.x-k8s.io/v1beta2",
					`{"f:spec":{"f:storage":{"f:size":{}}}}`,
				),
			},
		},
		{
			name: "preserved main fields would require another Apply identity",
			managedFields: []metav1.ManagedFieldsEntry{
				managedFieldsEntry(
					"capi-kthreescontrolplane",
					metav1.ManagedFieldsOperationApply,
					"infrastructure.cluster.x-k8s.io/v1beta2",
					`{
						"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}},
						"f:other":{"f:preserved":{}}
					}`,
				),
				managedFieldsEntry(
					classicManager,
					metav1.ManagedFieldsOperationUpdate,
					"infrastructure.cluster.x-k8s.io/v1beta1",
					`{"f:spec":{"f:diskSize":{}}}`,
				),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			obj := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "related-object",
					Namespace:       metav1.NamespaceDefault,
					ResourceVersion: "1",
					ManagedFields:   tt.managedFields,
				},
			}
			originalManagedFields := obj.GetManagedFields()
			originalMainEntries := countManagedFieldsEntries(
				obj,
				"capi-kthreescontrolplane",
				metav1.ManagedFieldsOperationApply,
				"",
			)
			c := &managedFieldsPatchClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

			result, err := MigrateManagedFields(
				context.Background(),
				c,
				c,
				obj,
				"capi-kthreescontrolplane",
				"capi-kthreescontrolplane-metadata",
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).To(Equal(ManagedFieldsMigrationResult{
				Outcome: ManagedFieldsMigrationInPlaceUpdateUnsupported,
				Reason:  "related-object spec ownership spans multiple API versions",
			}))
			g.Expect(c.patchAttempts).To(BeZero())
			g.Expect(obj.GetManagedFields()).To(Equal(originalManagedFields))
			g.Expect(countManagedFieldsEntries(
				obj,
				"capi-kthreescontrolplane",
				metav1.ManagedFieldsOperationApply,
				"",
			)).To(Equal(originalMainEntries))
		})
	}
}

func TestMigrateManagedFieldsRejectsInvalidFieldsV1(t *testing.T) {
	tests := []struct {
		name     string
		fieldsV1 *metav1.FieldsV1
	}{
		{name: "nil FieldsV1"},
		{name: "malformed FieldsV1", fieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":`)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			obj := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "related-object",
					Namespace: metav1.NamespaceDefault,
					ManagedFields: []metav1.ManagedFieldsEntry{{
						Manager:    "capi-kthreescontrolplane",
						Operation:  metav1.ManagedFieldsOperationApply,
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
						FieldsType: "FieldsV1",
						FieldsV1:   tt.fieldsV1,
					}},
				},
			}
			c := &managedFieldsPatchClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}

			_, err := MigrateManagedFields(
				context.Background(),
				c,
				c,
				obj,
				"capi-kthreescontrolplane",
				"capi-kthreescontrolplane-metadata",
			)
			g.Expect(err).To(MatchError(ContainSubstring("FieldsV1")))
			g.Expect(c.patchAttempts).To(BeZero())
		})
	}
}

func TestMigrateManagedFieldsRetriesConflictFromLiveState(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	mainManager := "capi-kthreescontrolplane"
	metadataManager := "capi-kthreescontrolplane-metadata"
	apiVersion := "infrastructure.cluster.x-k8s.io/v1beta1"
	classicEntry := managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersion, `{
		"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}},
		"f:spec":{"f:diskSize":{}}
	}`)
	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "related-object",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
			ManagedFields:   []metav1.ManagedFieldsEntry{classicEntry},
		},
	}
	concurrentEntry := managedFieldsEntry("concurrent-manager", metav1.ManagedFieldsOperationUpdate, "v1", `{"f:data":{"f:concurrent":{}}}`)
	newer := obj.DeepCopy()
	newer.ResourceVersion = "2"
	newer.ManagedFields = append(newer.ManagedFields, concurrentEntry)
	newer.Data = map[string]string{"concurrent": "value"}
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &managedFieldsPatchClient{
		Client:                 baseClient,
		conflictsRemaining:     1,
		successResourceVersion: "3",
	}
	apiReader := &configMapReader{Reader: baseClient, object: newer}

	result, err := MigrateManagedFields(context.Background(), c, apiReader, obj, mainManager, metadataManager)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(ManagedFieldsMigrationResult{Outcome: ManagedFieldsMigrationCompleted}))
	g.Expect(c.patchAttempts).To(Equal(2))
	g.Expect(c.resourceVersions).To(Equal([]string{"1", "2"}))
	g.Expect(c.optimisticLockResourceVersions).To(Equal([]string{"1", "2"}))
	g.Expect(apiReader.gets).To(Equal(1))
	g.Expect(obj.GetResourceVersion()).To(Equal("3"))
	g.Expect(obj.GetManagedFields()).To(ContainElement(concurrentEntry))
	g.Expect(obj.Data).To(Equal(newer.Data))
}

func TestMigrateManagedFieldsRecomputesUnsupportedAfterConflict(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	apiVersionV1 := "infrastructure.cluster.x-k8s.io/v1beta1"
	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "related-object",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedFieldsEntry(classicManager, metav1.ManagedFieldsOperationUpdate, apiVersionV1, `{"f:spec":{"f:diskSize":{}}}`),
			},
		},
	}
	newer := obj.DeepCopy()
	newer.ResourceVersion = "2"
	newer.ManagedFields = append(newer.ManagedFields, managedFieldsEntry(
		classicManager,
		metav1.ManagedFieldsOperationUpdate,
		"infrastructure.cluster.x-k8s.io/v1beta2",
		`{"f:spec":{"f:storage":{"f:size":{}}}}`,
	))
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &managedFieldsPatchClient{Client: baseClient, conflictsRemaining: 1}
	apiReader := &configMapReader{Reader: baseClient, object: newer}

	result, err := MigrateManagedFields(
		context.Background(),
		c,
		apiReader,
		obj,
		"capi-kthreescontrolplane",
		"capi-kthreescontrolplane-metadata",
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(ManagedFieldsMigrationResult{
		Outcome: ManagedFieldsMigrationInPlaceUpdateUnsupported,
		Reason:  "related-object spec ownership spans multiple API versions",
	}))
	g.Expect(c.patchAttempts).To(Equal(1))
	g.Expect(apiReader.gets).To(Equal(1))
}

func TestMigrateManagedFieldsReturnsLiveReadError(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	obj := migratableConfigMap()
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &managedFieldsPatchClient{Client: baseClient, conflictsRemaining: 1}
	apiReader := &errorReader{Reader: baseClient, err: errors.New("live read failed")}

	_, err := MigrateManagedFields(
		context.Background(),
		c,
		apiReader,
		obj,
		"capi-kthreescontrolplane",
		"capi-kthreescontrolplane-metadata",
	)
	g.Expect(err).To(MatchError(ContainSubstring("live read failed")))
	g.Expect(c.patchAttempts).To(Equal(1))
	g.Expect(apiReader.gets).To(Equal(1))
}

func TestMigrateManagedFieldsLimitsConflictRetries(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	obj := migratableConfigMap()
	newer := obj.DeepCopy()
	newer.ResourceVersion = "2"
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &managedFieldsPatchClient{
		Client:             baseClient,
		conflictsRemaining: retry.DefaultRetry.Steps + 1,
	}
	apiReader := &configMapReader{Reader: baseClient, object: newer}

	_, err := MigrateManagedFields(
		context.Background(),
		c,
		apiReader,
		obj,
		"capi-kthreescontrolplane",
		"capi-kthreescontrolplane-metadata",
	)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsConflict(err)).To(BeTrue())
	g.Expect(c.patchAttempts).To(Equal(retry.DefaultRetry.Steps))
	g.Expect(apiReader.gets).To(Equal(retry.DefaultRetry.Steps - 1))
}

type errorReader struct {
	client.Reader
	err  error
	gets int
}

func (r *errorReader) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	r.gets++
	return r.err
}

func migratableConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "related-object",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedFieldsEntry(
					classicManager,
					metav1.ManagedFieldsOperationUpdate,
					"infrastructure.cluster.x-k8s.io/v1beta1",
					`{
						"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}},
						"f:spec":{"f:diskSize":{}}
					}`,
				),
			},
		},
	}
}

func managedFieldsEntry(manager string, operation metav1.ManagedFieldsOperationType, apiVersion, fieldsV1 string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{
		Manager:    manager,
		Operation:  operation,
		APIVersion: apiVersion,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(fieldsV1)},
	}
}

func findManagedFieldsEntry(
	obj client.Object,
	manager string,
	operation metav1.ManagedFieldsOperationType,
	subresource string,
) *metav1.ManagedFieldsEntry {
	managedFields := obj.GetManagedFields()
	for i := range managedFields {
		entry := &managedFields[i]
		if entry.Manager == manager && entry.Operation == operation && entry.Subresource == subresource {
			return entry
		}
	}
	return nil
}

func countManagedFieldsEntries(
	obj client.Object,
	manager string,
	operation metav1.ManagedFieldsOperationType,
	subresource string,
) int {
	count := 0
	for _, entry := range obj.GetManagedFields() {
		if entry.Manager == manager && entry.Operation == operation && entry.Subresource == subresource {
			count++
		}
	}
	return count
}

func managedFieldsEntryOwns(t *testing.T, entry *metav1.ManagedFieldsEntry, path ...string) bool {
	t.Helper()
	fields := map[string]interface{}{}
	NewWithT(t).Expect(json.Unmarshal(entry.FieldsV1.Raw, &fields)).To(Succeed())
	_, found, err := unstructured.NestedFieldNoCopy(fields, path...)
	NewWithT(t).Expect(err).NotTo(HaveOccurred())
	return found
}
