package common

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DefaultConfig struct {
	PlacementRef addonv1alpha1.PlacementRef
	Config       addonv1alpha1.AddOnConfig
}

// EnsureAddonConfig ensures that the provided configuration are present in the CMAO
// for each placement.
func EnsureAddonConfig(ctx context.Context, logger logr.Logger, k8s client.Client, configs []DefaultConfig) error {
	// CMAO is a shared object, using retry
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cmao := &addonv1alpha1.ClusterManagementAddOn{}
		if err := k8s.Get(ctx, types.NamespacedName{Name: addon.Name}, cmao); err != nil {
			return fmt.Errorf("failed to get ClusterManagementAddOn: %w", err)
		}

		desiredCmao := cmao.DeepCopy()
		ensureConfigsInAddon(desiredCmao, configs)
		if equality.Semantic.DeepEqual(cmao, desiredCmao) {
			return nil
		}

		err := k8s.Update(ctx, desiredCmao)
		if err == nil {
			logger.Info("ClusterManagementAddOn placement configurations updated with default configurations")
		}
		return err
	})

	if retryErr != nil {
		return fmt.Errorf("failed to update CMAO with default configs: %w", retryErr)
	}

	return nil
}

func ensureConfigsInAddon(cmao *addonv1alpha1.ClusterManagementAddOn, configs []DefaultConfig) {
	// Group configs by placement.
	placementConfigs := map[addonv1alpha1.PlacementRef][]addonv1alpha1.AddOnConfig{}
	for _, cfg := range configs {
		placementConfigs[cfg.PlacementRef] = append(placementConfigs[cfg.PlacementRef], cfg.Config)
	}

	// For each placement in CMAO, ensure configs are present.
	for i, placement := range cmao.Spec.InstallStrategy.Placements {
		desiredConfigs := placementConfigs[placement.PlacementRef]
		for _, cfg := range desiredConfigs {
			isPresent := slices.ContainsFunc(placement.Configs, func(e addonv1alpha1.AddOnConfig) bool {
				return e == cfg
			})

			if isPresent {
				continue
			}

			cmao.Spec.InstallStrategy.Placements[i].Configs = append(cmao.Spec.InstallStrategy.Placements[i].Configs, cfg)
		}
	}
}
