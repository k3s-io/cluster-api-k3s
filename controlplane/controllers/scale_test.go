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
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
)

func TestSelectMachineForInPlaceUpdateOrScaleDownPrioritizesUnhealthyComponents(t *testing.T) {
	g := NewWithT(t)
	healthy := rolloutMachine("healthy", 1)
	unhealthy := rolloutMachine("unhealthy", 2)
	conditions.Set(unhealthy, metav1.Condition{
		Type:   controlplanev1.MachineAgentHealthyV1Beta2Condition,
		Status: metav1.ConditionFalse,
		Reason: "Unhealthy",
	})
	controlPlane := &k3s.ControlPlane{
		KCP:      &controlplanev1.KThreesControlPlane{},
		Cluster:  &clusterv1.Cluster{},
		Machines: collections.FromMachines(healthy, unhealthy),
	}

	selected, err := selectMachineForInPlaceUpdateOrScaleDown(
		context.Background(), controlPlane, collections.FromMachines(healthy, unhealthy),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(selected.Name).To(Equal(unhealthy.Name))
}

func TestSelectMachineForInPlaceUpdateOrScaleDownPrioritizesUnhealthyEtcdMember(t *testing.T) {
	g := NewWithT(t)
	controlPlane, _, _, outdated, results, c := newRolloutControlPlane(t, 2, 2, 0, []int{0, 1}, true)
	g.Expect(results).To(HaveLen(2))
	g.Expect(c).NotTo(BeNil())
	healthy := controlPlane.Machines["machine-0"]
	unhealthy := controlPlane.Machines["machine-1"]
	conditions.Set(unhealthy, metav1.Condition{
		Type:   controlplanev1.MachineEtcdMemberHealthyV1Beta2Condition,
		Status: metav1.ConditionFalse,
		Reason: "Unhealthy",
	})

	selected, err := selectMachineForInPlaceUpdateOrScaleDown(context.Background(), controlPlane, outdated)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(selected.Name).To(Equal(unhealthy.Name))
	g.Expect(selected.Name).NotTo(Equal(healthy.Name))
}

func TestSelectMachineForInPlaceUpdateOrScaleDownPriority(t *testing.T) {
	tests := []struct {
		name      string
		machines  []*clusterv1.Machine
		outdated  []string
		want      string
		mutate    func(map[string]*clusterv1.Machine)
		configure func(*k3s.ControlPlane)
	}{
		{
			name:     "outdated Machine with delete annotation",
			machines: []*clusterv1.Machine{rolloutMachine("outdated-annotated", 2), rolloutMachine("outdated", 1), rolloutMachine("current-annotated", 0)},
			outdated: []string{"outdated-annotated", "outdated"},
			want:     "outdated-annotated",
			mutate: func(machines map[string]*clusterv1.Machine) {
				machines["outdated-annotated"].Annotations = map[string]string{clusterv1beta1.DeleteMachineAnnotation: ""}
				machines["current-annotated"].Annotations = map[string]string{clusterv1beta1.DeleteMachineAnnotation: ""}
			},
		},
		{
			name:     "any Machine with delete annotation before remaining outdated Machines",
			machines: []*clusterv1.Machine{rolloutMachine("outdated", 0), rolloutMachine("current-annotated", 1)},
			outdated: []string{"outdated"},
			want:     "current-annotated",
			mutate: func(machines map[string]*clusterv1.Machine) {
				machines["current-annotated"].Annotations = map[string]string{clusterv1beta1.DeleteMachineAnnotation: ""}
			},
		},
		{
			name:     "oldest remaining outdated Machine",
			machines: []*clusterv1.Machine{rolloutMachine("oldest-outdated", 0), rolloutMachine("newest-outdated", 1), rolloutMachine("current", 2)},
			outdated: []string{"oldest-outdated", "newest-outdated"},
			want:     "oldest-outdated",
		},
		{
			name:     "oldest Machine when no outdated Machine remains",
			machines: []*clusterv1.Machine{rolloutMachine("oldest", 0), rolloutMachine("newest", 1)},
			want:     "oldest",
		},
		{
			name:     "oldest candidate in the failure domain with most Machines",
			machines: []*clusterv1.Machine{rolloutMachine("zone-a-oldest", 0), rolloutMachine("zone-a-newest", 2), rolloutMachine("zone-b-oldest", 1)},
			outdated: []string{"zone-a-oldest", "zone-a-newest", "zone-b-oldest"},
			want:     "zone-a-oldest",
			mutate: func(machines map[string]*clusterv1.Machine) {
				machines["zone-a-oldest"].Spec.FailureDomain = "zone-a"
				machines["zone-a-newest"].Spec.FailureDomain = "zone-a"
				machines["zone-b-oldest"].Spec.FailureDomain = "zone-b"
			},
			configure: func(controlPlane *k3s.ControlPlane) {
				controlPlane.Cluster.Status.FailureDomains = []clusterv1.FailureDomain{
					{Name: "zone-a", ControlPlane: ptr.To(true)},
					{Name: "zone-b", ControlPlane: ptr.To(true)},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			machineMap := map[string]*clusterv1.Machine{}
			for _, machine := range tt.machines {
				machineMap[machine.Name] = machine
			}
			if tt.mutate != nil {
				tt.mutate(machineMap)
			}
			controlPlane := &k3s.ControlPlane{
				KCP:      &controlplanev1.KThreesControlPlane{},
				Cluster:  &clusterv1.Cluster{},
				Machines: collections.FromMachines(tt.machines...),
			}
			if tt.configure != nil {
				tt.configure(controlPlane)
			}
			outdated := collections.Machines{}
			for _, name := range tt.outdated {
				outdated[name] = machineMap[name]
			}

			selected, err := selectMachineForInPlaceUpdateOrScaleDown(context.Background(), controlPlane, outdated)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(selected.Name).To(Equal(tt.want))
		})
	}
}

func rolloutMachine(name string, minute int) *clusterv1.Machine {
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(timeForRolloutSelection(minute)),
		},
	}
}

func timeForRolloutSelection(minute int) time.Time {
	return time.Date(2026, 8, 25, 10, minute, 0, 0, time.UTC)
}
