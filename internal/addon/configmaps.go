package addon

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetConfigMaps fetches Kubernetes configMaps based on the specified
// configMap name for each target in `configMapNames`.
// If a configMap doesn't exist in the `addonNamespace` (addon refers to `ManagedClusterAddon` resource) this
// function will instead look for it in the `configResourceNamespace`.
// If no configMap is found an error will be returned.
func GetConfigMaps(ctx context.Context, k8s client.Client, configResourceNamespace string, addonNamespace string, configMapNames []string) ([]corev1.ConfigMap, error) {
	configMaps := []corev1.ConfigMap{}
	for _, configMapName := range configMapNames {
		configMap := &corev1.ConfigMap{}
		key := client.ObjectKey{Name: configMapName, Namespace: addonNamespace}
		err := k8s.Get(ctx, key, configMap, &client.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			key = client.ObjectKey{Name: configMapName, Namespace: configResourceNamespace}
			err = k8s.Get(ctx, key, configMap, &client.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get existing configMap with key %s/%s: %w", key.Namespace, key.Name, err)
			}
		case err != nil:
			return nil, fmt.Errorf("failed to get existing configMap with key %s/%s: %w", key.Namespace, key.Name, err)
		}
		configMaps = append(configMaps, *configMap)
	}

	return configMaps, nil
}
