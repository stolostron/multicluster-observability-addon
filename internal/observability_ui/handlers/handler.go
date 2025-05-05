package handlers

import (
	"context"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/observability_ui/manifests"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, obs addon.ObsUIOptions) manifests.Options {
	opts := manifests.Options{
		Enabled:   obs.Enabled,
		LogsUI:    obs.Logs,
		MetricsUI: obs.Metrics,
	}
	return opts
}
