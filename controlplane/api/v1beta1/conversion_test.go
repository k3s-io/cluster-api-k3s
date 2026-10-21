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
package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	bootstrapv1beta2 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(bootstrapv1beta2.AddToScheme(scheme)).To(Succeed())

	t.Run("for KThreesControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &bootstrapv1beta2.KThreesControlPlane{},
		Spoke:       &KThreesControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
}

func TestConversionPreservesMachineTemplateTaints(t *testing.T) {
	g := NewWithT(t)
	taints := []clusterv1.MachineTaint{{
		Key:         "node-role.kubernetes.io/control-plane",
		Effect:      corev1.TaintEffectNoSchedule,
		Propagation: clusterv1.MachineTaintPropagationAlways,
	}}
	hub := &bootstrapv1beta2.KThreesControlPlane{}
	hub.Spec.MachineTemplate.Spec.Taints = taints

	spoke := &KThreesControlPlane{}
	g.Expect(spoke.ConvertFrom(hub)).To(Succeed())

	roundTripped := &bootstrapv1beta2.KThreesControlPlane{}
	g.Expect(spoke.ConvertTo(roundTripped)).To(Succeed())
	g.Expect(roundTripped.Spec.MachineTemplate.Spec.Taints).To(Equal(taints))
}
