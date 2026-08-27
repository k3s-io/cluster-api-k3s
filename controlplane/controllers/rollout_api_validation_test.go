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
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

func TestSetupWithManagerRequiresRuntimeClientForInPlaceUpdates(t *testing.T) {
	utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)

	err := (&KThreesControlPlaneReconciler{}).SetupWithManager(context.Background(), nil, nil, 1)
	NewWithT(t).Expect(err).To(MatchError("RuntimeClient must not be nil when InPlaceUpdates feature gate is enabled"))
}

var _ = Describe("KThreesControlPlaneTemplate rollout strategy schema", func() {
	ctx := context.Background()

	newTemplate := func(name string, rolloutStrategy *controlplanev1.RolloutStrategy) *controlplanev1.KThreesControlPlaneTemplate {
		return &controlplanev1.KThreesControlPlaneTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
			},
			Spec: controlplanev1.KThreesControlPlaneTemplateSpec{
				Template: controlplanev1.KThreesControlPlaneTemplateResource{
					Spec: controlplanev1.KThreesControlPlaneTemplateResourceSpec{
						RolloutStrategy: rolloutStrategy,
						MachineTemplate: controlplanev1.KThreesControlPlaneMachineTemplate{
							InfrastructureRef: corev1.ObjectReference{
								APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
								Kind:       "GenericMachineTemplate",
								Name:       "infra-template",
							},
						},
					},
				},
			},
		}
	}

	It("defaults an omitted rollout strategy", func() {
		template := newTemplate("rollout-default", nil)
		Expect(k8sClient.Create(ctx, template)).To(Succeed())
		defer func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, template))).To(Succeed())
		}()

		stored := &controlplanev1.KThreesControlPlaneTemplate{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(template), stored)).To(Succeed())
		Expect(stored.Spec.Template.Spec.RolloutStrategy).NotTo(BeNil())
		Expect(stored.Spec.Template.Spec.RolloutStrategy.Type).To(Equal(controlplanev1.RollingUpdateStrategyType))
		Expect(stored.Spec.Template.Spec.RolloutStrategy.RollingUpdate).NotTo(BeNil())
		Expect(stored.Spec.Template.Spec.RolloutStrategy.RollingUpdate.MaxSurge).NotTo(BeNil())
		Expect(stored.Spec.Template.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntValue()).To(Equal(1))
	})

	validValues := []struct {
		name     string
		maxSurge intstr.IntOrString
	}{
		{name: "integer-zero", maxSurge: intstr.FromInt32(0)},
		{name: "integer-one", maxSurge: intstr.FromInt32(1)},
		{name: "string-zero", maxSurge: intstr.FromString("0")},
		{name: "string-one", maxSurge: intstr.FromString("1")},
	}
	for _, tt := range validValues {
		It("accepts "+tt.name, func() {
			template := newTemplate("rollout-valid-"+tt.name, &controlplanev1.RolloutStrategy{
				Type: controlplanev1.RollingUpdateStrategyType,
				RollingUpdate: &controlplanev1.RollingUpdate{
					MaxSurge: ptr.To(tt.maxSurge),
				},
			})
			Expect(k8sClient.Create(ctx, template)).To(Succeed())
			defer func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, template))).To(Succeed())
			}()
		})
	}

	invalidValues := []struct {
		name         string
		strategyType controlplanev1.RolloutStrategyType
		maxSurge     intstr.IntOrString
	}{
		{
			name:         "percentage",
			strategyType: controlplanev1.RollingUpdateStrategyType,
			maxSurge:     intstr.FromString("50%"),
		},
		{
			name:         "integer-two",
			strategyType: controlplanev1.RollingUpdateStrategyType,
			maxSurge:     intstr.FromInt32(2),
		},
		{
			name:         "unknown-strategy",
			strategyType: controlplanev1.RolloutStrategyType("Unknown"),
			maxSurge:     intstr.FromInt32(1),
		},
	}
	for _, tt := range invalidValues {
		It("rejects "+tt.name, func() {
			template := newTemplate("rollout-invalid-"+tt.name, &controlplanev1.RolloutStrategy{
				Type: tt.strategyType,
				RollingUpdate: &controlplanev1.RollingUpdate{
					MaxSurge: ptr.To(tt.maxSurge),
				},
			})
			Expect(k8sClient.Create(ctx, template)).NotTo(Succeed())
		})
	}
})
