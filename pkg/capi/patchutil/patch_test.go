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

	g.Expect(err).To(MatchError(ContainSubstring("failed to apply patch: error decoding json patch (RFC6902)")))
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

	g.Expect(err).To(MatchError(ContainSubstring("failed to apply patch: error applying json patch (RFC6902)")))
	g.Expect(changed).To(BeFalse())
	g.Expect(obj.Raw).To(Equal(original))
	output := logs() + "\n" + err.Error()
	g.Expect(output).To(ContainSubstring("JSONPatch"))
	g.Expect(output).To(ContainSubstring(`"operationCount"=1`))
	g.Expect(output).NotTo(ContainSubstring(sentinel))
	g.Expect(output).NotTo(ContainSubstring(string(patchBody)))
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
