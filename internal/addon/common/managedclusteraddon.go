package common

import (
	"errors"
	"fmt"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrInvalidConfigNamespace = errors.New("config reference namespace is not allowed")

// ValidateConfigNamespaces restricts desired configs to shared templates or the managed cluster's namespace.
// Validate the entire list before fetching any config, since MCA writers can override these references.
func ValidateConfigNamespaces(mcAddon *addonapiv1alpha1.ManagedClusterAddOn) error {
	for _, config := range mcAddon.Status.ConfigReferences {
		namespace := config.Namespace // Deprecated fallback, overridden below when DesiredConfig is set.
		name := config.Name
		switch {
		case config.DesiredConfig != nil:
			namespace = config.DesiredConfig.Namespace
			name = config.DesiredConfig.Name
		case name == "":
			// No DesiredConfig and no deprecated reference set: nothing is configured yet.
			continue
		}
		if namespace == addoncfg.InstallNamespace || (namespace != "" && namespace == mcAddon.Namespace) {
			continue
		}
		return fmt.Errorf("%w: %s/%s %q in namespace %q for ManagedClusterAddOn %s/%s; expected %q or %q",
			ErrInvalidConfigNamespace, config.Group, config.Resource, name, namespace,
			mcAddon.Namespace, mcAddon.Name, addoncfg.InstallNamespace, mcAddon.Namespace)
	}
	return nil
}

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
