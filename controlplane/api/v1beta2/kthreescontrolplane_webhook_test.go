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
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/feature"
)

func TestDefaultKThreesControlPlaneSpec(t *testing.T) {
	kcp := &KThreesControlPlane{
		Spec: KThreesControlPlaneSpec{},
	}
	g := NewWithT(t)

	g.Expect((&KThreesControlPlane{}).Default(context.Background(), kcp)).To(Succeed())
	g.Expect(kcp.Spec.RolloutStrategy).ToNot(BeNil())
	g.Expect(kcp.Spec.RolloutStrategy.Type).To(Equal(RollingUpdateStrategyType))
	g.Expect(kcp.Spec.RolloutStrategy.RollingUpdate).ToNot(BeNil())
	g.Expect(kcp.Spec.RolloutStrategy.RollingUpdate.MaxSurge).ToNot(BeNil())
	g.Expect(kcp.Spec.RolloutStrategy.RollingUpdate.MaxSurge.IntValue()).To(Equal(1))
}

func TestKThreesControlPlaneValidation(t *testing.T) {
	tests := []struct {
		name                 string
		maxSurge             intstr.IntOrString
		replicas             int32
		strategyType         RolloutStrategyType
		enableInPlaceUpdates bool
		wantErr              bool
	}{
		{
			name:         "allows integer zero with three replicas",
			maxSurge:     intstr.FromInt32(0),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
		},
		{
			name:         "allows integer one",
			maxSurge:     intstr.FromInt32(1),
			replicas:     1,
			strategyType: RollingUpdateStrategyType,
		},
		{
			name:         "allows numeric string zero with three replicas",
			maxSurge:     intstr.FromString("0"),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
		},
		{
			name:         "allows numeric string one",
			maxSurge:     intstr.FromString("1"),
			replicas:     1,
			strategyType: RollingUpdateStrategyType,
		},
		{
			name:         "rejects percentages",
			maxSurge:     intstr.FromString("50%"),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
			wantErr:      true,
		},
		{
			name:         "rejects integer two",
			maxSurge:     intstr.FromInt32(2),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
			wantErr:      true,
		},
		{
			name:         "rejects unknown strategy type",
			maxSurge:     intstr.FromInt32(1),
			replicas:     3,
			strategyType: RolloutStrategyType("Unknown"),
			wantErr:      true,
		},
		{
			name:         "rejects zero surge for one replica when feature disabled",
			maxSurge:     intstr.FromInt32(0),
			replicas:     1,
			strategyType: RollingUpdateStrategyType,
			wantErr:      true,
		},
		{
			name:                 "allows zero surge for one replica when feature enabled",
			maxSurge:             intstr.FromInt32(0),
			replicas:             1,
			strategyType:         RollingUpdateStrategyType,
			enableInPlaceUpdates: true,
		},
		{
			name:         "allows zero surge for three replicas when feature disabled",
			maxSurge:     intstr.FromInt32(0),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enableInPlaceUpdates {
				utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, true)
			}

			kcp := &KThreesControlPlane{
				Spec: KThreesControlPlaneSpec{
					Replicas: ptr.To(tt.replicas),
					RolloutStrategy: &RolloutStrategy{
						Type: tt.strategyType,
						RollingUpdate: &RollingUpdate{
							MaxSurge: ptr.To(tt.maxSurge),
						},
					},
				},
			}

			_, createErr := (&KThreesControlPlane{}).ValidateCreate(context.Background(), kcp)
			_, updateErr := (&KThreesControlPlane{}).ValidateUpdate(context.Background(), kcp.DeepCopy(), kcp)
			if tt.wantErr {
				NewWithT(t).Expect(createErr).To(HaveOccurred())
				NewWithT(t).Expect(updateErr).To(HaveOccurred())
				return
			}
			NewWithT(t).Expect(createErr).NotTo(HaveOccurred())
			NewWithT(t).Expect(updateErr).NotTo(HaveOccurred())
		})
	}
}
