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
package v1beta2

import (
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta3"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())

	t.Run("for KThreesConfig", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &bootstrapv1.KThreesConfig{},
		Spoke:       &KThreesConfig{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))
	t.Run("for KThreesConfigTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &bootstrapv1.KThreesConfigTemplate{},
		Spoke:       &KThreesConfigTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))
}

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeKThreesServerConfig,
		spokeKThreesConfigStatus,
		hubKThreesConfigStatus,
	}
}

func spokeKThreesServerConfig(in *KThreesServerConfig, c randfill.Continue) {
	c.FillNoCustom(in)

	// DeprecatedDisableExternalCloudProvider does not exist in v1beta3; it was never used by controllers.
	in.DeprecatedDisableExternalCloudProvider = false
}

func spokeKThreesConfigStatus(in *KThreesConfigStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// BootstrapData has never been used and does not exist in v1beta3.
	in.BootstrapData = nil

	// An empty DataSecretName becomes nil after a round trip through v1beta3, where the field is a string.
	if in.DataSecretName != nil && *in.DataSecretName == "" {
		in.DataSecretName = nil
	}

	// An empty v1beta2 struct can't be distinguished from nil after a round trip.
	if in.V1Beta2 != nil && len(in.V1Beta2.Conditions) == 0 {
		in.V1Beta2 = nil
	}
}

func hubKThreesConfigStatus(in *bootstrapv1.KThreesConfigStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Drop empty structs which can't be distinguished from nil after a round trip.
	if in.Deprecated != nil && (in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &bootstrapv1.KThreesConfigV1Beta1DeprecatedStatus{})) {
		in.Deprecated = nil
	}
}
