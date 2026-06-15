package common

import (
	"context"
	"errors"
	"fmt"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrMissingAODCRef  = errors.New("missing required AddOnDeploymentConfig reference in addon configuration")
	ErrMultipleAODCRef = errors.New("multiple AddOnDeploymentConfig references found - only one is supported")
)

func GetObjectKeys(configRef []addonapiv1beta1.ConfigReference, group, resource string) []client.ObjectKey {
	var keys []client.ObjectKey
	for _, config := range configRef {
		if config.Group != group {
			continue
		}
		if config.Resource != resource {
			continue
		}
		if config.DesiredConfig == nil {
			continue
		}

		keys = append(keys, client.ObjectKey{
			Name:      config.DesiredConfig.Name,
			Namespace: config.DesiredConfig.Namespace,
		})

	}
	return keys
}

func GetAddOnDeploymentConfig(ctx context.Context, getter addonutils.AddOnDeploymentConfigGetter, mcAddon *addonapiv1beta1.ManagedClusterAddOn) (*addonapiv1beta1.AddOnDeploymentConfig, error) {
	keys := GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addoncfg.AddonDeploymentConfigResource)
	switch {
	case len(keys) == 0:
		return nil, ErrMissingAODCRef
	case len(keys) > 1:
		return nil, ErrMultipleAODCRef
	}

	aodc, err := getter.Get(ctx, keys[0].Namespace, keys[0].Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get AddOnDeploymentConfig %s/%s: %w", keys[0].Namespace, keys[0].Name, err)
	}
	return aodc, nil
}
