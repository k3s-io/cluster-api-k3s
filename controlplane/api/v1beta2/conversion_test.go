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
	"sigs.k8s.io/randfill"

	bootstrapv1beta2 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta3"
)

var testGVKs = []schema.GroupVersionKind{
	{Group: "foo", Version: "v1beta1", Kind: "Bar"},
	{Group: "foo", Version: "v1beta2", Kind: "Baz"},
	{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "DockerMachineTemplate"},
	{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Kind: "AWSMachineTemplate"},
}

// testInfraName is the name used for infrastructure references in fuzz tests.
const testInfraName = "infra"

func TestFuzzyConversion(t *testing.T) {
	SetAPIVersionGetter(testAPIVersionGetter)

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(controlplanev1.AddToScheme(scheme)).To(Succeed())

	t.Run("for KThreesControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &controlplanev1.KThreesControlPlane{},
		Spoke:       &KThreesControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{KThreesControlPlaneFuzzFuncs},
	}))
	t.Run("for KThreesControlPlaneTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &controlplanev1.KThreesControlPlaneTemplate{},
		Spoke:       &KThreesControlPlaneTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{KThreesControlPlaneTemplateFuzzFuncs},
	}))
}

// testAPIVersionGetter returns the apiVersion of the test GroupVersionKinds used in fuzz tests.
func testAPIVersionGetter(gk schema.GroupKind) (string, error) {
	for _, gvk := range testGVKs {
		if gvk.GroupKind() == gk {
			return gvk.GroupVersion().String(), nil
		}
	}
	return "", fmt.Errorf("failed to map GroupKind %s to version", gk.String())
}

// KThreesControlPlaneFuzzFuncs returns the fuzzer functions for KThreesControlPlane conversion tests.
func KThreesControlPlaneFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubKThreesControlPlaneMachineTemplate,
		hubKThreesControlPlaneStatus,
		spokeKThreesControlPlane,
		spokeKThreesControlPlaneStatus,
		spokeKThreesControlPlaneMachineTemplate,
		spokeRemediationStrategy,
		spokeKThreesServerConfig,
	}
}

// KThreesControlPlaneTemplateFuzzFuncs returns the fuzzer functions for KThreesControlPlaneTemplate conversion tests.
func KThreesControlPlaneTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubKThreesControlPlaneMachineTemplate,
		spokeKThreesControlPlaneTemplate,
		spokeKThreesControlPlaneTemplateResource,
		spokeKThreesControlPlaneTemplateResourceSpec,
		spokeRemediationStrategy,
		spokeKThreesServerConfig,
	}
}

func hubKThreesControlPlaneMachineTemplate(in *controlplanev1.KThreesControlPlaneMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(in)

	// Ensure ref field is always set to realistic values.
	gvk := testGVKs[c.Int31n(4)]
	in.Spec.InfrastructureRef.APIGroup = gvk.Group
	in.Spec.InfrastructureRef.Kind = gvk.Kind
	if in.Spec.InfrastructureRef.Name == "" {
		in.Spec.InfrastructureRef.Name = testInfraName
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
	in.Spec.MachineTemplate.InfrastructureRef.APIVersion = gvk.GroupVersion().String()
	in.Spec.MachineTemplate.InfrastructureRef.Kind = gvk.Kind
	in.Spec.MachineTemplate.InfrastructureRef.Namespace = in.Namespace
	in.Spec.MachineTemplate.InfrastructureRef.UID = ""
	in.Spec.MachineTemplate.InfrastructureRef.ResourceVersion = ""
	in.Spec.MachineTemplate.InfrastructureRef.FieldPath = ""
	if in.Spec.MachineTemplate.InfrastructureRef.Name == "" {
		in.Spec.MachineTemplate.InfrastructureRef.Name = testInfraName
	}

	if reflect.DeepEqual(in.Spec.RolloutAfter, &metav1.Time{}) {
		in.Spec.RolloutAfter = nil
	}
	if reflect.DeepEqual(in.Spec.RemediationStrategy, &RemediationStrategy{}) {
		in.Spec.RemediationStrategy = nil
	}
}

func spokeKThreesControlPlaneStatus(in *KThreesControlPlaneStatus, c randfill.Continue) {
	c.FillNoCustom(in)

	// Ready is rebuilt from ReadyReplicas when converting from v1beta3.
	in.Ready = in.ReadyReplicas > 0

	// An empty version becomes nil after a round trip through v1beta3, where the field is a string.
	if in.Version != nil && *in.Version == "" {
		in.Version = nil
	}

	// An empty v1beta2 struct can't be distinguished from nil after a round trip.
	if reflect.DeepEqual(in.V1Beta2, &KThreesControlPlaneV1Beta2Status{}) {
		in.V1Beta2 = nil
	}
}

func spokeKThreesControlPlaneMachineTemplate(in *KThreesControlPlaneMachineTemplate, c randfill.Continue) {
	c.FillNoCustom(in)

	// Durations are stored with second granularity in v1beta3.
	in.NodeDrainTimeout = durationInSeconds(in.NodeDrainTimeout, c)
	in.NodeVolumeDetachTimeout = durationInSeconds(in.NodeVolumeDetachTimeout, c)
	in.NodeDeletionTimeout = durationInSeconds(in.NodeDeletionTimeout, c)
}

func spokeRemediationStrategy(in *RemediationStrategy, c randfill.Continue) {
	c.FillNoCustom(in)

	// Durations are stored with second granularity in v1beta3.
	in.MinHealthyPeriod = durationInSeconds(in.MinHealthyPeriod, c)
	in.RetryPeriod = metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second}
}

func spokeKThreesControlPlaneTemplate(in *KThreesControlPlaneTemplate, c randfill.Continue) {
	c.FillNoCustom(in)

	// Ensure ref fields are always set to realistic values; the v1beta3 reference does not
	// have a namespace (the namespace of the KThreesControlPlaneTemplate is used), uid, resourceVersion or fieldPath.
	ref := &in.Spec.Template.Spec.MachineTemplate.InfrastructureRef
	gvk := testGVKs[c.Int31n(4)]
	ref.APIVersion = gvk.GroupVersion().String()
	ref.Kind = gvk.Kind
	ref.Namespace = in.Namespace
	ref.UID = ""
	ref.ResourceVersion = ""
	ref.FieldPath = ""
	if ref.Name == "" {
		ref.Name = testInfraName
	}
}

func spokeKThreesControlPlaneTemplateResource(in *KThreesControlPlaneTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)

	// Only labels and annotations are preserved in v1beta3.
	in.ObjectMeta = metav1.ObjectMeta{
		Labels:      in.ObjectMeta.Labels,
		Annotations: in.ObjectMeta.Annotations,
	}
}

func spokeKThreesControlPlaneTemplateResourceSpec(in *KThreesControlPlaneTemplateResourceSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	// Durations are stored with second granularity in v1beta3.
	in.MachineTemplate.NodeDrainTimeout = durationInSeconds(in.MachineTemplate.NodeDrainTimeout, c)
	in.MachineTemplate.NodeVolumeDetachTimeout = durationInSeconds(in.MachineTemplate.NodeVolumeDetachTimeout, c)
	in.MachineTemplate.NodeDeletionTimeout = durationInSeconds(in.MachineTemplate.NodeDeletionTimeout, c)

	if reflect.DeepEqual(in.RolloutAfter, &metav1.Time{}) {
		in.RolloutAfter = nil
	}
	if reflect.DeepEqual(in.RemediationStrategy, &RemediationStrategy{}) {
		in.RemediationStrategy = nil
	}
}

func spokeKThreesServerConfig(in *bootstrapv1beta2.KThreesServerConfig, c randfill.Continue) {
	c.FillNoCustom(in)

	// DeprecatedDisableExternalCloudProvider does not exist in v1beta3; it was never used by controllers.
	in.DeprecatedDisableExternalCloudProvider = false
}

func durationInSeconds(d *metav1.Duration, c randfill.Continue) *metav1.Duration {
	if d == nil {
		return nil
	}
	return &metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second}
}
