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
