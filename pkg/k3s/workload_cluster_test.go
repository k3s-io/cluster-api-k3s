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

// Pinned to literal label keys (not the production constants) so the test
// asserts the K3s wire contract and survives constant renames.
const (
	testLabelNodeRoleControlPlane           = "node-role.kubernetes.io/control-plane"
	testLabelNodeRoleControlPlaneDeprecated = "node-role.kubernetes.io/master"
)

func makeNode(name string, ready bool, labels map[string]string) *corev1.Node {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{
				Type:   corev1.NodeReady,
				Status: status,
			}},
		},
	}
}

func TestClusterStatus(t *testing.T) {
	servingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k3sServingSecretKey,
			Namespace: metav1.NamespaceSystem,
		},
	}

	tests := []struct {
		name             string
		nodes            []client.Object
		includeSecret    bool
		expectNodes      int32
		expectReadyNodes int32
		expectHasSecret  bool
	}{
		{
			name: "modern K3s: nodes labeled with control-plane only are counted",
			nodes: []client.Object{
				makeNode("cp-modern-1", true, map[string]string{testLabelNodeRoleControlPlane: "true"}),
				makeNode("cp-modern-2", false, map[string]string{testLabelNodeRoleControlPlane: "true"}),
			},
			expectNodes:      2,
			expectReadyNodes: 1,
		},
		{
			name: "legacy K3s: nodes labeled with the deprecated master label only are counted",
			nodes: []client.Object{
				makeNode("cp-legacy-1", true, map[string]string{testLabelNodeRoleControlPlaneDeprecated: "true"}),
				makeNode("cp-legacy-2", false, map[string]string{testLabelNodeRoleControlPlaneDeprecated: "true"}),
			},
			expectNodes:      2,
			expectReadyNodes: 1,
		},
		{
			name: "transitional: a node carrying both labels is counted exactly once",
			nodes: []client.Object{
				makeNode("cp-both", true, map[string]string{
					testLabelNodeRoleControlPlane:           "true",
					testLabelNodeRoleControlPlaneDeprecated: "true",
				}),
			},
			expectNodes:      1,
			expectReadyNodes: 1,
		},
		{
			name: "mixed cluster: modern + legacy + dual-labelled nodes are all counted exactly once",
			nodes: []client.Object{
				makeNode("cp-modern", true, map[string]string{testLabelNodeRoleControlPlane: "true"}),
				makeNode("cp-legacy", false, map[string]string{testLabelNodeRoleControlPlaneDeprecated: "true"}),
				makeNode("cp-both", true, map[string]string{
					testLabelNodeRoleControlPlane:           "true",
					testLabelNodeRoleControlPlaneDeprecated: "true",
				}),
			},
			expectNodes:      3,
			expectReadyNodes: 2,
		},
		{
			name: "worker nodes (no control-plane role label) are excluded",
			nodes: []client.Object{
				makeNode("cp", true, map[string]string{testLabelNodeRoleControlPlane: "true"}),
				makeNode("worker-bare", true, nil),
				makeNode("worker-other-label", true, map[string]string{"example.com/role": "true"}),
			},
			expectNodes:      1,
			expectReadyNodes: 1,
		},
		{
			name: "k3s-serving secret detection still works with modern labels",
			nodes: []client.Object{
				makeNode("cp", true, map[string]string{testLabelNodeRoleControlPlane: "true"}),
			},
			includeSecret:    true,
			expectNodes:      1,
			expectReadyNodes: 1,
			expectHasSecret:  true,
		},
		{
			name: "k3s-serving secret detection still works with legacy labels",
			nodes: []client.Object{
				makeNode("cp", true, map[string]string{testLabelNodeRoleControlPlaneDeprecated: "true"}),
			},
			includeSecret:    true,
			expectNodes:      1,
			expectReadyNodes: 1,
			expectHasSecret:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			objs := append([]client.Object{}, tt.nodes...)
			if tt.includeSecret {
				objs = append(objs, servingSecret)
			}
			fakeClient := fake.NewClientBuilder().WithObjects(objs...).Build()
			w := &Workload{
				Client: fakeClient,
			}
			status, err := w.ClusterStatus(context.TODO())
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(status.Nodes).To(BeEquivalentTo(tt.expectNodes))
			g.Expect(status.ReadyNodes).To(BeEquivalentTo(tt.expectReadyNodes))
			g.Expect(status.HasK3sServingSecret).To(Equal(tt.expectHasSecret))
		})
	}
}
