package token

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	//nolint:errcheck
	ctrlClient.Create(context.Background(), secret)

	if token, err := Lookup(context.Background(), ctrlClient, clusterKey); token == nil || *token != testToken || err != nil {
		t.Errorf("Lookup() returned unexpected result. Expected: %v, Actual: %v, error: %v", testToken, token, err)
	}
}

func TestReconcile(t *testing.T) {
	// Create a Pod as the controllingOwner. By using a Pod, we avoid having to deal with schemes.
	// This could just as easily be a KThreesControlPlane or any other object
	controllingOwner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controlling-owner",
			Namespace: "default",
			UID:       "5c4d7cd1-5345-4887-a545-f45bad557ffd", // random, but required for proper ownerRef comparison
		},
	}
	additionalOwner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-controlling-owner",
			Namespace: "default",
			UID:       "2592fce9-789b-4788-9543-d0e9d1fb8a91",
		},
	}
	newControllingOwner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-controlling-owner",
			Namespace: "default",
			UID:       "d0339496-2625-4087-9a7e-e7186f4b3aab",
		},
	}

	// Mock a Kubernetes client
	ctrlClient := fake.NewClientBuilder().
		WithObjects(
			controllingOwner.DeepCopy(),
			additionalOwner.DeepCopy(),
			newControllingOwner.DeepCopy(),
		).
		Build()

	// Create a test cluster key
	clusterKey := client.ObjectKey{Name: "test-cluster", Namespace: "default"}

	// Test case: Secret does not exist
	if err := Reconcile(context.Background(), ctrlClient, clusterKey, controllingOwner); err != nil {
		t.Errorf("Reconcile() returned unexpected error when secret does not exist: %v", err)
	}

	// Verify that the secret has been created
	secret := &corev1.Secret{}
	key := client.ObjectKey{Name: name(clusterKey.Name), Namespace: clusterKey.Namespace}
	if err := ctrlClient.Get(context.Background(), key, secret); err != nil {
		t.Errorf("Failed to get secret: %v", err)
	}

	// Test case: Secret exists, ownership remains
	if err := Reconcile(context.Background(), ctrlClient, clusterKey, controllingOwner); err != nil {
		t.Errorf("Reconcile() returned unexpected error when secret exists: %v", err)
	}

	if err := ctrlClient.Get(context.Background(), key, secret); err != nil {
		t.Errorf("Failed to get secret: %v", err)
	}

	// Verify that controlling ownership has been set
	if !metav1.IsControlledBy(secret, controllingOwner) {
		t.Error("Reconcile() did not set correct ownership for existing secret")
	}

	// Test case: Secret already has a different controlling owner reference
	// and an additional non-controlling owner reference
	if err := addOwnerRef(ctrlClient, secret, additionalOwner); err != nil {
		t.Errorf("Failed to add additional non-controlling owner: %v", err)
	}

	if err := Reconcile(context.Background(), ctrlClient, clusterKey, newControllingOwner); err != nil {
		t.Errorf("Reconcile() returned unexpected error when secret exists with different controlling owner: %v", err)
	}

	if err := ctrlClient.Get(context.Background(), key, secret); err != nil {
		t.Errorf("Failed to get secret: %v", err)
	}

	// Verify that the new controller ref has replaced the old controller ref
	if !metav1.IsControlledBy(secret, newControllingOwner) {
		t.Error("Reconcile() did not set overwrite controlling ownership for existing secret")
	}

	// Verify that the non-controlling owner ref is still present
	if !isOwnedBy(secret, additionalOwner) {
		t.Error("Reconcile() did not maintain existing non-controlling ownership for existing secret")
	}
}

func TestUpsertControllerRef(t *testing.T) {
	// Helper function to create a new instance of TestObject
	newPod := func(name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				UID:  types.UID(uuid.New().String()),
			},
		}
	}

	// Test case 1: No pre-existing owner references
	t.Run("NoPreExistingOwnerReferences", func(t *testing.T) {
		controllee := newPod("controllee")
		controller := newPod("controller")

		upsertControllerRef(controllee, controller)

		if len(controllee.GetOwnerReferences()) != 1 {
			t.Error("Expected one owner reference, got:", len(controllee.GetOwnerReferences()))
		}
		if !metav1.IsControlledBy(controllee, controller) {
			t.Error("Expected controllee to be controlled by controller")
		}
	})

	// Test case 2: Pre-existing controlling owner reference
	t.Run("PreExistingControllingOwnerReference", func(t *testing.T) {
		controllee := newPod("controllee")
		oldController := newPod("old-controller")
		newController := newPod("new-controller")
		setControllerReference(oldController, controllee, scheme.Scheme)

		upsertControllerRef(controllee, newController)

		if len(controllee.GetOwnerReferences()) != 1 {
			t.Error("Expected one owner reference, got:", len(controllee.GetOwnerReferences()))
		}
		if !metav1.IsControlledBy(controllee, newController) {
			t.Error("Expected controllee to be controlled by controller")
		}
	})

	// Test case 3: Pre-existing non-controlling owner reference
	t.Run("PreExistingNonControllingOwnerReference", func(t *testing.T) {
		controllee := newPod("controllee")
		controller := newPod("controller")
		otherObject := newPod("otherObject")
		setOwnerReference(otherObject, controllee, scheme.Scheme)

		upsertControllerRef(controllee, controller)

		if len(controllee.GetOwnerReferences()) != 2 {
			t.Error("Expected two owner references, got:", len(controllee.GetOwnerReferences()))
		}
		if !metav1.IsControlledBy(controllee, controller) {
			t.Error("Expected controllee to be controlled by controller")
		}
	})

	// Test case 4: Pre-existing non-controlling owner references and pre-existing controlling owner reference
	t.Run("PreExistingNonControllingAndControllingOwnerReferences", func(t *testing.T) {
		controllee := newPod("controllee")

		otherObject := newPod("otherObject")
		setOwnerReference(otherObject, controllee, scheme.Scheme)

		oldController := newPod("old-controller")
		existingControllerRef := metav1.NewControllerRef(oldController, oldController.GetObjectKind().GroupVersionKind())
		controllee.OwnerReferences = append(controllee.OwnerReferences, *existingControllerRef)

		newController := newPod("new-controller")

		upsertControllerRef(controllee, newController)

		if len(controllee.GetOwnerReferences()) != 2 {
			t.Error("Expected two owner references, got:", len(controllee.GetOwnerReferences()))
		}
		if !metav1.IsControlledBy(controllee, newController) {
			t.Error("Expected controllee to be controlled by controller")
		}
	})
}

func addOwnerRef(client client.Client, object, owner client.Object) error {
	setOwnerReference(owner, object, client.Scheme())

	if err := client.Update(context.Background(), object); err != nil {
		return fmt.Errorf("failed to add owner to object: %v", err)
	}

	return nil
}

// isOwnedBy returns a boolean based upon whether the provided object "owned"
// has an owner reference for the provided object "owner".
func isOwnedBy(owned, owner metav1.Object) bool {
	// Retrieve the owner references from the owned object
	ownerReferences := owned.GetOwnerReferences()

	// Check if the owner references include the owner
	for _, ref := range ownerReferences {
		if ref.UID == owner.GetUID() {
			return true
		}
	}

	return false
}

// setOwnerReference is a helper function that wraps controllerutil.SetOwnerReference(...)
// with an //nolint:errcheck comment to suppress error check linters.
func setOwnerReference(owner, owned metav1.Object, scheme *runtime.Scheme) {
	//nolint:errcheck
	controllerutil.SetOwnerReference(owner, owned, scheme)
}

// setControllerReference is a helper function that wraps controllerutil.SetControllerReference(...)
// with an //nolint:errcheck comment to suppress error check linters.
func setControllerReference(owner, owned metav1.Object, scheme *runtime.Scheme) {
	//nolint:errcheck
	controllerutil.SetControllerReference(owner, owned, scheme)
}
