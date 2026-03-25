package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	"github.com/prometheus/prometheus/model/labels"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm"
)

func withCPUStatsGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Overview",
		panelgroup.PanelsPerLine(3),
		panels.CPUOverestimationPanel(datasource, labelMatcher),
		panels.CPURequestsCommitmentPanel(datasource, labelMatcher),
		panels.CPUUtilizationPanel(datasource, labelMatcher),
	)
}

func withCPUUsageGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Usage",
		panelgroup.PanelsPerLine(1),
		panels.CPUUsagePanel(datasource, labelMatcher),
	)
}

func withCPUQuotaGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Quota",
		panelgroup.PanelsPerLine(1),
		panels.CPUQuotaPanel(datasource, labelMatcher),
	)
}

func withMemoryStatsGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Overview",
		panelgroup.PanelsPerLine(3),
		panels.MemoryOverestimationPanel(datasource, labelMatcher),
		panels.MemoryRequestsCommitmentPanel(datasource, labelMatcher),
		panels.MemoryUtilizationPanel(datasource, labelMatcher),
	)
}

func withMemoryUsageGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Usage",
		panelgroup.PanelsPerLine(1),
		panels.MemoryUsagePanel(datasource, labelMatcher),
	)
}

func withMemoryQuotaGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Requests by Namespace",
		panelgroup.PanelsPerLine(1),
		panels.MemoryRequestsByNamespacePanel(datasource, labelMatcher),
	)
}

func withNetworkingGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("Networking",
		panelgroup.PanelsPerLine(1),
		panels.NetworkingCurrentStatusPanel(datasource, labelMatcher),
	)
}

func BuildACMOptimizationOverview(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcherV2(clusterLabelName)
	return dashboard.New("acm-optimization-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Resource Optimization / Cluster"),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("name",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_managed_cluster_labels{openshiftVersion_major!=\"3\"}",
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("cluster"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),
		withCPUStatsGroup(datasource, clusterLabelMatcher),
		withCPUUsageGroup(datasource, clusterLabelMatcher),
		withCPUQuotaGroup(datasource, clusterLabelMatcher),
		withMemoryStatsGroup(datasource, clusterLabelMatcher),
		withMemoryUsageGroup(datasource, clusterLabelMatcher),
		withMemoryQuotaGroup(datasource, clusterLabelMatcher),
		withNetworkingGroup(datasource, clusterLabelMatcher),
	)
}
