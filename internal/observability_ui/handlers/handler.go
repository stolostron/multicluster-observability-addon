package handlers

import (
	"context"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/observability_ui/manifests"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, obsUI addon.ObsUIOptions) manifests.Options {
	return manifests.Options{
		Enabled: obsUI.Enabled,
	}
}
