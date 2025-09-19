package manifests

import (
	"encoding/json"
	"log"

	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/perses/perses/go-sdk/dashboard"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	incident_management "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/incident-management"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	dsThanos         = "rbac-query-proxy-datasource"
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
	Perses             bool                                `json:"perses"`
	Dashboards         []DashboardValue                    `json:"dashboards,omitempty"`
	Metrics            *UIValues                           `json:"metrics,omitempty"`
	IncidentDetection  *imanifests.IncidentDetectionValues `json:"incidentDetection,omitempty"`
}

type UIValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts addon.Options, installCOO bool, isHubCluster bool) *COOValues {
	var dashboards []DashboardValue
	var incidentDetectionEnabled bool
	metricsUI := enableUI(opts.Platform.Metrics, isHubCluster)
	if metricsUI != nil {
		if metricsUI.Enabled {
			dashboards = append(dashboards, buildACMDashboards()...)
			dashboards = append(dashboards, buildK8sDashboards()...)
		}
	}

	incidentDetection := imanifests.EnableUI(opts.Platform.AnalyticsOptions.IncidentDetection)
	if incidentDetection != nil {
		if incidentDetection.Enabled {
			incidentDetectionEnabled = true
			if isHubCluster {
				dashboards = append(dashboards, buildIncidentDetetctionDashboards()...)
			}
		}
	}

	return &COOValues{
		// Decide if this chart is needed
		Enabled: len(dashboards) > 0 || incidentDetectionEnabled,
		// Decide if COO chart is needs to be installed
		InstallCOO:         installCOO,
		MonitoringUIPlugin: len(dashboards) > 0 || incidentDetectionEnabled,
		Perses:             len(dashboards) > 0,
		Dashboards:         dashboards,
		Metrics:            metricsUI,
		IncidentDetection:  incidentDetection,
	}
}

func enableUI(opts addon.MetricsOptions, isHub bool) *UIValues {
	if !opts.CollectionEnabled && !opts.UI.Enabled || !isHub {
		return nil
	}

	return &UIValues{
		Enabled: true,
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

func buildK8sDashboards() []DashboardValue {
	type dashboardBuilder struct {
		fn   func(string, string, string) ([]runtime.Object, error)
		name string
	}
	builders := []dashboardBuilder{
		{acm.BuildK8sDashboards, "Kubernetes"},
		{acm.BuildETCDDashboards, "ETCD"},
	}

	var dashboards []DashboardValue
	for _, builder := range builders {
		objs, err := builder.fn(config.InstallNamespace, dsThanos, clusterLabelName)
		if err != nil {
			log.Printf("Failed to build %s dashboards: %v", builder.name, err)
			continue
		}
		for _, obj := range objs {
			db, ok := obj.(*persesv1.PersesDashboard)
			if !ok {
				log.Printf("Failed to convert object to PersesDashboard: %v", obj)
				continue
			}
			data, err := json.Marshal(db.Spec)
			if err != nil {
				log.Printf("Failed to marshal %s dashboard: %v", builder.name, err)
				continue
			}
			dashboards = append(dashboards, DashboardValue{
				Name: db.Name,
				Data: string(data),
			})
		}
	}
	return dashboards
}
