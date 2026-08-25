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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

func TestTryInPlaceUpdatePreflightFallback(t *testing.T) {
	tests := []struct {
		name             string
		unhealthyMachine string
		wantFallback     bool
		wantWait         bool
	}{
		{
			name:             "selected Machine issue falls back to scale down",
			unhealthyMachine: "selected",
			wantFallback:     true,
		},
		{
			name:             "another Machine issue waits",
			unhealthyMachine: "other",
			wantWait:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			selected := healthyMachine("selected")
			other := healthyMachine("other")
			conditions.Set(collections.FromMachines(selected, other)[tt.unhealthyMachine], metav1.Condition{
				Type:   controlplanev1.MachineAgentHealthyV1Beta2Condition,
				Status: metav1.ConditionFalse,
				Reason: "Unhealthy",
			})
			controlPlane := &k3s.ControlPlane{
				KCP:      &controlplanev1.KThreesControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "kcp", Namespace: "default"}},
				Cluster:  &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"}},
				Machines: collections.FromMachines(selected, other),
			}
			canUpdateCalled := false
			r := &KThreesControlPlaneReconciler{
				recorder: record.NewFakeRecorder(10),
				overrideCanUpdateMachine: func(context.Context, *clusterv1.Machine, k3s.UpToDateResult) (bool, error) {
					canUpdateCalled = true
					return true, nil
				},
				overrideTriggerInPlaceUpdate: func(context.Context, *clusterv1.Machine, k3s.UpToDateResult) error {
					return nil
				},
			}

			fallback, result, err := r.tryInPlaceUpdate(context.Background(), controlPlane, selected, k3s.UpToDateResult{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(fallback).To(Equal(tt.wantFallback))
			g.Expect(result.IsZero()).To(Equal(!tt.wantWait))
			g.Expect(canUpdateCalled).To(BeFalse())
		})
	}
}

func healthyMachine(name string) *clusterv1.Machine {
	machine := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: name}}
	conditions.Set(machine, metav1.Condition{
		Type:   controlplanev1.MachineAgentHealthyV1Beta2Condition,
		Status: metav1.ConditionTrue,
		Reason: "Healthy",
	})
	return machine
}
