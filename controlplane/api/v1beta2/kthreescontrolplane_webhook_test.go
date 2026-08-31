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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestKThreesControlPlaneValidateCreate(t *testing.T) {
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
			name:         "rejects numeric string zero with leading zero",
			maxSurge:     intstr.FromString("00"),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
			wantErr:      true,
		},
		{
			name:         "rejects numeric string one with plus sign",
			maxSurge:     intstr.FromString("+1"),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
			wantErr:      true,
		},
		{
			name:         "rejects numeric string zero with minus sign",
			maxSurge:     intstr.FromString("-0"),
			replicas:     3,
			strategyType: RollingUpdateStrategyType,
			wantErr:      true,
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

			_, err := (&KThreesControlPlane{}).ValidateCreate(context.Background(), kcp)
			if tt.wantErr {
				NewWithT(t).Expect(err).To(HaveOccurred())
				return
			}
			NewWithT(t).Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestKThreesControlPlaneValidateUpdateGateTransition(t *testing.T) {
	zero := intstr.FromInt32(0)
	zeroString := intstr.FromString("0")
	one := intstr.FromInt32(1)
	malformed := intstr.FromString("50%")

	newControlPlane := func(replicas int32, maxSurge intstr.IntOrString) *KThreesControlPlane {
		return &KThreesControlPlane{
			Spec: KThreesControlPlaneSpec{
				Replicas: ptr.To(replicas),
				Version:  "v1.31.0+k3s1",
				RolloutStrategy: &RolloutStrategy{
					Type: RollingUpdateStrategyType,
					RollingUpdate: &RollingUpdate{
						MaxSurge: ptr.To(maxSurge),
					},
				},
			},
		}
	}

	tests := []struct {
		name            string
		oldObj          runtime.Object
		newKCP          *KThreesControlPlane
		wantErr         bool
		wantErrContains string
		wantBadReq      bool
	}{
		{
			name:   "allows unchanged unsafe configuration with version change",
			oldObj: newControlPlane(1, zero),
			newKCP: func() *KThreesControlPlane {
				kcp := newControlPlane(1, zero)
				kcp.Spec.Version = "v1.31.1+k3s1"
				return kcp
			}(),
		},
		{
			name:   "allows semantically unchanged string to integer zero surge",
			oldObj: newControlPlane(1, zeroString),
			newKCP: newControlPlane(1, zero),
		},
		{
			name:            "rejects replica increase while retaining zero surge",
			oldObj:          newControlPlane(1, zero),
			newKCP:          newControlPlane(2, zero),
			wantErr:         true,
			wantErrContains: "replica count needs to be at least 3",
		},
		{
			name:            "rejects replica decrease into unsafe configuration",
			oldObj:          newControlPlane(2, zero),
			newKCP:          newControlPlane(1, zero),
			wantErr:         true,
			wantErrContains: "replica count needs to be at least 3",
		},
		{
			name:            "rejects transition from positive to zero surge",
			oldObj:          newControlPlane(1, one),
			newKCP:          newControlPlane(1, zero),
			wantErr:         true,
			wantErrContains: "replica count needs to be at least 3",
		},
		{
			name:   "allows transition to safe replica count",
			oldObj: newControlPlane(1, zero),
			newKCP: newControlPlane(3, zero),
		},
		{
			name:   "allows transition to positive surge",
			oldObj: newControlPlane(1, zero),
			newKCP: newControlPlane(1, one),
		},
		{
			name:            "rejects malformed new maxSurge",
			oldObj:          newControlPlane(1, zero),
			newKCP:          newControlPlane(1, malformed),
			wantErr:         true,
			wantErrContains: "maxSurge must be 0 or 1",
		},
		{
			name:   "rejects unknown new rollout strategy type",
			oldObj: newControlPlane(1, zero),
			newKCP: func() *KThreesControlPlane {
				kcp := newControlPlane(1, zero)
				kcp.Spec.RolloutStrategy.Type = RolloutStrategyType("Unknown")
				return kcp
			}(),
			wantErr:         true,
			wantErrContains: "only RollingUpdate is supported",
		},
		{
			name:       "rejects wrong old object type",
			oldObj:     &metav1.PartialObjectMetadata{},
			newKCP:     newControlPlane(1, zero),
			wantErr:    true,
			wantBadReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.InPlaceUpdates, false)

			_, err := (&KThreesControlPlane{}).ValidateUpdate(context.Background(), tt.oldObj, tt.newKCP)
			if tt.wantErr {
				NewWithT(t).Expect(err).To(HaveOccurred())
				if tt.wantErrContains != "" {
					NewWithT(t).Expect(err.Error()).To(ContainSubstring(tt.wantErrContains))
				}
				if tt.wantBadReq {
					NewWithT(t).Expect(apierrors.IsBadRequest(err)).To(BeTrue())
				}
				return
			}
			NewWithT(t).Expect(err).NotTo(HaveOccurred())
		})
	}
}
