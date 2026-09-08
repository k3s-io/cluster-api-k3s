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

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
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

func TestMachinesNeedingRolloutCompatibilityAndDetailedResults(t *testing.T) {
	g := NewWithT(t)
	outdated := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}
	upToDate := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "up-to-date"}}
	result := UpToDateResult{ConditionMessages: []string{"outdated"}}
	controlPlane := &ControlPlane{
		Machines:                collections.FromMachines(outdated, upToDate),
		machinesNotUpToDate:     collections.FromMachines(outdated),
		machinesUpToDateResults: map[string]UpToDateResult{outdated.Name: result, upToDate.Name: {}},
	}

	machines := controlPlane.MachinesNeedingRollout()
	machinesWithResults, results := controlPlane.MachinesNeedingRolloutWithResults()
	g.Expect(machines.Names()).To(ConsistOf(outdated.Name))
	g.Expect(machinesWithResults).To(Equal(machines))
	g.Expect(results).To(HaveKeyWithValue(outdated.Name, result))
	g.Expect(controlPlane.UpToDateMachines().Names()).To(ConsistOf(upToDate.Name))
}

func TestInitializedEmptyRolloutCaches(t *testing.T) {
	g := NewWithT(t)
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: "outdated-by-legacy-filter"},
		Spec:       clusterv1.MachineSpec{Version: "v1.30.0+k3s1"},
	}
	controlPlane := &ControlPlane{
		KCP: &controlplanev1.KThreesControlPlane{
			Spec: controlplanev1.KThreesControlPlaneSpec{Version: "v1.31.0+k3s1"},
		},
		Machines:                collections.FromMachines(machine),
		machinesNotUpToDate:     collections.Machines{},
		machinesUpToDateResults: map[string]UpToDateResult{},
	}

	g.Expect(controlPlane.MachinesNeedingRollout()).To(BeEmpty())
	notUpToDate, results := controlPlane.NotUpToDateMachines()
	g.Expect(notUpToDate).To(BeEmpty())
	g.Expect(results).To(BeEmpty())
	g.Expect(controlPlane.UpToDateMachines().Names()).To(ConsistOf(machine.Name))
}

func TestMarkInPlaceUpdateUnsupportedMutatesOnlyNamedCachedResult(t *testing.T) {
	g := NewWithT(t)
	machine1 := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "machine-1"}}
	machine2 := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "machine-2"}}
	controlPlane := &ControlPlane{
		Machines:            collections.FromMachines(machine1, machine2),
		machinesNotUpToDate: collections.FromMachines(machine1, machine2),
		machinesUpToDateResults: map[string]UpToDateResult{
			machine1.Name: {
				LogMessages:              []string{"machine-1 existing log"},
				ConditionMessages:        []string{"machine-1 existing condition"},
				EligibleForInPlaceUpdate: true,
			},
			machine2.Name: {
				LogMessages:              []string{"machine-2 existing log"},
				ConditionMessages:        []string{"machine-2 existing condition"},
				EligibleForInPlaceUpdate: true,
			},
		},
	}

	const (
		logMessage       = "related-object spec ownership spans multiple API versions"
		conditionMessage = "Related-object ownership requires Machine replacement"
	)
	controlPlane.MarkInPlaceUpdateUnsupported(machine1.Name, logMessage, conditionMessage)
	controlPlane.MarkInPlaceUpdateUnsupported(machine1.Name, logMessage, conditionMessage)
	controlPlane.MarkInPlaceUpdateUnsupported("unknown-machine", logMessage, conditionMessage)

	_, results := controlPlane.MachinesNeedingRolloutWithResults()
	g.Expect(results[machine1.Name].EligibleForInPlaceUpdate).To(BeFalse())
	g.Expect(results[machine1.Name].LogMessages).To(Equal([]string{"machine-1 existing log", logMessage}))
	g.Expect(results[machine1.Name].ConditionMessages).To(Equal([]string{"machine-1 existing condition", conditionMessage}))
	g.Expect(results[machine2.Name].EligibleForInPlaceUpdate).To(BeTrue())
	g.Expect(results[machine2.Name].LogMessages).To(Equal([]string{"machine-2 existing log"}))
	g.Expect(results[machine2.Name].ConditionMessages).To(Equal([]string{"machine-2 existing condition"}))
	g.Expect(controlPlane.MachinesNeedingRollout().Names()).To(ConsistOf(machine1.Name, machine2.Name))
}

func TestReplaceMachineUpdatesCachedCollectionsWithoutChangingMembership(t *testing.T) {
	g := NewWithT(t)
	outdated := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "outdated", ResourceVersion: "1"}}
	otherOutdated := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "other-outdated", ResourceVersion: "1"}}
	upToDate := &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Name: "up-to-date", ResourceVersion: "1"}}
	controlPlane := &ControlPlane{
		Machines:                collections.FromMachines(outdated, otherOutdated, upToDate),
		machinesNotUpToDate:     collections.FromMachines(outdated, otherOutdated),
		machinesUpToDateResults: map[string]UpToDateResult{},
	}

	updatedOutdated := outdated.DeepCopy()
	updatedOutdated.ResourceVersion = "2"
	updatedOutdated.Labels = map[string]string{"updated": "true"}
	controlPlane.ReplaceMachine(updatedOutdated)

	machinesNeedingRollout, _ := controlPlane.MachinesNeedingRolloutWithResults()
	g.Expect(controlPlane.Machines[outdated.Name]).To(BeIdenticalTo(updatedOutdated))
	g.Expect(machinesNeedingRollout[outdated.Name]).To(BeIdenticalTo(updatedOutdated))
	g.Expect(controlPlane.Machines[otherOutdated.Name]).To(BeIdenticalTo(otherOutdated))
	g.Expect(machinesNeedingRollout[otherOutdated.Name]).To(BeIdenticalTo(otherOutdated))

	updatedUpToDate := upToDate.DeepCopy()
	updatedUpToDate.ResourceVersion = "2"
	controlPlane.ReplaceMachine(updatedUpToDate)

	machinesNeedingRollout, _ = controlPlane.MachinesNeedingRolloutWithResults()
	g.Expect(controlPlane.Machines[upToDate.Name]).To(BeIdenticalTo(updatedUpToDate))
	g.Expect(machinesNeedingRollout).NotTo(HaveKey(upToDate.Name))
}
