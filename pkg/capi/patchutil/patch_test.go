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

package patchutil

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestApplyPatchToObjectDoesNotLogSensitiveJSONPatch(t *testing.T) {
	const sentinel = "sentinel-json-patch-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	obj := testRawExtension()
	patchBody := []byte(fmt.Sprintf(`[
		{"op":"replace","path":"/spec/commands/0","value":%q},
		{"op":"replace","path":"/spec/version","value":"v2"},
		{"op":"add","path":"/metadata/annotations","value":{"secret":%q}}
	]`, sentinel, sentinel))

	changed, err := ApplyPatchToObject(ctx, &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(changed).To(BeTrue())
	g.Expect(obj.Raw).To(MatchJSON(fmt.Sprintf(`{
		"apiVersion":"infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":"TestMachine",
		"metadata":{"name":"machine-1","labels":{"preserved":"true"}},
		"spec":{"commands":[%q],"version":"v2"}
	}`, sentinel)))
	logOutput := logs()
	g.Expect(logOutput).To(ContainSubstring("JSONPatch"))
	g.Expect(logOutput).To(ContainSubstring("operationCount"))
	g.Expect(logOutput).To(ContainSubstring(`"operationCount"=3`))
	g.Expect(logOutput).NotTo(ContainSubstring(sentinel))
	g.Expect(logOutput).NotTo(ContainSubstring(string(patchBody)))
}

func TestApplyPatchToObjectDoesNotLogSensitiveJSONMergePatch(t *testing.T) {
	const sentinel = "sentinel-json-merge-patch-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	obj := testRawExtension()
	patchBody := []byte(fmt.Sprintf(`{
		"metadata":{"annotations":{"secret":%q}},
		"spec":{"commands":[%q],"version":"v2"}
	}`, sentinel, sentinel))

	changed, err := ApplyPatchToObject(ctx, &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONMergePatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(changed).To(BeTrue())
	g.Expect(obj.Raw).To(MatchJSON(fmt.Sprintf(`{
		"apiVersion":"infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":"TestMachine",
		"metadata":{"name":"machine-1","labels":{"preserved":"true"}},
		"spec":{"commands":[%q],"version":"v2"}
	}`, sentinel)))
	logOutput := logs()
	g.Expect(logOutput).To(ContainSubstring("JSONMergePatch"))
	g.Expect(logOutput).NotTo(ContainSubstring(sentinel))
	g.Expect(logOutput).NotTo(ContainSubstring(string(patchBody)))
}

func TestApplyPatchToObjectDoesNotExposeSensitiveMalformedJSONPatch(t *testing.T) {
	const sentinel = "sentinel-malformed-patch-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	obj := testRawExtension()
	original := bytes.Clone(obj.Raw)
	patchBody := []byte(fmt.Sprintf(
		`[{"op":"replace","path":"/spec/version","value":%q}`,
		sentinel,
	))

	changed, err := ApplyPatchToObject(ctx, &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).To(MatchError("failed to apply patch: invalid JSON patch"))
	g.Expect(changed).To(BeFalse())
	g.Expect(obj.Raw).To(Equal(original))
	output := logs() + "\n" + err.Error()
	g.Expect(output).NotTo(ContainSubstring(sentinel))
	g.Expect(output).NotTo(ContainSubstring(string(patchBody)))
}

func TestApplyPatchToObjectDoesNotExposeSensitiveJSONPatchApplicationError(t *testing.T) {
	const sentinel = "sentinel-application-patch-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	obj := testRawExtension()
	original := bytes.Clone(obj.Raw)
	patchBody := []byte(fmt.Sprintf(
		`[{"op":"replace","path":"/spec/%s","value":%q}]`,
		sentinel,
		sentinel,
	))

	changed, err := ApplyPatchToObject(ctx, &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).To(MatchError("failed to apply patch: JSON patch could not be applied"))
	g.Expect(changed).To(BeFalse())
	g.Expect(obj.Raw).To(Equal(original))
	output := logs() + "\n" + err.Error()
	g.Expect(output).To(ContainSubstring("JSONPatch"))
	g.Expect(output).To(ContainSubstring(`"operationCount"=1`))
	g.Expect(output).NotTo(ContainSubstring(sentinel))
	g.Expect(output).NotTo(ContainSubstring(string(patchBody)))
}

func TestApplyPatchToObjectDoesNotExposeSensitiveJSONPatchConversionError(t *testing.T) {
	const sentinel = "sentinel-json-patch-conversion-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	obj := testRawExtension()
	original := bytes.Clone(obj.Raw)
	patchBody := []byte(fmt.Sprintf(`[
		{"op":"add","path":"/spec/credentials","value":{"password":%q}},
		{"op":"remove","path":"/kind"}
	]`, sentinel))

	changed, err := ApplyPatchToObject(ctx, &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).To(MatchError("failed to apply patch: patched object is invalid"))
	g.Expect(changed).To(BeFalse())
	g.Expect(obj.Raw).To(Equal(original))
	expectNoDisclosure(g, logs(), err, sentinel, string(patchBody))
}

func TestApplyPatchToObjectDoesNotExposeSensitiveJSONMergePatchConversionError(t *testing.T) {
	const sentinel = "sentinel-json-merge-patch-conversion-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	obj := testRawExtension()
	original := bytes.Clone(obj.Raw)
	patchBody := []byte(fmt.Sprintf(`{
		"kind":null,
		"spec":{"credentials":{"password":%q}}
	}`, sentinel))

	changed, err := ApplyPatchToObject(ctx, &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONMergePatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).To(MatchError("failed to apply patch: patched object is invalid"))
	g.Expect(changed).To(BeFalse())
	g.Expect(obj.Raw).To(Equal(original))
	expectNoDisclosure(g, logs(), err, sentinel, string(patchBody))
}

func TestApplyPatchToObjectDoesNotExposeSensitiveMalformedJSONMergePatch(t *testing.T) {
	const sentinel = "sentinel-malformed-json-merge-patch-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	obj := testRawExtension()
	original := bytes.Clone(obj.Raw)
	patchBody := []byte(fmt.Sprintf(`{"spec":{"credentials":{"password":%q}`, sentinel))

	changed, err := ApplyPatchToObject(ctx, &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONMergePatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).To(MatchError("failed to apply patch: invalid JSON merge patch"))
	g.Expect(changed).To(BeFalse())
	g.Expect(obj.Raw).To(Equal(original))
	expectNoDisclosure(g, logs(), err, sentinel, string(patchBody))
}

func TestApplyPatchToObjectPanicDiagnosticsDoNotExposeSensitivePatch(t *testing.T) {
	const sentinel = "sentinel-panic-patch-secret"
	g := NewWithT(t)
	ctx, logs := contextWithVerboseLogger()
	patchBody := []byte(fmt.Sprintf(
		`[{"op":"add","path":"/spec/credentials","value":{"password":%q}}]`,
		sentinel,
	))

	changed, err := ApplyPatchToObject(ctx, nil, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONPatchType,
		Patch:     patchBody,
	}, "spec")

	g.Expect(err).To(MatchError("failed to apply patch: internal error"))
	g.Expect(changed).To(BeFalse())
	g.Expect(logs()).To(ContainSubstring("Patch application panicked"))
	expectNoDisclosure(g, logs(), err, sentinel, string(patchBody))
}

func TestPatchDoesNotExposeSensitiveInvalidDocuments(t *testing.T) {
	const sentinel = "sentinel-invalid-document-secret"

	t.Run("source object", func(t *testing.T) {
		g := NewWithT(t)
		object := runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"spec":{"password":%q}}`, sentinel))}

		err := Patch(&object, testRawExtension().Raw, "spec")

		g.Expect(err).To(MatchError("failed to apply patch: source object is invalid"))
		g.Expect(err.Error()).NotTo(ContainSubstring(sentinel))
		g.Expect(err.Error()).NotTo(ContainSubstring(string(object.Raw)))
	})

	t.Run("patched object", func(t *testing.T) {
		g := NewWithT(t)
		object := testRawExtension()
		patchedObject := []byte(fmt.Sprintf(`{"spec":{"password":%q}}`, sentinel))

		err := Patch(&object, patchedObject, "spec")

		g.Expect(err).To(MatchError("failed to apply patch: patched object is invalid"))
		g.Expect(err.Error()).NotTo(ContainSubstring(sentinel))
		g.Expect(err.Error()).NotTo(ContainSubstring(string(patchedObject)))
	})
}

func TestCopySpecDoesNotExposeSensitiveObjectValues(t *testing.T) {
	tests := []struct {
		name          string
		input         CopySpecInput
		expectedError string
		sensitive     []string
	}{
		{
			name: "source field lookup",
			input: CopySpecInput{
				Src:          testUnstructured("SourceMachine", "source-machine", map[string]interface{}{"spec": "sentinel-source-secret"}),
				Dest:         testUnstructured("DestinationMachine", "destination-machine", map[string]interface{}{}),
				SrcSpecPath:  "spec.template",
				DestSpecPath: "spec.template",
			},
			expectedError: "failed to copy spec: source field could not be read",
			sensitive:     []string{"sentinel-source-secret", "SourceMachine", "source-machine"},
		},
		{
			name: "preserved destination field lookup",
			input: CopySpecInput{
				Src:              testUnstructured("SourceMachine", "source-machine", map[string]interface{}{"spec": map[string]interface{}{"template": "replacement"}}),
				Dest:             testUnstructured("DestinationMachine", "destination-machine", map[string]interface{}{"spec": "sentinel-preserved-secret"}),
				SrcSpecPath:      "spec.template",
				DestSpecPath:     "spec.template",
				FieldsToPreserve: []Path{{"spec", "credentials", "password"}},
			},
			expectedError: "failed to copy spec: preserved destination field could not be read",
			sensitive:     []string{"sentinel-preserved-secret", "DestinationMachine", "destination-machine"},
		},
		{
			name: "destination field set",
			input: CopySpecInput{
				Src:          testUnstructured("SourceMachine", "source-machine", map[string]interface{}{"spec": map[string]interface{}{"template": "replacement"}}),
				Dest:         testUnstructured("DestinationMachine", "destination-machine", map[string]interface{}{"spec": "sentinel-destination-secret"}),
				SrcSpecPath:  "spec.template",
				DestSpecPath: "spec.template",
			},
			expectedError: "failed to copy spec: destination field could not be set",
			sensitive:     []string{"sentinel-destination-secret", "DestinationMachine", "destination-machine"},
		},
		{
			name: "preserved destination field restore",
			input: CopySpecInput{
				Src: testUnstructured("SourceMachine", "source-machine", map[string]interface{}{
					"spec": "sentinel-restore-secret",
				}),
				Dest: testUnstructured("DestinationMachine", "destination-machine", map[string]interface{}{
					"spec": map[string]interface{}{
						"credentials": map[string]interface{}{"password": "preserved-password"},
					},
				}),
				SrcSpecPath:      "spec",
				DestSpecPath:     "spec",
				FieldsToPreserve: []Path{{"spec", "credentials", "password"}},
			},
			expectedError: "failed to copy spec: preserved destination field could not be restored",
			sensitive:     []string{"sentinel-restore-secret", "preserved-password", "DestinationMachine", "destination-machine"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			err := CopySpec(tt.input)

			g.Expect(err).To(MatchError(tt.expectedError))
			expectNoDisclosure(g, "", err, tt.sensitive...)
		})
	}
}

func TestCopySpecPreservesFieldsAndIgnoresMissingOptionalFields(t *testing.T) {
	g := NewWithT(t)
	src := testUnstructured("SourceMachine", "source-machine", map[string]interface{}{
		"spec": map[string]interface{}{
			"credentials": map[string]interface{}{"password": "replacement-password"},
			"version":     "v2",
		},
	})
	dest := testUnstructured("DestinationMachine", "destination-machine", map[string]interface{}{
		"spec": map[string]interface{}{
			"credentials": map[string]interface{}{"password": "preserved-password"},
			"version":     "v1",
		},
	})

	err := CopySpec(CopySpecInput{
		Src:              src,
		Dest:             dest,
		SrcSpecPath:      "spec",
		DestSpecPath:     "spec",
		FieldsToPreserve: []Path{{"spec", "credentials", "password"}, {"spec", "optional", "missing"}},
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(dest.Object["spec"]).To(Equal(map[string]interface{}{
		"credentials": map[string]interface{}{"password": "preserved-password"},
		"version":     "v2",
	}))
}

func TestApplyPatchToObjectPreservesEmptyAndDeletionBehavior(t *testing.T) {
	g := NewWithT(t)

	for _, patch := range []runtimehooksv1.Patch{
		{PatchType: runtimehooksv1.JSONPatchType, Patch: []byte(`[]`)},
		{PatchType: runtimehooksv1.JSONMergePatchType},
		{PatchType: runtimehooksv1.JSONMergePatchType, Patch: []byte(`{}`)},
	} {
		obj := testRawExtension()
		original := bytes.Clone(obj.Raw)

		changed, err := ApplyPatchToObject(context.Background(), &obj, patch, "spec")

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(changed).To(BeFalse())
		g.Expect(obj.Raw).To(Equal(original))
	}

	obj := testRawExtension()
	changed, err := ApplyPatchToObject(context.Background(), &obj, runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONMergePatchType,
		Patch:     []byte(`{"spec":{"commands":null}}`),
	}, "spec")

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(changed).To(BeTrue())
	g.Expect(obj.Raw).To(MatchJSON(`{
		"apiVersion":"infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":"TestMachine",
		"metadata":{"name":"machine-1","labels":{"preserved":"true"}},
		"spec":{"version":"v1"}
	}`))
}

func expectNoDisclosure(g *WithT, logs string, err error, sensitiveValues ...string) {
	output := logs
	if err != nil {
		output += "\n" + err.Error()
	}
	for _, sensitiveValue := range sensitiveValues {
		g.Expect(output).NotTo(ContainSubstring(sensitiveValue))
	}
}

func contextWithVerboseLogger() (context.Context, func() string) {
	var logLines []string
	logger := funcr.New(func(prefix, args string) {
		logLines = append(logLines, prefix+args)
	}, funcr.Options{Verbosity: 5})
	return ctrl.LoggerInto(context.Background(), logger), func() string {
		return strings.Join(logLines, "\n")
	}
}

func testRawExtension() runtime.RawExtension {
	return runtime.RawExtension{Raw: []byte(`{
		"apiVersion":"infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":"TestMachine",
		"metadata":{"name":"machine-1","labels":{"preserved":"true"}},
		"spec":{"commands":["old-command"],"version":"v1"}
	}`)}
}

func testUnstructured(kind, name string, fields map[string]interface{}) *unstructured.Unstructured {
	object := map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name": name,
		},
	}
	for key, value := range fields {
		object[key] = value
	}
	return &unstructured.Unstructured{Object: object}
}
