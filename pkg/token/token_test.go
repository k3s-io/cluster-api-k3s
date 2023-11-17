package token

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLookup(t *testing.T) {
	const testToken = "test-token"

	// Mock a Kubernetes client
	ctrlClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	// Create a test cluster key
	clusterKey := client.ObjectKey{Name: "test-cluster", Namespace: "default"}

	// Test case: Secret does not exist
	if token, err := Lookup(context.Background(), ctrlClient, clusterKey); token != nil || err == nil {
		t.Errorf("Lookup() should return nil token and error when secret does not exist")
	}

	// Test case: Secret exists
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name(clusterKey.Name), Namespace: clusterKey.Namespace},
		Data:       map[string][]byte{"value": []byte(testToken)},
		Type:       clusterv1.ClusterSecretType,
	}
	ctrlClient.Create(context.Background(), secret)

	if token, err := Lookup(context.Background(), ctrlClient, clusterKey); token == nil || *token != testToken || err != nil {
		t.Errorf("Lookup() returned unexpected result. Expected: %v, Actual: %v, error: %v", testToken, token, err)
	}
}

func TestReconcile(t *testing.T) {
	// Create a Pod as the owner. By using a Pod, we avoid having to deal with schemes.
	// This could just as easily be a KThreesControlPlane or any other object
	owner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "controlling-owner", Namespace: "default"},
	}

	// Mock a Kubernetes client
	ctrlClient := fake.NewClientBuilder().WithObjects(owner.DeepCopy()).Build()

	// Create a test cluster key
	clusterKey := client.ObjectKey{Name: "test-cluster", Namespace: "default"}

	// Test case: Secret does not exist
	if err := Reconcile(context.Background(), ctrlClient, clusterKey, owner); err != nil {
		t.Errorf("Reconcile() returned unexpected error when secret does not exist: %v", err)
	}

	// Verify that the secret has been created
	secret := &corev1.Secret{}
	key := client.ObjectKey{Name: name(clusterKey.Name), Namespace: clusterKey.Namespace}
	if err := ctrlClient.Get(context.Background(), key, secret); err != nil {
		t.Errorf("Failed to get secret: %v", err)
	}

	// Test case: Secret exists, ownership not set
	if err := Reconcile(context.Background(), ctrlClient, clusterKey, owner); err != nil {
		t.Errorf("Reconcile() returned unexpected error when secret exists: %v", err)
	}

	// Verify that controlling ownership has been set
	if !metav1.IsControlledBy(secret, owner) {
		t.Error("Reconcile() did not set correct ownership for existing secret")
	}

	// Test case: Secret already has a different controlling owner reference
	newOwner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "new-controlling-owner", Namespace: "default"},
	}

	if err := Reconcile(context.Background(), ctrlClient, clusterKey, newOwner); err != nil {
		t.Errorf("Reconcile() returned unexpected error when secret exists with different controlling owner: %v", err)
	}

	// Verify that controlling ownership has been set
	if !metav1.IsControlledBy(secret, newOwner) {
		t.Error("Reconcile() did not set overwrite controlling ownership for existing secret")
	}
}
