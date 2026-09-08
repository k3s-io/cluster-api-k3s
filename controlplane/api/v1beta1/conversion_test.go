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
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	controlplanev1beta2 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(controlplanev1beta2.AddToScheme(scheme)).To(Succeed())

	t.Run("for KThreesControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &controlplanev1beta2.KThreesControlPlane{},
		Spoke:       &KThreesControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
}

func TestRolloutStrategyConversionRoundTrip(t *testing.T) {
	in := &KThreesControlPlane{
		Spec: KThreesControlPlaneSpec{
			RolloutStrategy: &RolloutStrategy{
				Type: RollingUpdateStrategyType,
				RollingUpdate: &RollingUpdate{
					MaxSurge: ptr.To(intstr.FromInt32(0)),
				},
			},
		},
	}
	hub := &controlplanev1beta2.KThreesControlPlane{}
	g := NewWithT(t)
	g.Expect(in.ConvertTo(hub)).To(Succeed())
	g.Expect(hub.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntValue()).To(Equal(0))

	out := &KThreesControlPlane{}
	g.Expect(out.ConvertFrom(hub)).To(Succeed())
	g.Expect(out.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntValue()).To(Equal(0))
}
