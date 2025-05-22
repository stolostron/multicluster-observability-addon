package manifests

import (
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	mmanifests "github.com/stolostron/multicluster-observability-addon/internal/metrics/manifests"
)

type COOValues struct {
	Enabled            bool                                `json:"enabled"`
	InstallCOO         bool                                `json:"installCOO"`
	MonitoringUIPlugin bool                                `json:"monitoringUIPlugin"`
	Metrics            *mmanifests.UIValues                `json:"metrics,omitempty"`
	IncidentDetection  *imanifests.IncidentDetectionValues `json:"incidentDetection,omitempty"`
}

func BuildValues(opts addon.Options, installCOO bool, isHubCluster bool) *COOValues {
	metrics := mmanifests.EnableUI(opts.Platform.Metrics, isHubCluster)

	incidentDetection := imanifests.EnableUI(opts.Platform.AnalyticsOptions.IncidentDetection)

	// Decide if we need to create a monitoring UI plugin
	monitoringUIPlugin := false
	if metrics != nil {
		monitoringUIPlugin = monitoringUIPlugin || metrics.ACM.Enabled
	}
	if incidentDetection != nil {
		monitoringUIPlugin = monitoringUIPlugin || incidentDetection.Enabled
	}

	return &COOValues{
		// Decide if COO chart is needed
		Enabled:            monitoringUIPlugin,
		InstallCOO:         installCOO,
		MonitoringUIPlugin: monitoringUIPlugin,
		Metrics:            metrics,
		IncidentDetection:  incidentDetection,
	}
}
