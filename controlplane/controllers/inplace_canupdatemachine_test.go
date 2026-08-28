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
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

const (
	currentK3sVersion = "v1.31.1+k3s1"
	desiredK3sVersion = "v1.31.2+k3s1"
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
				result.CurrentKThreesConfig.Spec.Version = currentK3sVersion
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

func TestCanUpdateMachineNormalizesCurrentRelatedObjectMetadataWithoutChangingSpecs(t *testing.T) {
	g := NewWithT(t)
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
	machine, result, c := canUpdateFixtures(t)
	result.CurrentKThreesConfig.Labels = map[string]string{"stale": "label"}
	result.CurrentKThreesConfig.Annotations = map[string]string{"stale": "annotation"}
	result.CurrentKThreesConfig.Spec.Version = currentK3sVersion
	result.CurrentKThreesConfig.Spec.PostK3sCommands = []string{"preserved"}
	result.DesiredKThreesConfig.Labels = map[string]string{"desired": "label"}
	result.DesiredKThreesConfig.Annotations = map[string]string{"desired": "annotation"}
	result.DesiredKThreesConfig.Spec.Version = desiredK3sVersion
	result.DesiredKThreesConfig.Spec.PostK3sCommands = []string{"preserved"}
	result.CurrentInfraMachine.SetLabels(map[string]string{"stale": "label"})
	result.CurrentInfraMachine.SetAnnotations(map[string]string{"stale": "annotation"})
	result.DesiredInfraMachine.SetLabels(map[string]string{"desired": "label"})
	result.DesiredInfraMachine.SetAnnotations(map[string]string{"desired": "annotation"})

	runtimeClient := &fakeRuntimeClient{
		handlers: []string{"handler"},
		response: runtimehooksv1.CanUpdateMachineResponse{
			MachinePatch: jsonPatch(`[{"op":"replace","path":"/spec/version","value":"v1.31.2+k3s1"}]`),
		},
	}
	r := &KThreesControlPlaneReconciler{Client: c, RuntimeClient: runtimeClient}

	canUpdate, err := r.canUpdateMachine(context.Background(), machine, result)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(canUpdate).To(BeTrue())
	g.Expect(runtimeClient.request).NotTo(BeNil())

	currentConfig := &bootstrapv1.KThreesConfig{}
	desiredConfig := &bootstrapv1.KThreesConfig{}
	g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
		runtimeClient.request.Current.BootstrapConfig.Object.(*unstructured.Unstructured).Object,
		currentConfig,
	)).To(Succeed())
	g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
		runtimeClient.request.Desired.BootstrapConfig.Object.(*unstructured.Unstructured).Object,
		desiredConfig,
	)).To(Succeed())
	g.Expect(currentConfig.Labels).To(Equal(map[string]string{"desired": "label"}))
	g.Expect(currentConfig.Annotations).To(Equal(map[string]string{"desired": "annotation"}))
	g.Expect(currentConfig.Spec.Version).To(BeEmpty())
	g.Expect(desiredConfig.Spec.Version).To(BeEmpty())
	g.Expect(currentConfig.Spec.PostK3sCommands).To(Equal([]string{"preserved"}))
	g.Expect(desiredConfig.Spec.PostK3sCommands).To(Equal([]string{"preserved"}))

	currentInfra := runtimeClient.request.Current.InfrastructureMachine.Object.(*unstructured.Unstructured)
	desiredInfra := runtimeClient.request.Desired.InfrastructureMachine.Object.(*unstructured.Unstructured)
	g.Expect(currentInfra.GetLabels()).To(Equal(map[string]string{"desired": "label"}))
	g.Expect(currentInfra.GetAnnotations()).To(Equal(map[string]string{"desired": "annotation"}))
	g.Expect(currentInfra.Object["spec"]).To(Equal(map[string]interface{}{"size": "small"}))
	g.Expect(desiredInfra.Object["spec"]).To(Equal(map[string]interface{}{"size": "small"}))

	g.Expect(result.CurrentKThreesConfig.Labels).To(Equal(map[string]string{"stale": "label"}))
	g.Expect(result.CurrentKThreesConfig.Spec.Version).To(Equal(currentK3sVersion))
	g.Expect(result.CurrentInfraMachine.GetLabels()).To(Equal(map[string]string{"stale": "label"}))
}

func TestCanUpdateMachineDesiredInfraExpectedDryRunIsNonCoverable(t *testing.T) {
	tests := map[string]error{
		"invalid": apierrors.NewInvalid(
			schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "TestMachine"},
			"infra-1",
			field.ErrorList{field.Invalid(field.NewPath("spec", "size"), "large", "immutable")},
		),
		"forbidden": apierrors.NewForbidden(
			schema.GroupResource{Group: "infrastructure.cluster.x-k8s.io", Resource: "testmachines"},
			"infra-1",
			errors.New(`admission webhook denied the request: immutable field`),
		),
	}

	for name, dryRunErr := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
			machine, result, c := canUpdateFixtures(t)
			result.DesiredInfraMachine.Object["spec"] = map[string]interface{}{"size": "large"}
			runtimeClient := &fakeRuntimeClient{handlers: []string{"handler"}}
			r := &KThreesControlPlaneReconciler{
				Client: &nthInfraPatchErrorClient{
					Client: c,
					failAt: 2,
					err:    dryRunErr,
				},
				RuntimeClient: runtimeClient,
			}

			canUpdate, err := r.canUpdateMachine(context.Background(), machine, result)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(canUpdate).To(BeFalse())
			g.Expect(runtimeClient.callCount).To(BeZero())
		})
	}
}

func TestCanUpdateMachineDesiredInfraUnexpectedDryRunReturnsError(t *testing.T) {
	g := NewWithT(t)
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
	machine, result, c := canUpdateFixtures(t)
	result.DesiredInfraMachine.Object["spec"] = map[string]interface{}{"size": "large"}
	runtimeClient := &fakeRuntimeClient{handlers: []string{"handler"}}
	r := &KThreesControlPlaneReconciler{
		Client: &nthInfraPatchErrorClient{
			Client: c,
			failAt: 2,
			err:    errors.New("unexpected dry-run failure"),
		},
		RuntimeClient: runtimeClient,
	}

	canUpdate, err := r.canUpdateMachine(context.Background(), machine, result)

	g.Expect(err).To(MatchError(ContainSubstring("server side apply dry-run failed for desired InfraMachine")))
	g.Expect(canUpdate).To(BeFalse())
	g.Expect(runtimeClient.callCount).To(BeZero())
}

func TestCanUpdateMachineNonCoverableInfraReasonIsSanitized(t *testing.T) {
	const sentinel = "sentinel-dry-run-secret"
	g := NewWithT(t)
	machine, result, c := canUpdateFixtures(t)
	r := &KThreesControlPlaneReconciler{
		Client: &infraPatchErrorClient{
			Client: c,
			err: apierrors.NewInvalid(
				schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "TestMachine"},
				"infra-1",
				field.ErrorList{field.Invalid(field.NewPath("spec", "credentials", "password"), sentinel, "invalid")},
			),
		},
		RuntimeClient: &fakeRuntimeClient{},
	}

	canUpdate, reasons, err := r.canExtensionsUpdateMachine(context.Background(), machine, result, []string{"handler"})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(canUpdate).To(BeFalse())
	g.Expect(reasons).To(Equal([]string{"TestMachine spec is not fully covered for in-place update"}))
	g.Expect(strings.Join(reasons, ",")).NotTo(ContainSubstring(sentinel))
}

func TestMatchesMachineReportsOnlySanitizedUncoveredReasons(t *testing.T) {
	const sentinel = "sentinel-capability-secret"
	oldContent := "old-content-" + sentinel
	newContent := "new-content-" + sentinel
	oldCommand := "old-command-" + sentinel
	newCommand := "new-command-" + sentinel
	currentMachine, result, _ := canUpdateFixtures(t)

	request := &runtimehooksv1.CanUpdateMachineRequest{
		Current: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine: *currentMachine,
			BootstrapConfig: runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": bootstrapv1.GroupVersion.String(),
				"kind":       "KThreesConfig",
				"spec": map[string]interface{}{
					"files":          []interface{}{map[string]interface{}{"path": "/etc/sensitive", "content": oldContent}},
					"preK3sCommands": []interface{}{oldCommand},
				},
			}}},
			InfrastructureMachine: runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "TestMachine",
				"spec":       map[string]interface{}{"credentials": map[string]interface{}{"password": oldContent}},
			}}},
		},
		Desired: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine: *result.DesiredMachine,
			BootstrapConfig: runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": bootstrapv1.GroupVersion.String(),
				"kind":       "KThreesConfig",
				"spec": map[string]interface{}{
					"files":          []interface{}{map[string]interface{}{"path": "/etc/sensitive", "content": newContent}},
					"preK3sCommands": []interface{}{newCommand},
				},
			}}},
			InfrastructureMachine: runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
				"kind":       "TestMachine",
				"spec":       map[string]interface{}{"credentials": map[string]interface{}{"password": newContent}},
			}}},
		},
	}

	matches, reasons, err := matchesMachine(request)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(matches).To(BeFalse())
	g.Expect(reasons).To(Equal([]string{
		`Machine version "v1.31.1+k3s1" is not equal to desired version "v1.31.2+k3s1"`,
		"KThreesConfig spec is not fully covered for in-place update",
		"TestMachine spec is not fully covered for in-place update",
	}))
	for _, sensitiveValue := range []string{sentinel, oldContent, newContent, oldCommand, newCommand} {
		g.Expect(strings.Join(reasons, ", ")).NotTo(ContainSubstring(sensitiveValue))
	}
}

func TestCanUpdateMachineLogsOnlySanitizedReasons(t *testing.T) {
	const sentinel = "sentinel-capability-log-secret"
	oldValue := "old-" + sentinel
	newValue := "new-" + sentinel
	g := NewWithT(t)
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
	machine, result, c := canUpdateFixtures(t)
	result.CurrentKThreesConfig.Spec.PostK3sCommands = []string{oldValue}
	result.DesiredKThreesConfig.Spec.PostK3sCommands = []string{newValue}
	result.CurrentInfraMachine.Object["spec"] = map[string]interface{}{"credentials": map[string]interface{}{"password": oldValue}}
	result.DesiredInfraMachine.Object["spec"] = map[string]interface{}{"credentials": map[string]interface{}{"password": newValue}}

	var logLines []string
	logger := funcr.New(func(prefix, args string) {
		logLines = append(logLines, prefix+args)
	}, funcr.Options{})
	ctx := ctrl.LoggerInto(context.Background(), logger)
	r := &KThreesControlPlaneReconciler{
		Client:        c,
		RuntimeClient: &fakeRuntimeClient{handlers: []string{"handler"}},
	}

	canUpdate, err := r.canUpdateMachine(ctx, machine, result)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(canUpdate).To(BeFalse())
	logOutput := strings.Join(logLines, "\n")
	g.Expect(logOutput).To(ContainSubstring(
		`Machine version \"v1.31.1+k3s1\" is not equal to desired version \"v1.31.2+k3s1\"`,
	))
	g.Expect(logOutput).To(ContainSubstring("KThreesConfig spec is not fully covered for in-place update"))
	g.Expect(logOutput).To(ContainSubstring("TestMachine spec is not fully covered for in-place update"))
	for _, sensitiveValue := range []string{sentinel, oldValue, newValue} {
		g.Expect(logOutput).NotTo(ContainSubstring(sensitiveValue))
	}
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
		Spec: clusterv1.MachineSpec{Version: currentK3sVersion},
	}
	desiredMachine := currentMachine.DeepCopy()
	desiredMachine.Spec.Version = desiredK3sVersion

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
	request   *runtimehooksv1.CanUpdateMachineRequest
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
	request runtimehooksv1.RequestObject,
	response runtimehooksv1.ResponseObject,
	_ ...runtimeclient.CallExtensionOption,
) error {
	f.callCount++
	f.request = request.(*runtimehooksv1.CanUpdateMachineRequest).DeepCopy()
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
