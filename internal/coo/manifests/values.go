package manifests

import (
	"encoding/json"
	"log"

	"github.com/perses/perses/go-sdk/dashboard"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	mmanifests "github.com/stolostron/multicluster-observability-addon/internal/metrics/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	incident_management "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/incident-management"
)

var (
	dsThanos         = "thanos-query-frontend"
	clusterLabelName = ""
)

type DashboardValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type DashboardBuilderFunc func(project string, datasource string, clusterLabelName string) (dashboard.Builder, error)

type DashboardBuilder struct {
	fn   DashboardBuilderFunc
	name string
}

type COOValues struct {
	Enabled            bool                                `json:"enabled"`
	InstallCOO         bool                                `json:"installCOO"`
	MonitoringUIPlugin bool                                `json:"monitoringUIPlugin"`
	Dashboards         []DashboardValue                    `json:"dashboards,omitempty"`
	Metrics            *mmanifests.UIValues                `json:"metrics,omitempty"`
	IncidentDetection  *imanifests.IncidentDetectionValues `json:"incidentDetection,omitempty"`
}

func BuildValues(opts addon.Options, installCOO bool, isHubCluster bool) *COOValues {
	var dashboards []DashboardValue

	metricsUI := mmanifests.EnableUI(opts.Platform.Metrics, isHubCluster)
	if metricsUI != nil {
		if metricsUI.Enabled {
			dashboards = append(dashboards, buildACMDashboards()...)
		}
	}

	incidentDetection := imanifests.EnableUI(opts.Platform.AnalyticsOptions.IncidentDetection)
	if incidentDetection != nil {
		if incidentDetection.Enabled {
			dashboards = append(dashboards, buildIncidentDetetctionDashboards()...)
		}
	}

	return &COOValues{
		// Decide if this chart is needed
		Enabled: len(dashboards) > 0,
		// Decide if COO chart is needs to be installed
		InstallCOO:         installCOO,
		MonitoringUIPlugin: len(dashboards) > 0,
		Dashboards:         dashboards,
		Metrics:            metricsUI,
		IncidentDetection:  incidentDetection,
	}
}

func buildDashboards(builders []DashboardBuilder, datasource string) []DashboardValue {
	var dashboards []DashboardValue

	for _, builder := range builders {
		db, err := builder.fn(config.InstallNamespace, datasource, clusterLabelName)
		if err != nil {
			log.Printf("Failed to build %s dashboard: %v", builder.name, err)
			continue
		}
		data, err := json.Marshal(db.Dashboard.Spec)
		if err != nil {
			log.Printf("Failed to marshal %s dashboard: %v", builder.name, err)
			continue
		}
		dashboards = append(dashboards, DashboardValue{
			Name: db.Dashboard.Metadata.Name,
			Data: string(data),
		})
	}

	return dashboards
}

func buildACMDashboards() []DashboardValue {
	builders := []DashboardBuilder{
		{acm.BuildClusterResourceUse, "ClusterResourceUse"},
		{acm.BuildNodeResourceUse, "NodeResourceUse"},
		{acm.BuildACMOptimizationOverview, "ACMOptimizationOverview"},
		{acm.BuildACMClustersOverview, "ACMClustersOverview"},
		{acm.BuildACMAlertAnalysis, "ACMAlertAnalysis"},
		{acm.BuildACMAlertsByCluster, "ACMAlertsByCluster"},
		{acm.BuildACMClustersByAlert, "ACMClustersByAlert"},
	}

	return buildDashboards(builders, dsThanos)
}

func buildIncidentDetetctionDashboards() []DashboardValue {
	builders := []DashboardBuilder{
		{incident_management.BuildACMIncidentsOverview, "IncidentDetectionOverview"},
	}

	return buildDashboards(builders, dsThanos)
}
