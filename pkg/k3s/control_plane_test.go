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
package k3s

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta3"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta3"
)

func TestMachineNotUpToDateReasons(t *testing.T) {
	now := time.Now()
	kcp := &controlplanev1.KThreesControlPlane{
		Spec: controlplanev1.KThreesControlPlaneSpec{
			Version: "v1.33.1+k3s1",
			MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
				Spec: controlplanev1.KThreesControlPlaneMachineTemplateSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "infrastructure.cluster.x-k8s.io",
						Kind:     "DockerMachineTemplate",
						Name:     "infra",
					},
				},
			},
		},
	}
	machine := func(name, version string, created time.Time) *clusterv1.Machine {
		return &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.Time{Time: created}},
			Spec: clusterv1.MachineSpec{
				Version:   version,
				Bootstrap: clusterv1.Bootstrap{ConfigRef: clusterv1.ContractVersionedObjectReference{APIGroup: bootstrapv1.GroupVersion.Group, Kind: "KThreesConfig", Name: name}},
			},
		}
	}

	upToDate := machine("up-to-date", "v1.33.1+k3s1", now.Add(-time.Hour))
	oldVersion := machine("old-version", "v1.32.0+k3s1", now.Add(-time.Hour))
	controlPlane := &ControlPlane{
		KCP: kcp,
		KthreesConfigs: map[string]*bootstrapv1.KThreesConfig{
			"up-to-date":  {Spec: kcp.Spec.KThreesConfigSpec},
			"old-version": {Spec: kcp.Spec.KThreesConfigSpec},
		},
		InfraResources:     map[string]*unstructured.Unstructured{},
		reconciliationTime: metav1.Time{Time: now},
	}

	t.Run("up-to-date machine", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(controlPlane.MachineNotUpToDateReasons(upToDate)).To(BeEmpty())
	})
	t.Run("machine with an old version", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(controlPlane.MachineNotUpToDateReasons(oldVersion)).To(ConsistOf("Version v1.32.0+k3s1, v1.33.1+k3s1 required"))
	})
	t.Run("machine with an outdated KThreesConfig", func(t *testing.T) {
		g := NewWithT(t)
		changed := kcp.Spec.KThreesConfigSpec.DeepCopy()
		changed.PreK3sCommands = []string{"echo changed"}
		cp := *controlPlane
		cp.KthreesConfigs = map[string]*bootstrapv1.KThreesConfig{"up-to-date": {Spec: *changed}}
		g.Expect(cp.MachineNotUpToDateReasons(upToDate)).To(ConsistOf("KThreesConfig is not up-to-date"))
	})
	t.Run("machine created before rollout.after expired", func(t *testing.T) {
		g := NewWithT(t)
		cp := *controlPlane
		cp.KCP = kcp.DeepCopy()
		cp.KCP.Spec.Rollout.After = metav1.Time{Time: now.Add(-time.Minute)}
		g.Expect(cp.MachineNotUpToDateReasons(upToDate)).To(ConsistOf("KThreesControlPlane spec.rollout.after expired"))
	})
}
