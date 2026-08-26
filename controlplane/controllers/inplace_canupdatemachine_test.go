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
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/component-base/featuregate/testing"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	runtimev1 "sigs.k8s.io/cluster-api/api/runtime/v1beta2"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimeclient "sigs.k8s.io/cluster-api/exp/runtime/client"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

func TestCanUpdateMachine(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		mutate        func(*k3s.UpToDateResult)
		handlers      []string
		discoveryErr  error
		callErr       error
		infraPatchErr error
		response      runtimehooksv1.CanUpdateMachineResponse
		want          bool
		wantErrText   string
	}{
		{name: "feature disabled"},
		{
			name:     "nil related object",
			enabled:  true,
			handlers: []string{"handler"},
			mutate: func(result *k3s.UpToDateResult) {
				result.DesiredInfraMachine = nil
			},
		},
		{name: "zero handlers", enabled: true},
		{
			name:         "discovery error",
			enabled:      true,
			discoveryErr: errors.New("discovery failed"),
			wantErrText:  "discovery failed",
		},
		{
			name:        "multiple handlers",
			enabled:     true,
			handlers:    []string{"one", "two"},
			wantErrText: "found multiple CanUpdateMachine hooks",
		},
		{
			name:        "call error",
			enabled:     true,
			handlers:    []string{"handler"},
			callErr:     errors.New("extension failed"),
			wantErrText: "extension failed",
		},
		{
			name:     "invalid hook patch returns error",
			enabled:  true,
			handlers: []string{"handler"},
			response: runtimehooksv1.CanUpdateMachineResponse{
				MachinePatch: jsonPatch(`[{"op":"replace","path":"/spec/version","value":`),
			},
			wantErrText: "failed to apply patches from extension handler",
		},
		{
			name:          "unexpected InfraMachine dry-run error is returned",
			enabled:       true,
			handlers:      []string{"handler"},
			infraPatchErr: errors.New("unexpected dry-run failure"),
			wantErrText:   "server side apply dry-run failed for current InfraMachine",
		},
		{
			name:     "invalid InfraMachine dry-run is not coverable",
			enabled:  true,
			handlers: []string{"handler"},
			infraPatchErr: apierrors.NewInvalid(
				schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "TestMachine"},
				"infra-1",
				field.ErrorList{field.Invalid(field.NewPath("spec"), nil, "invalid")},
			),
		},
		{
			name:     "forbidden InfraMachine dry-run is not coverable",
			enabled:  true,
			handlers: []string{"handler"},
			infraPatchErr: apierrors.NewForbidden(
				schema.GroupResource{Group: "infrastructure.cluster.x-k8s.io", Resource: "testmachines"},
				"infra-1",
				errors.New("forbidden"),
			),
		},
		{
			name:     "version patch covers complete diff",
			enabled:  true,
			handlers: []string{"handler"},
			response: runtimehooksv1.CanUpdateMachineResponse{
				MachinePatch: jsonPatch(`[{"op":"replace","path":"/spec/version","value":"v1.31.2+k3s1"}]`),
			},
			want: true,
		},
		{
			name:     "version patch ignores populated current bootstrap version",
			enabled:  true,
			handlers: []string{"handler"},
			mutate: func(result *k3s.UpToDateResult) {
				result.CurrentKThreesConfig.Spec.Version = "v1.31.1+k3s1"
				result.DesiredKThreesConfig.Spec.Version = ""
			},
			response: runtimehooksv1.CanUpdateMachineResponse{
				MachinePatch: jsonPatch(`[{"op":"replace","path":"/spec/version","value":"v1.31.2+k3s1"}]`),
			},
			want: true,
		},
		{
			name:     "empty infrastructure patch leaves difference",
			enabled:  true,
			handlers: []string{"handler"},
			mutate: func(result *k3s.UpToDateResult) {
				result.DesiredInfraMachine.Object["spec"] = map[string]interface{}{"size": "large"}
			},
			response: runtimehooksv1.CanUpdateMachineResponse{
				MachinePatch:               jsonPatch(`[{"op":"replace","path":"/spec/version","value":"v1.31.2+k3s1"}]`),
				InfrastructureMachinePatch: jsonPatch(`[]`),
			},
		},
		{
			name:     "bootstrap-only partial coverage leaves version difference",
			enabled:  true,
			handlers: []string{"handler"},
			mutate: func(result *k3s.UpToDateResult) {
				result.DesiredKThreesConfig.Spec.PostK3sCommands = []string{"new"}
			},
			response: runtimehooksv1.CanUpdateMachineResponse{
				BootstrapConfigPatch: jsonPatch(`[{"op":"add","path":"/spec/postK3sCommands","value":["new"]}]`),
			},
		},
		{
			name:     "full three-object coverage",
			enabled:  true,
			handlers: []string{"handler"},
			mutate: func(result *k3s.UpToDateResult) {
				result.DesiredKThreesConfig.Spec.PostK3sCommands = []string{"new"}
				result.DesiredInfraMachine.Object["spec"] = map[string]interface{}{"size": "large"}
			},
			response: runtimehooksv1.CanUpdateMachineResponse{
				MachinePatch:               jsonPatch(`[{"op":"replace","path":"/spec/version","value":"v1.31.2+k3s1"}]`),
				BootstrapConfigPatch:       jsonPatch(`[{"op":"add","path":"/spec/postK3sCommands","value":["new"]}]`),
				InfrastructureMachinePatch: jsonPatch(`[{"op":"replace","path":"/spec/size","value":"large"}]`),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, tt.enabled)
			machine, result, c := canUpdateFixtures(t)
			if tt.infraPatchErr != nil {
				c = &infraPatchErrorClient{Client: c, err: tt.infraPatchErr}
			}
			if tt.mutate != nil {
				tt.mutate(&result)
			}
			r := &KThreesControlPlaneReconciler{
				Client: c,
				RuntimeClient: &fakeRuntimeClient{
					handlers:  tt.handlers,
					getAllErr: tt.discoveryErr,
					callErr:   tt.callErr,
					response:  tt.response,
				},
			}

			canUpdate, err := r.canUpdateMachine(context.Background(), machine, result)
			if tt.wantErrText != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErrText)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(canUpdate).To(Equal(tt.want))
		})
	}
}

func TestCanUpdateMachineCallsExtensionWhenDesiredInfraDryRunRejectsImmutableChange(t *testing.T) {
	g := NewWithT(t)
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
	machine, result, c := canUpdateFixtures(t)
	result.DesiredInfraMachine.Object["spec"] = map[string]interface{}{"size": "large"}
	runtimeClient := &fakeRuntimeClient{
		handlers: []string{"handler"},
		response: runtimehooksv1.CanUpdateMachineResponse{
			MachinePatch:               jsonPatch(`[{"op":"replace","path":"/spec/version","value":"v1.31.2+k3s1"}]`),
			InfrastructureMachinePatch: jsonPatch(`[]`),
		},
	}
	r := &KThreesControlPlaneReconciler{
		Client: &nthInfraPatchErrorClient{
			Client: c,
			failAt: 2,
			err: apierrors.NewForbidden(
				schema.GroupResource{Group: "infrastructure.cluster.x-k8s.io", Resource: "testmachines"},
				"infra-1",
				errors.New(`admission webhook "validation.testmachine.infrastructure.cluster.x-k8s.io" denied the request: immutable field`),
			),
		},
		RuntimeClient: runtimeClient,
	}

	canUpdate, err := r.canUpdateMachine(context.Background(), machine, result)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(canUpdate).To(BeFalse())
	g.Expect(runtimeClient.callCount).To(Equal(1))
}

func TestCanUpdateMachineReturnsDesiredInfraRBACDryRunError(t *testing.T) {
	g := NewWithT(t)
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
	machine, result, c := canUpdateFixtures(t)
	result.DesiredInfraMachine.Object["spec"] = map[string]interface{}{"size": "large"}
	runtimeClient := &fakeRuntimeClient{handlers: []string{"handler"}}
	r := &KThreesControlPlaneReconciler{
		Client: &nthInfraPatchErrorClient{
			Client: c,
			failAt: 2,
			err: apierrors.NewForbidden(
				schema.GroupResource{Group: "infrastructure.cluster.x-k8s.io", Resource: "testmachines"},
				"infra-1",
				errors.New("RBAC denied"),
			),
		},
		RuntimeClient: runtimeClient,
	}

	canUpdate, err := r.canUpdateMachine(context.Background(), machine, result)

	g.Expect(err).To(MatchError(ContainSubstring("server side apply dry-run failed for desired InfraMachine")))
	g.Expect(canUpdate).To(BeFalse())
	g.Expect(runtimeClient.callCount).To(BeZero())
}

func canUpdateFixtures(t *testing.T) (*clusterv1.Machine, k3s.UpToDateResult, client.Client) {
	t.Helper()
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())

	currentMachine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "machine-1", Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{Version: "v1.31.1+k3s1"},
	}
	desiredMachine := currentMachine.DeepCopy()
	desiredMachine.Spec.Version = "v1.31.2+k3s1"

	currentConfig := &bootstrapv1.KThreesConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: bootstrapv1.GroupVersion.String(), Kind: "KThreesConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-1", Namespace: "default"},
	}
	desiredConfig := currentConfig.DeepCopy()

	currentInfra := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       "TestMachine",
		"metadata":   map[string]interface{}{"name": "infra-1", "namespace": "default"},
		"spec":       map[string]interface{}{"size": "small"},
	}}
	desiredInfra := currentInfra.DeepCopy()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(currentMachine, currentInfra).Build()

	return currentMachine, k3s.UpToDateResult{
		EligibleForInPlaceUpdate: true,
		DesiredMachine:           desiredMachine,
		CurrentInfraMachine:      currentInfra,
		DesiredInfraMachine:      desiredInfra,
		CurrentKThreesConfig:     currentConfig,
		DesiredKThreesConfig:     desiredConfig,
	}, c
}

func jsonPatch(patch string) runtimehooksv1.Patch {
	return runtimehooksv1.Patch{PatchType: runtimehooksv1.JSONPatchType, Patch: []byte(patch)}
}

type fakeRuntimeClient struct {
	handlers  []string
	getAllErr error
	callErr   error
	response  runtimehooksv1.CanUpdateMachineResponse
	callCount int
}

func (f *fakeRuntimeClient) WarmUp(*runtimev1.ExtensionConfigList) error { return nil }
func (f *fakeRuntimeClient) IsReady() bool                               { return true }
func (f *fakeRuntimeClient) Discover(_ context.Context, e *runtimev1.ExtensionConfig) (*runtimev1.ExtensionConfig, error) {
	return e, nil
}
func (f *fakeRuntimeClient) Register(*runtimev1.ExtensionConfig) error   { return nil }
func (f *fakeRuntimeClient) Unregister(*runtimev1.ExtensionConfig) error { return nil }
func (f *fakeRuntimeClient) GetAllExtensions(context.Context, runtimecatalog.Hook, client.Object) ([]string, error) {
	return f.handlers, f.getAllErr
}

type infraPatchErrorClient struct {
	client.Client
	err error
}

func (c *infraPatchErrorClient) Patch(ctx context.Context, object client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if object.GetObjectKind().GroupVersionKind().Kind == "TestMachine" {
		return c.err
	}
	return c.Client.Patch(ctx, object, patch, opts...)
}
func (f *fakeRuntimeClient) CallAllExtensions(context.Context, runtimecatalog.Hook, client.Object, runtimehooksv1.RequestObject, runtimehooksv1.ResponseObject) error {
	return nil
}
func (f *fakeRuntimeClient) CallExtension(
	_ context.Context,
	_ runtimecatalog.Hook,
	_ client.Object,
	_ string,
	_ runtimehooksv1.RequestObject,
	response runtimehooksv1.ResponseObject,
	_ ...runtimeclient.CallExtensionOption,
) error {
	f.callCount++
	if f.callErr != nil {
		return f.callErr
	}
	typed := response.(*runtimehooksv1.CanUpdateMachineResponse)
	*typed = *f.response.DeepCopy()
	return nil
}

type nthInfraPatchErrorClient struct {
	client.Client
	failAt int
	count  int
	err    error
}

func (c *nthInfraPatchErrorClient) Patch(ctx context.Context, object client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if object.GetObjectKind().GroupVersionKind().Kind == "TestMachine" {
		c.count++
		if c.count == c.failAt {
			return c.err
		}
	}
	return c.Client.Patch(ctx, object, patch, opts...)
}
