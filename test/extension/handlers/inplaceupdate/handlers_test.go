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

package inplaceupdate

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	callRecordNamespace = "test-namespace"
	callRecordName      = "hook-calls"
)

func TestDoCanUpdateMachineClaimsOnlyVersionAndRecordsCall(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)
	req := canUpdateRequest("v1.34.5+k3s1")

	resp := &runtimehooksv1.CanUpdateMachineResponse{}
	h.DoCanUpdateMachine(context.Background(), req, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(decodeJSONPatch(t, resp.MachinePatch)).To(Equal([]jsonPatchOperation{{
		Operation: "replace",
		Path:      "/spec/version",
		Value:     "v1.34.5+k3s1",
	}}))
	g.Expect(resp.BootstrapConfigPatch.PatchType).To(Equal(runtimehooksv1.JSONPatchType))
	g.Expect(resp.BootstrapConfigPatch.Patch).To(MatchJSON(`[]`))
	g.Expect(resp.InfrastructureMachinePatch.PatchType).To(Equal(runtimehooksv1.JSONPatchType))
	g.Expect(resp.InfrastructureMachinePatch.Patch).To(MatchJSON(`[]`))
	g.Expect(callCount(t, c, "machine-1.canUpdateMachine")).To(Equal(1))
}

func TestDoCanUpdateMachineReturnsEmptyPatchesForIdenticalVersions(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)
	req := canUpdateRequest("v1.33.5+k3s1")

	resp := &runtimehooksv1.CanUpdateMachineResponse{}
	h.DoCanUpdateMachine(context.Background(), req, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(resp.MachinePatch.PatchType).To(Equal(runtimehooksv1.JSONPatchType))
	g.Expect(resp.MachinePatch.Patch).To(MatchJSON(`[]`))
	g.Expect(resp.BootstrapConfigPatch.Patch).To(MatchJSON(`[]`))
	g.Expect(resp.InfrastructureMachinePatch.Patch).To(MatchJSON(`[]`))
	g.Expect(callCount(t, c, "machine-1.canUpdateMachine")).To(Equal(1))
}

func TestDoCanUpdateMachineFailsForMalformedRequestObjects(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)
	req := canUpdateRequest("v1.34.5+k3s1")
	req.Current.InfrastructureMachine.Raw = []byte(`{`)

	resp := &runtimehooksv1.CanUpdateMachineResponse{}
	h.DoCanUpdateMachine(context.Background(), req, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusFailure))
	g.Expect(resp.Message).To(ContainSubstring("infrastructureMachine"))
	g.Expect(callCount(t, c, "machine-1.canUpdateMachine")).To(Equal(1))
}

func TestDoCanUpdateMachineFailsWhenCallCannotBeRecorded(t *testing.T) {
	g := NewWithT(t)
	h, _ := newHandlers(t)
	req := canUpdateRequest("v1.34.5+k3s1")
	req.Settings["callRecordConfigMap"] = "missing"

	resp := &runtimehooksv1.CanUpdateMachineResponse{}
	h.DoCanUpdateMachine(context.Background(), req, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusFailure))
	g.Expect(resp.Message).To(ContainSubstring("record CanUpdateMachine call"))
}

func TestDoCanUpdateMachineResolvesUIDFromStoredMachine(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)
	req := canUpdateRequest("v1.34.5+k3s1")
	req.Current.Machine.UID = ""
	req.Desired.Machine.UID = ""

	resp := &runtimehooksv1.CanUpdateMachineResponse{}
	h.DoCanUpdateMachine(context.Background(), req, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(callCount(t, c, "machine-1.canUpdateMachine")).To(Equal(1))
}

func TestDoUpdateMachineProgressIsDeterministicAndPersisted(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)
	req := updateRequest("machine-1")

	first := &runtimehooksv1.UpdateMachineResponse{}
	h.DoUpdateMachine(context.Background(), req, first)
	g.Expect(first.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(first.RetryAfterSeconds).To(Equal(int32(5)))
	g.Expect(callCount(t, c, "machine-1.updateMachine")).To(Equal(1))

	second := &runtimehooksv1.UpdateMachineResponse{}
	h.DoUpdateMachine(context.Background(), req, second)
	g.Expect(second.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(second.RetryAfterSeconds).To(BeZero())
	g.Expect(callCount(t, c, "machine-1.updateMachine")).To(Equal(2))
}

func TestDoUpdateMachineResolvesUIDFromStoredMachine(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)
	req := updateRequest("machine-1")
	req.Desired.Machine.UID = ""

	resp := &runtimehooksv1.UpdateMachineResponse{}
	h.DoUpdateMachine(context.Background(), req, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(callCount(t, c, "machine-1.updateMachine")).To(Equal(1))
}

func TestDoUpdateMachineTracksMachineUIDsIndependently(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)

	first := &runtimehooksv1.UpdateMachineResponse{}
	h.DoUpdateMachine(context.Background(), updateRequest("machine-1"), first)
	second := &runtimehooksv1.UpdateMachineResponse{}
	h.DoUpdateMachine(context.Background(), updateRequest("machine-2"), second)

	g.Expect(first.RetryAfterSeconds).To(Equal(int32(5)))
	g.Expect(second.RetryAfterSeconds).To(Equal(int32(5)))
	g.Expect(callCount(t, c, "machine-1.updateMachine")).To(Equal(1))
	g.Expect(callCount(t, c, "machine-2.updateMachine")).To(Equal(1))
}

func TestDoUpdateMachineFailsWhenCallCannotBeRecorded(t *testing.T) {
	g := NewWithT(t)
	h, _ := newHandlers(t)
	req := updateRequest("machine-1")
	req.Settings["callRecordConfigMap"] = "missing"

	resp := &runtimehooksv1.UpdateMachineResponse{}
	h.DoUpdateMachine(context.Background(), req, resp)

	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusFailure))
	g.Expect(resp.Message).To(ContainSubstring("record UpdateMachine call"))
}

func TestDoCanUpdateMachineDoesNotLoseConcurrentCallRecords(t *testing.T) {
	g := NewWithT(t)
	h, c := newHandlers(t)

	const calls = 10
	responses := make(chan *runtimehooksv1.CanUpdateMachineResponse, calls)
	var waitGroup sync.WaitGroup
	for range calls {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			resp := &runtimehooksv1.CanUpdateMachineResponse{}
			h.DoCanUpdateMachine(context.Background(), canUpdateRequest("v1.34.5+k3s1"), resp)
			responses <- resp
		}()
	}
	waitGroup.Wait()
	close(responses)

	for resp := range responses {
		g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	}
	g.Expect(callCount(t, c, "machine-1.canUpdateMachine")).To(Equal(calls))
}

func newHandlers(t *testing.T) (*ExtensionHandlers, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add core API to scheme: %v", err)
	}
	if err := clusterv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add Cluster API to scheme: %v", err)
	}
	storedMachine := machine("machine-1", "v1.33.5+k3s1")
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: callRecordNamespace,
				Name:      callRecordName,
			},
		}, &storedMachine).
		Build()
	return NewExtensionHandlers(c), c
}

func canUpdateRequest(desiredVersion string) *runtimehooksv1.CanUpdateMachineRequest {
	uid := types.UID("machine-1")
	return &runtimehooksv1.CanUpdateMachineRequest{
		CommonRequest: runtimehooksv1.CommonRequest{Settings: callRecordSettings()},
		Current: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine:               machine(uid, "v1.33.5+k3s1"),
			InfrastructureMachine: rawObject("DockerMachine"),
			BootstrapConfig:       rawObject("KThreesConfig"),
		},
		Desired: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine:               machine(uid, desiredVersion),
			InfrastructureMachine: rawObject("DockerMachine"),
			BootstrapConfig:       rawObject("KThreesConfig"),
		},
	}
}

func updateRequest(uid types.UID) *runtimehooksv1.UpdateMachineRequest {
	return &runtimehooksv1.UpdateMachineRequest{
		CommonRequest: runtimehooksv1.CommonRequest{Settings: callRecordSettings()},
		Desired: runtimehooksv1.UpdateMachineRequestObjects{
			Machine:               machine(uid, "v1.34.5+k3s1"),
			InfrastructureMachine: rawObject("DockerMachine"),
			BootstrapConfig:       rawObject("KThreesConfig"),
		},
	}
}

func machine(uid types.UID, version string) clusterv1.Machine {
	return clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      string(uid),
			UID:       uid,
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: "test-cluster",
			Version:     version,
		},
	}
}

func rawObject(kind string) runtime.RawExtension {
	return runtime.RawExtension{Raw: []byte(`{"apiVersion":"test.cluster.x-k8s.io/v1beta1","kind":"` + kind + `","spec":{}}`)}
}

func callRecordSettings() map[string]string {
	return map[string]string{
		"callRecordNamespace": callRecordNamespace,
		"callRecordConfigMap": callRecordName,
	}
}

func decodeJSONPatch(t *testing.T, patch runtimehooksv1.Patch) []jsonPatchOperation {
	t.Helper()
	if patch.PatchType != runtimehooksv1.JSONPatchType {
		t.Fatalf("expected JSON patch, got %q", patch.PatchType)
	}
	operations := []jsonPatchOperation{}
	if err := json.Unmarshal(patch.Patch, &operations); err != nil {
		t.Fatalf("failed to decode JSON patch: %v", err)
	}
	return operations
}

func callCount(t *testing.T, c client.Client, key string) int {
	t.Helper()
	configMap := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Namespace: callRecordNamespace,
		Name:      callRecordName,
	}, configMap); err != nil {
		t.Fatalf("failed to get call record ConfigMap: %v", err)
	}
	count, err := strconv.Atoi(configMap.Data[key])
	if err != nil {
		t.Fatalf("failed to parse call count %q: %v", configMap.Data[key], err)
	}
	return count
}
