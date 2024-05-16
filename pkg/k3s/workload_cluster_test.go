package k3s

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterStatus(t *testing.T) {
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				labelNodeRoleControlPlane: "true",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{
				Type:   corev1.NodeReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			Labels: map[string]string{
				labelNodeRoleControlPlane: "true",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{
				Type:   corev1.NodeReady,
				Status: corev1.ConditionFalse,
			}},
		},
	}
	servingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k3sServingSecretKey,
			Namespace: metav1.NamespaceSystem,
		},
	}
	tests := []struct {
		name            string
		objs            []client.Object
		expectErr       bool
		expectHasSecret bool
	}{
		{
			name:            "returns cluster status",
			objs:            []client.Object{node1, node2},
			expectHasSecret: false,
		},
		{
			name:            "returns cluster status with k3s-serving secret",
			objs:            []client.Object{node1, node2, servingSecret},
			expectHasSecret: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithObjects(tt.objs...).Build()
			w := &Workload{
				Client: fakeClient,
			}
			status, err := w.ClusterStatus(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(status.Nodes).To(BeEquivalentTo(2))
			g.Expect(status.ReadyNodes).To(BeEquivalentTo(1))
			g.Expect(status.HasK3sServingSecret).To(Equal(tt.expectHasSecret))
		})
	}
}
