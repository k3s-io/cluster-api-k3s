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
	"fmt"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/randfill"

	controlplanev1beta2 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta3"
)

var testGVKs = []schema.GroupVersionKind{
	{Group: "foo", Version: "v1beta1", Kind: "Bar"},
	{Group: "foo", Version: "v1beta2", Kind: "Baz"},
	{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "DockerMachineTemplate"},
	{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Kind: "AWSMachineTemplate"},
}

func TestFuzzyConversion(t *testing.T) {
	controlplanev1beta2.SetAPIVersionGetter(func(gk schema.GroupKind) (string, error) {
		for _, gvk := range testGVKs {
			if gvk.GroupKind() == gk {
				return gvk.GroupVersion().String(), nil
			}
		}
		return "", fmt.Errorf("failed to map GroupKind %s to version", gk.String())
	})

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(controlplanev1beta2.AddToScheme(scheme)).To(Succeed())
	g.Expect(controlplanev1.AddToScheme(scheme)).To(Succeed())

	t.Run("for KThreesControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &controlplanev1.KThreesControlPlane{},
		Spoke:  &KThreesControlPlane{},
		SpokeAfterMutation: func(spoke conversion.Convertible) {
			// Remove the annotation storing hub data, which is added by ConvertFrom
			// in addition to the standard conversion data annotation.
			delete(spoke.(metav1.Object).GetAnnotations(), HubDataAnnotation)
		},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))
}

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubKThreesControlPlaneMachineTemplate,
		hubKThreesControlPlaneStatus,
		spokeKThreesControlPlane,
		spokeKThreesControlPlaneStatus,
		spokeRemediationStrategy,
	}
}

func hubKThreesControlPlaneMachineTemplate(in *controlplanev1.KThreesControlPlaneMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(in)

	// Ensure ref field is always set to realistic values.
	gvk := testGVKs[c.Int31n(4)]
	in.Spec.InfrastructureRef.APIGroup = gvk.Group
	in.Spec.InfrastructureRef.Kind = gvk.Kind
	if in.Spec.InfrastructureRef.Name == "" {
		in.Spec.InfrastructureRef.Name = "infra"
	}
}

func hubKThreesControlPlaneStatus(in *controlplanev1.KThreesControlPlaneStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Always create struct with at least one mandatory field.
	if in.Deprecated == nil {
		in.Deprecated = &controlplanev1.KThreesControlPlaneDeprecatedStatus{}
	}
	if in.Deprecated.V1Beta1 == nil {
		in.Deprecated.V1Beta1 = &controlplanev1.KThreesControlPlaneV1Beta1DeprecatedStatus{}
	}

	// nil becomes &0 after hub => spoke => hub conversion.
	if in.Replicas == nil {
		in.Replicas = ptr.To(int32(0))
	}
}

func spokeKThreesControlPlane(in *KThreesControlPlane, c randfill.Continue) {
	c.FillNoCustom(in)

	// Ensure ref fields are always set to realistic values; the v1beta3 reference does not
	// have a namespace (the namespace of the KThreesControlPlane is used), uid, resourceVersion or fieldPath.
	gvk := testGVKs[c.Int31n(4)]
	in.Spec.InfrastructureTemplate.APIVersion = gvk.GroupVersion().String()
	in.Spec.InfrastructureTemplate.Kind = gvk.Kind
	in.Spec.InfrastructureTemplate.Namespace = in.Namespace
	in.Spec.InfrastructureTemplate.UID = ""
	in.Spec.InfrastructureTemplate.ResourceVersion = ""
	in.Spec.InfrastructureTemplate.FieldPath = ""
	if in.Spec.InfrastructureTemplate.Name == "" {
		in.Spec.InfrastructureTemplate.Name = "infra"
	}

	// Durations are stored with second granularity in v1beta3.
	if in.Spec.NodeDrainTimeout != nil {
		in.Spec.NodeDrainTimeout = &metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second}
	}

	if reflect.DeepEqual(in.Spec.UpgradeAfter, &metav1.Time{}) {
		in.Spec.UpgradeAfter = nil
	}
	if reflect.DeepEqual(in.Spec.RemediationStrategy, &RemediationStrategy{}) {
		in.Spec.RemediationStrategy = nil
	}
}

func spokeKThreesControlPlaneStatus(in *KThreesControlPlaneStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Ready is rebuilt from ReadyReplicas when converting from v1beta3.
	in.Ready = in.ReadyReplicas > 0
}

func spokeRemediationStrategy(in *RemediationStrategy, c randfill.Continue) {
	c.FillNoCustom(in)

	// Durations are stored with second granularity in v1beta3.
	if in.MinHealthyPeriod != nil {
		in.MinHealthyPeriod = &metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second}
	}
	in.RetryPeriod = metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second}
}
