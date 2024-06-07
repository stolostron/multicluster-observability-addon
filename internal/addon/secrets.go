package addon

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Endpoint defines the name of an endpoint that will be available to store
// signal data.
type Endpoint string

// GetSecrets fetches Kubernetes secrets based on the specified
// secret name for each target in targetSecretName.
// If a secret doesn't exist in the namespace of the ManagedClusterAddon this
// function will instead look for it in the configResourceNamespace.
// If no secret is found an error will be returned.
func GetSecrets(ctx context.Context, k8s client.Client, configResourceNamespace string, addonNamespace string, targetSecretName map[Endpoint]string) (map[Endpoint]corev1.Secret, error) {
	secretKeys := make(map[Endpoint]corev1.Secret, len(targetSecretName))
	for targetName, secretName := range targetSecretName {
		secretReference := &corev1.Secret{}
		key := client.ObjectKey{Name: secretName, Namespace: addonNamespace}
		err := k8s.Get(ctx, key, secretReference, &client.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			key = client.ObjectKey{Name: secretName, Namespace: configResourceNamespace}
			err = k8s.Get(ctx, key, secretReference, &client.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get existing secret with key %s/%s: %w", key.Namespace, key.Name, err)
			}
		case err != nil:
			return nil, fmt.Errorf("failed to get existing secret with key %s/%s: %w", key.Namespace, key.Name, err)
		}
		secretKeys[targetName] = *secretReference
	}

	return secretKeys, nil
}
