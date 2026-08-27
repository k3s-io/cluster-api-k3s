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
