package controllers

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

func TestComputeDesiredMachineCopiesMachineTemplateTaints(t *testing.T) {
	taints := []clusterv1.MachineTaint{{
		Key:         "node-role.kubernetes.io/control-plane",
		Effect:      corev1.TaintEffectNoSchedule,
		Propagation: clusterv1.MachineTaintPropagationAlways,
	}}
	kcp := &controlplanev1.KThreesControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "control-plane",
			Namespace: "default",
		},
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Version: "v1.35.0",
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				Spec: controlplanev1.KThreesControlPlaneMachineTemplateSpec{
					Taints: taints,
				},
			},
		},
	}
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	for _, tt := range []struct {
		name            string
		existingMachine *clusterv1.Machine
	}{
		{name: "new machine"},
		{
			name: "existing machine",
			existingMachine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "existing"},
				Spec:       clusterv1.MachineSpec{Version: "v1.35.0"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			machine, err := (&KThreesControlPlaneReconciler{}).computeDesiredMachine(
				kcp,
				cluster,
				"failure-domain",
				tt.existingMachine,
			)
			if err != nil {
				t.Fatalf("computeDesiredMachine() error = %v", err)
			}
			if !reflect.DeepEqual(machine.Spec.Taints, taints) {
				t.Errorf("machine taints = %#v, want %#v", machine.Spec.Taints, taints)
			}
		})
	}
}
