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

package k3s

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/util/collections"

	"github.com/k3s-io/cluster-api-k3s/pkg/capi/hooks"
)

func TestInPlaceProgressSelectors(t *testing.T) {
	tests := []struct {
		name                string
		mutate              func(*clusterv1.Machine)
		wantCompleteTrigger bool
		wantCompleteUpdate  bool
	}{
		{name: "no annotation"},
		{
			name: "partial trigger",
			mutate: func(machine *clusterv1.Machine) {
				machine.Annotations[clusterv1.UpdateInProgressAnnotation] = ""
			},
			wantCompleteTrigger: true,
			wantCompleteUpdate:  true,
		},
		{
			name: "pending hook",
			mutate: func(machine *clusterv1.Machine) {
				machine.Annotations[clusterv1.UpdateInProgressAnnotation] = ""
				hooks.MarkObjectAsPending(machine, runtimehooksv1.UpdateMachine)
			},
			wantCompleteUpdate: true,
		},
		{
			name: "cleanup pending after annotation removal",
			mutate: func(machine *clusterv1.Machine) {
				hooks.MarkObjectAsPending(machine, runtimehooksv1.UpdateMachine)
			},
			wantCompleteUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			machine := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "machine-1", Annotations: map[string]string{}}}
			if tt.mutate != nil {
				tt.mutate(machine)
			}
			controlPlane := &ControlPlane{Machines: collections.FromMachines(machine)}
			g.Expect(controlPlane.MachinesToCompleteTriggerInPlaceUpdate().Has(machine)).To(Equal(tt.wantCompleteTrigger))
			g.Expect(controlPlane.MachinesToCompleteInPlaceUpdate().Has(machine)).To(Equal(tt.wantCompleteUpdate))
		})
	}
}

func TestMachinesNeedingRolloutReturnsCachedResults(t *testing.T) {
	g := NewWithT(t)
	outdated := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}
	upToDate := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "up-to-date"}}
	result := UpToDateResult{ConditionMessages: []string{"outdated"}}
	controlPlane := &ControlPlane{
		Machines:                collections.FromMachines(outdated, upToDate),
		machinesNotUpToDate:     collections.FromMachines(outdated),
		machinesUpToDateResults: map[string]UpToDateResult{outdated.Name: result, upToDate.Name: {}},
	}

	machines, results := controlPlane.MachinesNeedingRollout()
	g.Expect(machines.Names()).To(ConsistOf(outdated.Name))
	g.Expect(results).To(HaveKeyWithValue(outdated.Name, result))
	g.Expect(controlPlane.UpToDateMachines().Names()).To(ConsistOf(upToDate.Name))
}
