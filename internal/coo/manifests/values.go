package manifests

import (
	"encoding/json"
	"log"

	"github.com/perses/perses/go-sdk/dashboard"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	mmanifests "github.com/stolostron/multicluster-observability-addon/internal/metrics/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	incident_management "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/incident-management"
)

var (
	project          = "open-cluster-management-observability"
	datasource       = "thanos-query-frontend"
	clusterLabelName = ""
)

type DashboardValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type DashboardBuilderFunc func(project string, datasource string, clusterLabelName string) (dashboard.Builder, error)

// DashboardBuilder is a struct that holds a dashboard builder function and its name
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

func buildACMDashboards() []DashboardValue {
	var dashboards []DashboardValue

	builders := []DashboardBuilder{
		{acm.BuildClusterResourceUse, "ClusterResourceUse"},
		{acm.BuildNodeResourceUse, "NodeResourceUse"},
		{acm.BuildACMOptimizationOverview, "ACMOptimizationOverview"},
		{acm.BuildACMClustersOverview, "ACMClustersOverview"},
	}

	for _, builder := range builders {
		db, err := builder.fn(project, datasource, clusterLabelName)
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

func buildIncidentDetetctionDashboards() []DashboardValue {
	var dashboards []DashboardValue

	builders := []DashboardBuilder{
		{incident_management.BuildACMIncidentsOverview, "IncidentDetectionOverview"},
	}

	for _, builder := range builders {
		db, err := builder.fn(project, datasource, clusterLabelName)
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

func BuildValues(opts addon.Options, installCOO bool, isHubCluster bool) *COOValues {
	metricsUI := mmanifests.EnableUI(opts.Platform.Metrics, isHubCluster)

	incidentDetection := imanifests.EnableUI(opts.Platform.AnalyticsOptions.IncidentDetection)

	var dashboards []DashboardValue

	monitoringUIPlugin := false
	if metricsUI != nil {
		monitoringUIPlugin = metricsUI.Enabled
		if metricsUI.Enabled {
			dashboards = append(dashboards, buildACMDashboards()...)
		}
	}

	if incidentDetection != nil {
		monitoringUIPlugin = monitoringUIPlugin || incidentDetection.Enabled
		if incidentDetection.Enabled {
			dashboards = append(dashboards, buildIncidentDetetctionDashboards()...)
		}
	}

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
