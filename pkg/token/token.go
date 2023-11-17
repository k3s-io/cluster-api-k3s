package token

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func Lookup(ctx context.Context, ctrlclient client.Client, clusterKey client.ObjectKey) (*string, error) {
	var s *corev1.Secret
	var err error

	if s, err = getSecret(ctx, ctrlclient, clusterKey); err != nil {
		return nil, fmt.Errorf("Failed to lookup token: %v", err)
	}
	if val, ok := s.Data["value"]; ok {
		ret := string(val)
		return &ret, nil
	}

	return nil, fmt.Errorf("Found token secret without value")
}

func Reconcile(ctx context.Context, ctrlclient client.Client, clusterKey client.ObjectKey, owner client.Object) error {
	var s *corev1.Secret
	var err error

	// Find the token secret
	if s, err = getSecret(ctx, ctrlclient, clusterKey); err != nil {
		if apierrors.IsNotFound(err) {
			// Secret does not exist, create it
			_, err = generateAndStore(ctx, ctrlclient, clusterKey, owner)
			return err
		}
	}

	// Secret exists
	// Ensure the secret has correct ownership; this is necessary because at one point, the secret was owned by KThreesConfig
	if !metav1.IsControlledBy(s, owner) {
		controllerutil.SetControllerReference(owner, s, ctrlclient.Scheme())
		if err := ctrlclient.Update(ctx, s); err != nil {
			return fmt.Errorf("Failed to update ownership of token: %v", err)
		}
	}

	return nil
}

// randomB64 generates a cryptographically secure random byte slice of length size and returns its base64 encoding
func randomB64(size int) (string, error) {
	token := make([]byte, size)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

// name returns the name of the token secret, computed by convention using the name of the cluster
func name(clusterName string) string {
	return fmt.Sprintf("%s-token", clusterName)
}

func getSecret(ctx context.Context, ctrlclient client.Client, clusterKey client.ObjectKey) (*corev1.Secret, error) {
	s := &corev1.Secret{}
	key := client.ObjectKey{
		Name:      name(clusterKey.Name),
		Namespace: clusterKey.Namespace,
	}
	if err := ctrlclient.Get(ctx, key, s); err != nil {
		return nil, err
	}

	return s, nil
}

func generateAndStore(ctx context.Context, ctrlclient client.Client, clusterKey client.ObjectKey, owner client.Object) (*string, error) {
	tokn, err := randomB64(16)
	if err != nil {
		return nil, fmt.Errorf("Failed to generate token: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name(clusterKey.Name),
			Namespace: clusterKey.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterKey.Name,
			},
		},
		Data: map[string][]byte{
			"value": []byte(tokn),
		},
		Type: clusterv1.ClusterSecretType,
	}

	controllerutil.SetControllerReference(owner, secret, ctrlclient.Scheme())

	// as secret creation and scope.Config status patch are not atomic operations
	// it is possible that secret creation happens but the config.Status patches are not applied
	if err := ctrlclient.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("Failed to store token: %v", err)
	}

	return &tokn, nil
}
