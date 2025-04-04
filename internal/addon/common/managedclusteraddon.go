package common

import (
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetObjectKeys(configRef []addonapiv1alpha1.ConfigReference, group, resource string) []client.ObjectKey {
	var keys []client.ObjectKey
	for _, config := range configRef {
		if config.Group != group {
			continue
		}
		if config.Resource != resource {
			continue
		}

		keys = append(keys, client.ObjectKey{
			Name:      config.Name,
			Namespace: config.Namespace,
		})
	}
	return keys
}
