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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
