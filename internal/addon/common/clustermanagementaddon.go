package common

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DefaultConfig struct {
	PlacementRef addonv1alpha1.PlacementRef
	Config       addonv1alpha1.AddOnConfig
}

func NewMCOAClusterManagementAddOn() *addonv1alpha1.ClusterManagementAddOn {
	return &addonv1alpha1.ClusterManagementAddOn{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterManagementAddOn",
			APIVersion: addonv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: addon.Name,
		},
	}
}

// HasCMAOOwnerReference returns true when the ClusterManagementAddOn is among the owners of the object
func HasCMAOOwnerReference(ctx context.Context, k8s client.Client, obj client.Object) (bool, error) {
	cmao := NewMCOAClusterManagementAddOn()
	if err := k8s.Get(ctx, client.ObjectKeyFromObject(cmao), cmao); err != nil {
		return false, fmt.Errorf("failed to get the ClusterManagementAddOn: %w", err)
	}
	isOwned, err := controllerutil.HasOwnerReference(obj.GetOwnerReferences(), cmao, k8s.Scheme())
	if err != nil {
		return false, fmt.Errorf("failed to check owner reference: %w", err)
	}
	return isOwned, nil
}

// EnsureAddonConfig ensures that the provided configurations are present in the CMAO
// for each placement.
func EnsureAddonConfig(ctx context.Context, logger logr.Logger, k8s client.Client, configs []DefaultConfig) error {
	// Get the current CMAO
	cmao := &addonv1alpha1.ClusterManagementAddOn{}
	if err := k8s.Get(ctx, types.NamespacedName{Name: addon.Name}, cmao); err != nil {
		return fmt.Errorf("failed to get ClusterManagementAddOn: %w", err)
	}

	desiredCmao := cmao.DeepCopy()
	desiredCmao.ManagedFields = nil // required for patching with ssa
	ensureConfigsInAddon(desiredCmao, configs)

	// If there are no changes, nothing to do
	if equality.Semantic.DeepEqual(cmao.Spec.InstallStrategy.Placements, desiredCmao.Spec.InstallStrategy.Placements) {
		return nil
	}

	if err := ServerSideApply(ctx, k8s, desiredCmao, nil); err != nil {
		return fmt.Errorf("failed to apply updated CMAO configuration: %w", err)
	}

	logger.Info("ClusterManagementAddOn placement configurations updated with default configurations",
		"name", desiredCmao.Name,
		"placementCount", len(desiredCmao.Spec.InstallStrategy.Placements))

	return nil
}

func ensureConfigsInAddon(cmao *addonv1alpha1.ClusterManagementAddOn, configs []DefaultConfig) {
	containsConfig := func(configs []addonv1alpha1.AddOnConfig, cfg addonv1alpha1.AddOnConfig) bool {
		return slices.ContainsFunc(configs, func(e addonv1alpha1.AddOnConfig) bool {
			return e == cfg
		})
	}

	// Group configs by placement.
	placementConfigs := map[addonv1alpha1.PlacementRef][]addonv1alpha1.AddOnConfig{}
	for _, cfg := range configs {
		if containsConfig(placementConfigs[cfg.PlacementRef], cfg.Config) {
			continue
		}
		placementConfigs[cfg.PlacementRef] = append(placementConfigs[cfg.PlacementRef], cfg.Config)
	}

	// For each placement in CMAO, ensure configs are present.
	for i, placement := range cmao.Spec.InstallStrategy.Placements {
		// Do not add configs to a placementRef if they are already present.
		desiredConfigs := placementConfigs[placement.PlacementRef]
		dedupConfigs := make([]addonv1alpha1.AddOnConfig, 0, len(desiredConfigs))
		for _, cfg := range desiredConfigs {
			if containsConfig(placement.Configs, cfg) {
				continue
			}
			dedupConfigs = append(dedupConfigs, cfg)
		}

		cmao.Spec.InstallStrategy.Placements[i].Configs = append(cmao.Spec.InstallStrategy.Placements[i].Configs, dedupConfigs...)
	}
}
