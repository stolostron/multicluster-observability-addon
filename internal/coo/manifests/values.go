package manifests

import (
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
)

// COOValues contains only the flags that remaining Helm templates (COO
// subscription/operatorgroup) still need. Hub-only Perses resources are
// now reconciled directly by HubResourceReconciler.
type COOValues struct {
	Enabled    bool `json:"enabled"`
	InstallCOO bool `json:"installCOO"`
}

type UIValues struct {
	Enabled bool `json:"enabled"`
}

// BuildValues constructs COO Helm values from addon options.
// Hub-only Perses resources (dashboards, datasources, UIPlugin) are now
// reconciled directly by HubResourceReconciler; this function only computes
// the flags that remaining Helm templates (COO subscription) still need.
// installCOOIsNeeded is the caller's resolved decision on whether MCOA should
// install and manage its own COO subscription on this cluster.
func BuildValues(opts addon.Options, installCOOIsNeeded bool, isHubCluster bool, hasCardinalityRules bool) *COOValues {
	var incidentDetectionEnabled bool
	var rightSizingEnabled bool
	metricsUI := EnableUI(opts.Platform.Metrics, isHubCluster)
	hasDashboards := metricsUI != nil && metricsUI.Enabled

	incidentDetection := imanifests.EnableUI(opts.Platform.AnalyticsOptions.IncidentDetection)
	if incidentDetection != nil && incidentDetection.Enabled {
		incidentDetectionEnabled = true
	}

	if isHubCluster && opts.Platform.AnalyticsOptions.RightSizing.Delegated {
		if opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled {
			rightSizingEnabled = true
		}
		if opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled {
			rightSizingEnabled = true
		}
	}

	var installCOO bool
	if (metricsUI != nil && metricsUI.Enabled) || incidentDetectionEnabled || rightSizingEnabled {
		installCOO = installCOOIsNeeded
	}

	return &COOValues{
		Enabled:    hasDashboards || incidentDetectionEnabled || rightSizingEnabled,
		InstallCOO: installCOO,
	}
}

func EnableUI(opts addon.MetricsOptions, isHub bool) *UIValues {
	if !isHub {
		return nil
	}

	if !opts.CollectionEnabled || !opts.UI.Enabled {
		return nil
	}

	return &UIValues{
		Enabled: true,
	}
}
