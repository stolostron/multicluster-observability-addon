package manifests

import (
	"encoding/json"
	"log"

	"github.com/perses/perses/go-sdk/dashboard"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	imanifests "github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	hcp "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm/hosted-control-plane"
	apiserver "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm/k8s/apiserver"
	compute "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm/k8s/compute"
	etcd "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm/k8s/etcd"
	networking "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm/k8s/networking"
	slo "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm/k8s/slo"
	incident_management "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/incident-management"
	rsperses "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/rightsizing"
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
	Enabled            bool `json:"enabled"`
	InstallCOO         bool `json:"installCOO"`
	MonitoringUIPlugin bool `json:"monitoringUIPlugin"`
	Perses             bool `json:"perses"`
	// omitempty removed: when no regular dashboards are needed, the key must
	// still appear in the serialized JSON so Helm uses the empty list instead
	// of falling back to the default in values.yaml.
	Dashboards          []DashboardValue                    `json:"dashboards"`
	AnalyticsDashboards []DashboardValue                    `json:"analyticsDashboards,omitempty"`
	Metrics             *UIValues                           `json:"metrics,omitempty"`
	IncidentDetection   *imanifests.IncidentDetectionValues `json:"incidentDetection,omitempty"`
}

type UIValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts addon.Options, installOfCOOOnTheHubIsNeeded bool, isHubCluster bool) *COOValues {
	var dashboards []DashboardValue
	var incidentDetectionEnabled bool
	var rightSizingEnabled bool
	metricsUI := enableUI(opts.Platform.Metrics, isHubCluster)
	if metricsUI != nil {
		if metricsUI.Enabled {
			dashboards = append(dashboards, buildACMDashboards()...)
		}
	}

	incidentDetection := imanifests.EnableUI(opts.Platform.AnalyticsOptions.IncidentDetection)
	if incidentDetection != nil {
		if incidentDetection.Enabled {
			incidentDetectionEnabled = true
		}
	}

	var analyticsDashboards []DashboardValue
	if isHubCluster {
		if incidentDetectionEnabled {
			analyticsDashboards = append(analyticsDashboards, buildIncidentDetetctionDashboards()...)
		}
		if opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled ||
			opts.Platform.AnalyticsOptions.RightSizing.PredictionEnabled {
			rightSizingEnabled = true
			analyticsDashboards = append(analyticsDashboards, buildNamespaceRSDashboards()...)
			analyticsDashboards = append(analyticsDashboards, buildForecastingDashboards()...)
		}
		if opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled {
			rightSizingEnabled = true
			analyticsDashboards = append(analyticsDashboards, buildVMRSDashboards()...)
		}
	}

	var installCOO bool
	if (metricsUI != nil && metricsUI.Enabled) || incidentDetectionEnabled || rightSizingEnabled {
		if isHubCluster {
			installCOO = installOfCOOOnTheHubIsNeeded
		} else {
			installCOO = true
		}
	}

	return &COOValues{
		Enabled:             len(dashboards) > 0 || len(analyticsDashboards) > 0 || incidentDetectionEnabled,
		InstallCOO:          installCOO,
		MonitoringUIPlugin:  len(dashboards) > 0 || len(analyticsDashboards) > 0 || incidentDetectionEnabled,
		Perses:              len(dashboards) > 0 || len(analyticsDashboards) > 0,
		Dashboards:          dashboards,
		AnalyticsDashboards: analyticsDashboards,
		Metrics:             metricsUI,
		IncidentDetection:   incidentDetection,
	}
}

func enableUI(opts addon.MetricsOptions, isHub bool) *UIValues {
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

func buildDashboards(builders []DashboardBuilder, datasource string, project string) []DashboardValue {
	var dashboards []DashboardValue

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

func buildACMDashboards() []DashboardValue {
	builders := []DashboardBuilder{
		{acm.BuildClusterResourceUse, "ClusterResourceUse"},
		{acm.BuildNodeResourceUse, "NodeResourceUse"},
		{acm.BuildACMOptimizationOverview, "ACMOptimizationOverview"},
		{acm.BuildACMClustersOverview, "ACMClustersOverview"},
		{acm.BuildACMAlertAnalysis, "ACMAlertAnalysis"},
		{acm.BuildACMAlertsByCluster, "ACMAlertsByCluster"},
		{acm.BuildACMClustersByAlert, "ACMClustersByAlert"},
		{hcp.BuildACMHCPOverview, "ACMHCPOverview"},
		{hcp.BuildACMHCPResources, "ACMHCPResources"},
		{apiserver.BuildAPIServerOverview, "APIServerOverview"},
		{etcd.BuildETCDOverview, "ETCDOverview"},
		{slo.BuildSLOAPIServer, "SLOAPIServer"},
		{slo.BuildSLOAPIServerCluster, "SLOAPIServerCluster"},
		{networking.BuildNetworkingCluster, "NetworkingCluster"},
		{networking.BuildNetworkingNamespacePods, "NetworkingNamespacePods"},
		{networking.BuildNetworkingNode, "NetworkingNode"},
		{networking.BuildNetworkingPod, "NetworkingPod"},
		{compute.BuildComputeCluster, "ComputeCluster"},
		{compute.BuildComputeNamespacePods, "ComputeNamespacePods"},
		{compute.BuildComputeNamespaceWorkloads, "ComputeNamespaceWorkloads"},
		{compute.BuildComputeNodePods, "ComputeNodePods"},
		{compute.BuildComputePod, "ComputePod"},
		{compute.BuildComputeWorkload, "ComputeWorkload"},
	}

	return buildDashboards(builders, dsThanos, config.InstallNamespace)
}

func buildIncidentDetetctionDashboards() []DashboardValue {
	builders := []DashboardBuilder{
		{incident_management.BuildACMIncidentsOverview, "IncidentDetectionOverview"},
	}

	return buildDashboards(builders, dsThanos, config.AnalyticsNamespace)
}

func buildNamespaceRSDashboards() []DashboardValue {
	builders := []DashboardBuilder{
		{func(project, datasource, clusterLabelName string) (dashboard.Builder, error) {
			return rsperses.BuildNamespaceRightSizing(project, datasource, clusterLabelName)
		}, "NamespaceRightSizing"},
	}

	return buildDashboards(builders, dsThanos, config.AnalyticsNamespace)
}

func buildForecastingDashboards() []DashboardValue {
	builders := []DashboardBuilder{
		{rsperses.BuildForecasting, "Forecasting"},
	}

	return buildDashboards(builders, dsThanos, config.AnalyticsNamespace)
}

func buildVMRSDashboards() []DashboardValue {
	builders := []DashboardBuilder{
		{rsperses.BuildVMOverview, "VMRightSizingOverview"},
		{rsperses.BuildVMOverestimation, "VMOverestimation"},
		{rsperses.BuildVMUnderestimation, "VMUnderestimation"},
	}

	return buildDashboards(builders, dsThanos, config.AnalyticsNamespace)
}
