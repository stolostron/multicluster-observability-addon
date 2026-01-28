package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm"
)

func withCPUGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("CPU",
		panelgroup.PanelsPerLine(2),
		panels.CPUOverestimationPanel(datasource, labelMatcher),
		panels.CPUUsagePanel(datasource, labelMatcher),
		panels.CPURequestsCommitmentPanel(datasource, labelMatcher),
		panels.CPUUtilizationPanel(datasource, labelMatcher),
		panels.CPUQuotaPanel(datasource, labelMatcher),
	)
}

func withMemoryGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Memory",
		panelgroup.PanelsPerLine(2),
		panels.MemoryOverestimationPanel(datasource, labelMatcher),
		panels.MemoryUsagePanel(datasource, labelMatcher),
		panels.MemoryRequestsCommitmentPanel(datasource, labelMatcher),
		panels.MemoryUtilizationPanel(datasource, labelMatcher),
		panels.MemoryRequestsByNamespacePanel(datasource, labelMatcher),
	)
}

func withNetworkingGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Networking",
		panelgroup.PanelsPerLine(2),
		panels.NetworkingCurrentStatusPanel(datasource, labelMatcher),
	)
}

func BuildACMOptimizationOverview(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
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
		withCPUGroup(datasource, clusterLabelMatcher),
		withMemoryGroup(datasource, clusterLabelMatcher),
		withNetworkingGroup(datasource, clusterLabelMatcher),
	)
}
