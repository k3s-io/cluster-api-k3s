//go:build e2e
// +build e2e

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

package e2e

import (
	"errors"
	"reflect"
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	runtimev1 "sigs.k8s.io/cluster-api/api/runtime/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

func TestRunInPlaceAfterEachCleansUpAfterEvidenceFailure(t *testing.T) {
	cleanupCalls := []string{}
	assertion := gomega.NewGomega(func(message string, _ ...int) {
		panic(message)
	})

	var failure any
	func() {
		defer func() {
			failure = recover()
		}()
		runInPlaceAfterEach(
			func() {
				assertion.Expect(errors.New("call-record ConfigMap unavailable")).NotTo(gomega.HaveOccurred())
			},
			func() {
				cleanupCalls = append(cleanupCalls, "extension config")
			},
			func() {
				cleanupCalls = append(cleanupCalls, "cluster namespace and watches")
			},
		)
	}()

	if failure == nil {
		t.Fatal("expected evidence collection to fail")
	}
	want := []string{"extension config", "cluster namespace and watches"}
	if !reflect.DeepEqual(cleanupCalls, want) {
		t.Fatalf("cleanup calls = %#v, want %#v", cleanupCalls, want)
	}
}

func TestInPlaceExtensionConfigsAreUniqueAndDisjoint(t *testing.T) {
	namespaces := []string{"scenario-1", "scenario-2"}
	configs := []*runtimev1.ExtensionConfig{
		inPlaceExtensionConfig(inPlaceExtensionConfigScenario1, namespaces[0]),
		inPlaceExtensionConfig(inPlaceExtensionConfigScenario2, namespaces[1]),
	}

	if configs[0].Name == configs[1].Name {
		t.Fatalf("ExtensionConfig names must be unique, both were %q", configs[0].Name)
	}
	for i, config := range configs {
		if errs := validation.IsDNS1123Subdomain(config.Name); len(errs) > 0 {
			t.Fatalf("config %d has unsafe name %q: %v", i, config.Name, errs)
		}
		wantNamespace := namespaces[i]
		selector, err := metav1.LabelSelectorAsSelector(config.Spec.NamespaceSelector)
		if err != nil {
			t.Fatalf("config %d selector is invalid: %v", i, err)
		}
		if !selector.Matches(labels.Set{"kubernetes.io/metadata.name": wantNamespace}) {
			t.Fatalf("config %d selector does not match %q", i, wantNamespace)
		}
		otherNamespace := namespaces[1-i]
		if selector.Matches(labels.Set{"kubernetes.io/metadata.name": otherNamespace}) {
			t.Fatalf("config %d selector also matches %q", i, otherNamespace)
		}
		if got := config.Spec.Settings["callRecordNamespace"]; got != wantNamespace {
			t.Fatalf("config %d callRecordNamespace = %q, want %q", i, got, wantNamespace)
		}
		if got := config.Spec.Settings["callRecordConfigMap"]; got != inPlaceCallRecordConfigMap {
			t.Fatalf("config %d callRecordConfigMap = %q, want %q", i, got, inPlaceCallRecordConfigMap)
		}
	}
}

func TestExtensionConfigDiscoveredForCurrentGeneration(t *testing.T) {
	config := inPlaceExtensionConfig(inPlaceExtensionConfigScenario1, "scenario-1")
	config.Generation = 2
	config.Status.Conditions = []metav1.Condition{{
		Type:               runtimev1.ExtensionConfigDiscoveredCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: 1,
	}}

	if extensionConfigDiscoveredForCurrentGeneration(config) {
		t.Fatal("stale Discovered=True condition must not be accepted")
	}
	config.Status.Conditions[0].ObservedGeneration = config.Generation
	if !extensionConfigDiscoveredForCurrentGeneration(config) {
		t.Fatal("current Discovered=True condition should be accepted")
	}
	config.Status.Conditions[0].Status = metav1.ConditionFalse
	if extensionConfigDiscoveredForCurrentGeneration(config) {
		t.Fatal("current Discovered=False condition must not be accepted")
	}
}

func TestShouldUseDockerCLIImageLoader(t *testing.T) {
	tests := []struct {
		name         string
		socketExists bool
		explicit     bool
		osRelease    string
		want         bool
	}{
		{name: "native socket remains on normal path", socketExists: true, explicit: true},
		{name: "native Linux without socket remains on normal path"},
		{name: "WSL without socket uses fallback", osRelease: "5.15.153.1-microsoft-standard-WSL2", want: true},
		{name: "explicit opt-in without socket uses fallback", explicit: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseDockerCLIImageLoader(tt.socketExists, tt.explicit, tt.osRelease); got != tt.want {
				t.Fatalf("got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestSaveImageWithDockerCLINativePath(t *testing.T) {
	var calls [][]string
	run := func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte("saved"), nil
	}

	output, err := saveImageWithDockerCLI("image:dev", "/work/image.tar", run)
	if err != nil {
		t.Fatalf("saveImageWithDockerCLI returned error: %v", err)
	}
	if string(output) != "saved" {
		t.Fatalf("unexpected output %q", output)
	}
	want := [][]string{{"docker", "save", "--output", "/work/image.tar", "image:dev"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected calls: got %#v, want %#v", calls, want)
	}
}

func TestSaveImageWithDockerCLIWindowsPathRetry(t *testing.T) {
	var calls [][]string
	run := func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		switch len(calls) {
		case 1:
			return nil, errors.New("invalid output path")
		case 2:
			return []byte("C:\\work\\image.tar\r\n"), nil
		default:
			return []byte("saved"), nil
		}
	}

	output, err := saveImageWithDockerCLI("image:dev", "/work/image.tar", run)
	if err != nil {
		t.Fatalf("saveImageWithDockerCLI returned error: %v", err)
	}
	if string(output) != "saved" {
		t.Fatalf("unexpected output %q", output)
	}
	want := [][]string{
		{"docker", "save", "--output", "/work/image.tar", "image:dev"},
		{"wslpath", "-w", "/work/image.tar"},
		{"docker", "save", "--output", "C:\\work\\image.tar", "image:dev"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected calls: got %#v, want %#v", calls, want)
	}
}

func TestImageLoadFailure(t *testing.T) {
	loadErr := errors.New("load failed")
	if err := imageLoadFailure(clusterctl.MustLoadImage, loadErr); !errors.Is(err, loadErr) {
		t.Fatalf("mustLoad error = %v, want %v", err, loadErr)
	}
	if err := imageLoadFailure(clusterctl.TryLoadImage, loadErr); err != nil {
		t.Fatalf("tryLoad error = %v, want nil", err)
	}
}

func TestObserveMachineCardinality(t *testing.T) {
	evidence := &inPlaceScenarioEvidence{}
	for _, observed := range []int{1, 3, 2} {
		if err := observeMachineCardinality(evidence, observed, 3); err != nil {
			t.Fatalf("observeMachineCardinality(%d) returned error: %v", observed, err)
		}
	}
	if evidence.MaxControlPlaneMachineCardinality != 3 {
		t.Fatalf("max cardinality = %d, want 3", evidence.MaxControlPlaneMachineCardinality)
	}

	if err := observeMachineCardinality(evidence, 4, 3); err == nil {
		t.Fatal("expected cardinality limit error")
	}
	if evidence.MaxControlPlaneMachineCardinality != 4 {
		t.Fatalf("max cardinality after violation = %d, want 4", evidence.MaxControlPlaneMachineCardinality)
	}
}
