package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

// ApplyManagedClusterAddOnConfigs reconciles spec.configs on the ManagedClusterAddOn in
// clusterNamespace for a single group/resource. Matching configs not in desired are removed;
// desired configs are added. Other configs are left untouched.
//
// If the ManagedClusterAddOn does not exist and desired is empty, this is a no-op. If desired is
// non-empty, the NotFound error is returned so the caller can requeue.
func ApplyManagedClusterAddOnConfigs(ctx context.Context, logger logr.Logger, k8s client.Client, clusterNamespace string, desired []addonapiv1beta1.AddOnConfig, group, resource string) error {
	mcAddon := &addonapiv1beta1.ManagedClusterAddOn{}
	key := types.NamespacedName{Name: addoncfg.Name, Namespace: clusterNamespace}
	if err := k8s.Get(ctx, key, mcAddon); err != nil {
		if apierrors.IsNotFound(err) {
			if len(desired) == 0 {
				return nil
			}
			return err
		}
		return fmt.Errorf("failed to get ManagedClusterAddOn %s/%s: %w", clusterNamespace, addoncfg.Name, err)
	}

	filtered := make([]addonapiv1beta1.AddOnConfig, 0, len(mcAddon.Spec.Configs))
	for _, cfg := range mcAddon.Spec.Configs {
		if cfg.Group == group && cfg.Resource == resource {
			continue
		}
		filtered = append(filtered, cfg)
	}
	for _, cfg := range desired {
		if containsAddOnConfig(filtered, cfg) {
			continue
		}
		filtered = append(filtered, cfg)
	}

	if equality.Semantic.DeepEqual(mcAddon.Spec.Configs, filtered) {
		return nil
	}

	desiredAddon := mcAddon.DeepCopy()
	desiredAddon.Spec.Configs = filtered
	if err := k8s.Patch(ctx, desiredAddon, client.MergeFrom(mcAddon)); err != nil {
		return fmt.Errorf("failed to patch ManagedClusterAddOn %s/%s configs: %w", clusterNamespace, addoncfg.Name, err)
	}

	logger.Info("ManagedClusterAddOn configs updated",
		"name", addoncfg.Name,
		"namespace", clusterNamespace,
		"group", group,
		"resource", resource,
		"desiredCount", len(desired))

	return nil
}
