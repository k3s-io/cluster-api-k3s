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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type patchTrackingClient struct {
	client.Client
	dryRun bool
}

func (c *patchTrackingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	patchOptions := &client.PatchOptions{}
	for _, opt := range opts {
		opt.ApplyToPatch(patchOptions)
	}
	for _, value := range patchOptions.DryRun {
		if value == metav1.DryRunAll {
			c.dryRun = true
		}
	}
	obj.SetResourceVersion("1")
	return nil
}

type patchTrackingCache struct {
	added []string
}

func (c *patchTrackingCache) Add(key string) {
	c.added = append(c.added, key)
}

func (c *patchTrackingCache) Has(string) bool {
	return false
}

func TestPatchWithDryRunDoesNotCacheResult(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	trackingClient := &patchTrackingClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}
	trackingCache := &patchTrackingCache{}
	original := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "config",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "1",
		},
	}
	modified := original.DeepCopy()
	modified.Data = map[string]string{"key": "value"}

	g.Expect(Patch(
		context.Background(),
		trackingClient,
		"test-manager",
		modified,
		WithCachingProxy{Cache: trackingCache, Original: original},
		WithDryRun{},
	)).To(Succeed())
	g.Expect(trackingClient.dryRun).To(BeTrue())
	g.Expect(trackingCache.added).To(BeEmpty())
}
