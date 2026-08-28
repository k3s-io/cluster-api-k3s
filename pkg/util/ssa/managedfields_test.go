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

func TestNeedsMigration(t *testing.T) {
	tests := []struct {
		name          string
		manager       string
		operation     metav1.ManagedFieldsOperationType
		subresource   string
		fieldsV1      string
		wantMigration bool
	}{
		{
			name:          "old main Apply owns cluster-name label",
			manager:       "old-manager",
			operation:     metav1.ManagedFieldsOperationApply,
			fieldsV1:      `{"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}}}`,
			wantMigration: true,
		},
		{
			name:      "old main Apply does not own cluster-name label",
			manager:   "old-manager",
			operation: metav1.ManagedFieldsOperationApply,
			fieldsV1:  `{"f:metadata":{"f:labels":{"f:other":{}}}}`,
		},
		{
			name:        "old main status Apply owns cluster-name label",
			manager:     "old-manager",
			operation:   metav1.ManagedFieldsOperationApply,
			subresource: "status",
			fieldsV1:    `{"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}}}`,
		},
		{
			name:      "old main Update owns cluster-name label",
			manager:   "old-manager",
			operation: metav1.ManagedFieldsOperationUpdate,
			fieldsV1:  `{"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}}}`,
		},
		{
			name:      "other manager Apply owns cluster-name label",
			manager:   "other-manager",
			operation: metav1.ManagedFieldsOperationApply,
			fieldsV1:  `{"f:metadata":{"f:labels":{"f:cluster.x-k8s.io/cluster-name":{}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			object := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ManagedFields: []metav1.ManagedFieldsEntry{{
						Manager:     tt.manager,
						Operation:   tt.operation,
						Subresource: tt.subresource,
						FieldsV1:    &metav1.FieldsV1{Raw: []byte(tt.fieldsV1)},
					}},
				},
			}

			got, err := needsMigration(object, "old-manager")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(tt.wantMigration))
		})
	}
}
