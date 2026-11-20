/*
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
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta3"
)

func machineWithConditions(name string, conds ...metav1.Condition) *clusterv1.Machine {
	m := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
		},
	}
	for _, c := range conds {
		conditions.Set(m, c)
	}
	return m
}

func TestSetReplicas(t *testing.T) {
	g := NewWithT(t)

	kcp := &controlplanev1.KThreesControlPlane{}
	machines := collections.FromMachines(
		machineWithConditions("m1",
			metav1.Condition{Type: clusterv1.MachineReadyCondition, Status: metav1.ConditionTrue, Reason: "Ready"},
			metav1.Condition{Type: clusterv1.MachineAvailableCondition, Status: metav1.ConditionTrue, Reason: "Available"},
			metav1.Condition{Type: clusterv1.MachineUpToDateCondition, Status: metav1.ConditionTrue, Reason: "UpToDate"},
		),
		machineWithConditions("m2",
			metav1.Condition{Type: clusterv1.MachineReadyCondition, Status: metav1.ConditionTrue, Reason: "Ready"},
			metav1.Condition{Type: clusterv1.MachineAvailableCondition, Status: metav1.ConditionFalse, Reason: "NotAvailable"},
			metav1.Condition{Type: clusterv1.MachineUpToDateCondition, Status: metav1.ConditionFalse, Reason: "NotUpToDate"},
		),
		machineWithConditions("m3"),
	)

	setReplicas(kcp, machines)

	g.Expect(kcp.Status.Replicas).To(Equal(ptr.To(int32(3))))
	g.Expect(kcp.Status.ReadyReplicas).To(Equal(ptr.To(int32(2))))
	g.Expect(kcp.Status.AvailableReplicas).To(Equal(ptr.To(int32(1))))
	g.Expect(kcp.Status.UpToDateReplicas).To(Equal(ptr.To(int32(1))))
}

func TestSetAvailableCondition(t *testing.T) {
	tests := []struct {
		name           string
		initialized    bool
		deleting       bool
		existing       *metav1.Condition
		readyNodes     *int32
		expectedStatus metav1.ConditionStatus
	}{
		{
			name:           "not available if not initialized",
			readyNodes:     ptr.To(int32(1)),
			expectedStatus: metav1.ConditionFalse,
		},
		{
			name:           "not available if deleting",
			initialized:    true,
			deleting:       true,
			readyNodes:     ptr.To(int32(1)),
			expectedStatus: metav1.ConditionFalse,
		},
		{
			name:           "not available if there are no ready nodes",
			initialized:    true,
			readyNodes:     ptr.To(int32(0)),
			expectedStatus: metav1.ConditionFalse,
		},
		{
			name:           "available if initialized and there are ready nodes",
			initialized:    true,
			readyNodes:     ptr.To(int32(2)),
			expectedStatus: metav1.ConditionTrue,
		},
		{
			name:           "unknown if the workload cluster can't be read and the condition is not set yet",
			initialized:    true,
			expectedStatus: metav1.ConditionUnknown,
		},
		{
			name:        "preserve the existing condition if the workload cluster can't be read",
			initialized: true,
			existing: &metav1.Condition{
				Type:   controlplanev1.KThreesControlPlaneAvailableCondition,
				Status: metav1.ConditionTrue,
				Reason: controlplanev1.KThreesControlPlaneAvailableReason,
			},
			expectedStatus: metav1.ConditionTrue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			kcp := &controlplanev1.KThreesControlPlane{}
			if tt.initialized {
				kcp.Status.Initialization.ControlPlaneInitialized = ptr.To(true)
			}
			if tt.deleting {
				kcp.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			}
			if tt.existing != nil {
				conditions.Set(kcp, *tt.existing)
			}

			setAvailableCondition(kcp, tt.readyNodes)

			c := conditions.Get(kcp, controlplanev1.KThreesControlPlaneAvailableCondition)
			g.Expect(c).ToNot(BeNil())
			g.Expect(c.Status).To(Equal(tt.expectedStatus))
		})
	}
}

func TestSetInitializedCondition(t *testing.T) {
	g := NewWithT(t)

	kcp := &controlplanev1.KThreesControlPlane{}
	setInitializedCondition(kcp)
	g.Expect(conditions.IsFalse(kcp, controlplanev1.KThreesControlPlaneInitializedCondition)).To(BeTrue())

	kcp.Status.Initialization.ControlPlaneInitialized = ptr.To(true)
	setInitializedCondition(kcp)
	g.Expect(conditions.IsTrue(kcp, controlplanev1.KThreesControlPlaneInitializedCondition)).To(BeTrue())
}

func TestSetScalingConditions(t *testing.T) {
	tests := []struct {
		name                string
		replicas            *int32
		machines            int
		expectedScalingUp   metav1.ConditionStatus
		expectedScalingDown metav1.ConditionStatus
	}{
		{
			name:                "unknown if replicas is not set",
			machines:            1,
			expectedScalingUp:   metav1.ConditionUnknown,
			expectedScalingDown: metav1.ConditionUnknown,
		},
		{
			name:                "scaling up",
			replicas:            ptr.To(int32(3)),
			machines:            1,
			expectedScalingUp:   metav1.ConditionTrue,
			expectedScalingDown: metav1.ConditionFalse,
		},
		{
			name:                "scaling down",
			replicas:            ptr.To(int32(1)),
			machines:            3,
			expectedScalingUp:   metav1.ConditionFalse,
			expectedScalingDown: metav1.ConditionTrue,
		},
		{
			name:                "not scaling",
			replicas:            ptr.To(int32(3)),
			machines:            3,
			expectedScalingUp:   metav1.ConditionFalse,
			expectedScalingDown: metav1.ConditionFalse,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			kcp := &controlplanev1.KThreesControlPlane{Spec: controlplanev1.KThreesControlPlaneSpec{Replicas: tt.replicas}}
			machines := collections.New()
			for i := range tt.machines {
				machines.Insert(machineWithConditions(string(rune('a' + i))))
			}

			setScalingUpCondition(kcp, machines)
			setScalingDownCondition(kcp, machines)

			g.Expect(conditions.Get(kcp, controlplanev1.KThreesControlPlaneScalingUpCondition).Status).To(Equal(tt.expectedScalingUp))
			g.Expect(conditions.Get(kcp, controlplanev1.KThreesControlPlaneScalingDownCondition).Status).To(Equal(tt.expectedScalingDown))
		})
	}
}

func TestSetV1Beta1ResizedCondition(t *testing.T) {
	g := NewWithT(t)

	kcp := &controlplanev1.KThreesControlPlane{Spec: controlplanev1.KThreesControlPlaneSpec{Replicas: ptr.To(int32(3))}}
	setV1Beta1ResizedCondition(kcp, collections.FromMachines(machineWithConditions("m1")))

	// The deprecated v1beta1 condition is stored in status.deprecated.v1beta1.conditions.
	g.Expect(v1beta1conditions.IsFalse(kcp, controlplanev1.ResizedV1Beta1Condition)).To(BeTrue())
	g.Expect(v1beta1conditions.GetReason(kcp, controlplanev1.ResizedV1Beta1Condition)).To(Equal(controlplanev1.ScalingUpV1Beta1Reason))
	g.Expect(kcp.Status.Deprecated.V1Beta1.Conditions).To(HaveLen(1))
	g.Expect(kcp.Status.Conditions).To(BeEmpty())
}
