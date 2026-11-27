//go:build e2e
// +build e2e

/*
Copyright 2026 The Kubernetes Authors.

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

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
	controlplanev1 "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta2"
)

// WaitForV1Beta2StatusInput is the input for WaitForV1Beta2Status.
type WaitForV1Beta2StatusInput struct {
	Client       client.Client
	ControlPlane *controlplanev1.KThreesControlPlane
}

// WaitForV1Beta2Status waits for status.v1beta2 of the KThreesControlPlane, and of the KThreesConfigs in its namespace,
// to report a healthy control plane with all replicas ready, available and up-to-date, and generated bootstrap data.
func WaitForV1Beta2Status(ctx context.Context, input WaitForV1Beta2StatusInput, intervals ...interface{}) {
	Expect(input.Client).ToNot(BeNil(), "Invalid argument. input.Client can't be nil when calling WaitForV1Beta2Status")
	Expect(input.ControlPlane).ToNot(BeNil(), "Invalid argument. input.ControlPlane can't be nil when calling WaitForV1Beta2Status")

	By("Waiting for status.v1beta2 of the KThreesControlPlane and the KThreesConfigs to report a healthy control plane")
	Eventually(func(g Gomega) {
		kcp := &controlplanev1.KThreesControlPlane{}
		g.Expect(input.Client.Get(ctx, client.ObjectKeyFromObject(input.ControlPlane), kcp)).To(Succeed())

		g.Expect(kcp.Status.V1Beta2).ToNot(BeNil(), "KThreesControlPlane %s: status.v1beta2 is not set", kcp.Name)
		replicas := ptr.Deref(kcp.Spec.Replicas, 0)
		g.Expect(kcp.Status.V1Beta2.ReadyReplicas).To(HaveValue(Equal(replicas)), "KThreesControlPlane %s: status.v1beta2.readyReplicas", kcp.Name)
		g.Expect(kcp.Status.V1Beta2.AvailableReplicas).To(HaveValue(Equal(replicas)), "KThreesControlPlane %s: status.v1beta2.availableReplicas", kcp.Name)
		g.Expect(kcp.Status.V1Beta2.UpToDateReplicas).To(HaveValue(Equal(replicas)), "KThreesControlPlane %s: status.v1beta2.upToDateReplicas", kcp.Name)

		expectV1Beta2Conditions(g, "KThreesControlPlane "+kcp.Name, kcp.GetV1Beta2Conditions(), map[string]metav1.ConditionStatus{
			controlplanev1.KThreesControlPlaneAvailableV1Beta2Condition:                     metav1.ConditionTrue,
			controlplanev1.KThreesControlPlaneInitializedV1Beta2Condition:                   metav1.ConditionTrue,
			controlplanev1.KThreesControlPlaneCertificatesAvailableV1Beta2Condition:         metav1.ConditionTrue,
			controlplanev1.KThreesControlPlaneTokenAvailableV1Beta2Condition:                metav1.ConditionTrue,
			controlplanev1.KThreesControlPlaneControlPlaneComponentsHealthyV1Beta2Condition: metav1.ConditionTrue,
			controlplanev1.KThreesControlPlaneMachinesReadyV1Beta2Condition:                 metav1.ConditionTrue,
			controlplanev1.KThreesControlPlaneMachinesUpToDateV1Beta2Condition:              metav1.ConditionTrue,
			controlplanev1.KThreesControlPlaneRollingOutV1Beta2Condition:                    metav1.ConditionFalse,
			controlplanev1.KThreesControlPlaneScalingUpV1Beta2Condition:                     metav1.ConditionFalse,
			controlplanev1.KThreesControlPlaneScalingDownV1Beta2Condition:                   metav1.ConditionFalse,
			controlplanev1.KThreesControlPlaneRemediatingV1Beta2Condition:                   metav1.ConditionFalse,
			controlplanev1.KThreesControlPlaneDeletingV1Beta2Condition:                      metav1.ConditionFalse,
		})

		// EtcdClusterHealthy is only required to be set: when the etcd-proxy pod is not running yet at the first check,
		// the condition can stay False until the next resync, because the KThreesControlPlane is not requeued for it.
		g.Expect(meta.FindStatusCondition(kcp.GetV1Beta2Conditions(), controlplanev1.KThreesControlPlaneEtcdClusterHealthyV1Beta2Condition)).ToNot(BeNil(),
			"KThreesControlPlane %s: condition %s is not set", kcp.Name, controlplanev1.KThreesControlPlaneEtcdClusterHealthyV1Beta2Condition)

		configs := &bootstrapv1.KThreesConfigList{}
		g.Expect(input.Client.List(ctx, configs, client.InNamespace(kcp.Namespace))).To(Succeed())
		g.Expect(configs.Items).ToNot(BeEmpty(), "no KThreesConfigs found in namespace %s", kcp.Namespace)
		for i := range configs.Items {
			config := &configs.Items[i]
			g.Expect(config.Status.V1Beta2).ToNot(BeNil(), "KThreesConfig %s: status.v1beta2 is not set", config.Name)
			expectV1Beta2Conditions(g, "KThreesConfig "+config.Name, config.GetV1Beta2Conditions(), map[string]metav1.ConditionStatus{
				bootstrapv1.KThreesConfigReadyV1Beta2Condition:                 metav1.ConditionTrue,
				bootstrapv1.KThreesConfigDataSecretAvailableV1Beta2Condition:   metav1.ConditionTrue,
				bootstrapv1.KThreesConfigCertificatesAvailableV1Beta2Condition: metav1.ConditionTrue,
			})
		}
	}, intervals...).Should(Succeed())
}

// expectV1Beta2Conditions checks that each condition in want is set with the expected status.
func expectV1Beta2Conditions(g Gomega, object string, got []metav1.Condition, want map[string]metav1.ConditionStatus) {
	for conditionType, status := range want {
		c := meta.FindStatusCondition(got, conditionType)
		g.Expect(c).ToNot(BeNil(), "%s: condition %s is not set", object, conditionType)
		g.Expect(c.Status).To(Equal(status), "%s: condition %s is %s (reason %s, message %q), expected %s",
			object, conditionType, c.Status, c.Reason, c.Message, status)
	}
}
