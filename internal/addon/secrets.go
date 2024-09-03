package addon

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSecrets fetches Kubernetes secrets based on the specified
// secret name for each target in `secretNames`.
// If a secret doesn't exist in the `addonNamespace` (addon refers to `ManagedClusterAddon` resource) this
// function will instead look for it in the `configResourceNamespace`.
// If no secret is found an error will be returned.
func GetSecrets(ctx context.Context, k8s client.Client, configResourceNamespace string, addonNamespace string, secretNames []string) ([]corev1.Secret, error) {
	secrets := make([]corev1.Secret, 0, len(secretNames))
	for _, secretName := range secretNames {
		secret := &corev1.Secret{}
		key := client.ObjectKey{Name: secretName, Namespace: addonNamespace}
		err := k8s.Get(ctx, key, secret, &client.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			key = client.ObjectKey{Name: secretName, Namespace: configResourceNamespace}
			err = k8s.Get(ctx, key, secret, &client.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get existing secret with key %s/%s: %w", key.Namespace, key.Name, err)
			}
		case err != nil:
			return nil, fmt.Errorf("failed to get existing secret with key %s/%s: %w", key.Namespace, key.Name, err)
		}
		secrets = append(secrets, *secret)
	}

	return secrets, nil
}
