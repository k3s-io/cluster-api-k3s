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

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
)

func TestReconcileJoinWaitsForControlPlaneEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		controlPlane bool
	}{
		{name: "worker", controlPlane: false},
		{name: "control plane", controlPlane: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
			g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
			g.Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())

			// The control plane is initialized, but the Cluster has no ControlPlaneEndpoint,
			// for example because an update of the Cluster cleared it.
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: metav1.NamespaceDefault},
				Status: clusterv1.ClusterStatus{
					Initialization: clusterv1.ClusterInitializationStatus{
						InfrastructureProvisioned: ptr.To(true),
						ControlPlaneInitialized:   ptr.To(true),
					},
				},
			}
			machine := &clusterv1.Machine{
				TypeMeta: metav1.TypeMeta{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine",
					Namespace: metav1.NamespaceDefault,
					Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
				},
				Spec: clusterv1.MachineSpec{ClusterName: cluster.Name},
			}
			if tt.controlPlane {
				machine.Labels[clusterv1.MachineControlPlaneLabel] = ""
			}
			config := &bootstrapv1.KThreesConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: metav1.NamespaceDefault,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machine.Name,
					}},
				},
			}

			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster, machine, config).
				WithStatusSubresource(cluster, machine, config).
				Build()
			r := &KThreesConfigReconciler{Client: c, Log: log.Log, Scheme: scheme}

			res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: config.Namespace, Name: config.Name}})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.RequeueAfter).To(Equal(controlPlaneEndpointRequeueAfter))

			// No bootstrap data is generated without the endpoint.
			got := &bootstrapv1.KThreesConfig{}
			g.Expect(c.Get(context.Background(), types.NamespacedName{Namespace: config.Namespace, Name: config.Name}, got)).To(Succeed())
			g.Expect(got.Status.Ready).To(BeFalse())
			g.Expect(got.Status.DataSecretName).To(BeNil())
		})
	}
}
