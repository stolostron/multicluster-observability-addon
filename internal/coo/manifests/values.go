package manifests

import (
	"encoding/json"
	"log"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	mmanifests "github.com/stolostron/multicluster-observability-addon/internal/metrics/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
)

type DashboardValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type COOValues struct {
	Enabled            bool                                `json:"enabled"`
	InstallCOO         bool                                `json:"installCOO"`
	MonitoringUIPlugin bool                                `json:"monitoringUIPlugin"`
	Dashboards         []DashboardValue                    `json:"dashboards,omitempty"`
	Metrics            *mmanifests.UIValues                `json:"metrics,omitempty"`
	IncidentDetection  *imanifests.IncidentDetectionValues `json:"incidentDetection,omitempty"`
}

func buildACMDashboardValues(monitoringUIPlugin bool) []DashboardValue {
	if !monitoringUIPlugin {
		return nil
	}

	var dashboards []DashboardValue
	project := "open-cluster-management-observability"
	datasource := "thanos-query-frontend"
	clusterLabelName := ""

	// Generate cluster resource use dashboard only
	clusterDashboard, err := acm.BuildClusterResourceUse(project, datasource, clusterLabelName)
	if err != nil {
		return nil
	}
	// BuildClusterResourceUse returns dashboard.Builder, so we can access the Dashboard field directly
	clusterDashboardJSON, err := json.Marshal(clusterDashboard.Dashboard.Spec)
	if err == nil {
		dashboards = append(dashboards, DashboardValue{
			Name: clusterDashboard.Dashboard.Metadata.Name,
			Data: string(clusterDashboardJSON),
		})
	}
	// Generate node resource use dashboard only
	nodeDashboard, err := acm.BuildNodeResourceUse(project, datasource, clusterLabelName)
	if err != nil {
		return nil
	}
	nodeDashboardJSON, err := json.Marshal(nodeDashboard.Dashboard.Spec)
	if err == nil {
		dashboards = append(dashboards, DashboardValue{
			Name: nodeDashboard.Dashboard.Metadata.Name,
			Data: string(nodeDashboardJSON),
		})
	}

	return dashboards
}

func BuildValues(opts addon.Options, installCOO bool, isHubCluster bool) *COOValues {
	metricsUI := mmanifests.EnableUI(opts.Platform.Metrics, isHubCluster)

	incidentDetection := imanifests.EnableUI(opts.Platform.AnalyticsOptions.IncidentDetection)

	monitoringUIPlugin := false
	if metricsUI != nil {
		monitoringUIPlugin = metricsUI.Enabled
	}
	if incidentDetection != nil {
		monitoringUIPlugin = monitoringUIPlugin || incidentDetection.Enabled
	}
	var dashboards []DashboardValue
	if monitoringUIPlugin {
		dashboards = buildACMDashboardValues(monitoringUIPlugin)
	}

	// add a log here for debug
	log.Printf("COOValues: Enabled=%v, InstallCOO=%v, MonitoringUIPlugin=%v, Dashboards=%d, Metrics=%v, IncidentDetection=%v",
		monitoringUIPlugin, installCOO, monitoringUIPlugin, len(dashboards), metricsUI, incidentDetection)

	return &COOValues{
		// Decide if COO chart is needed
		Enabled:            monitoringUIPlugin,
		InstallCOO:         installCOO,
		MonitoringUIPlugin: monitoringUIPlugin,
		Dashboards:         dashboards,
		Metrics:            metricsUI,
		IncidentDetection:  incidentDetection,
	}
}
