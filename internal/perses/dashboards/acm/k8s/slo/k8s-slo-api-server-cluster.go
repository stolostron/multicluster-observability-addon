package slo

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/slo"
)

func withSLOOverviewGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Service-Level Overview - Kubernetes API Server Request Duration",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(4),
		panels.ClusterTarget(datasource),
		panels.ClusterPast7Days(datasource),
		panels.ClusterPast30Days(datasource),
	)
}

func withErrorBudget7dGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Error Budget for 7 Days",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(5),
		panels.ClusterDayOfWeek(datasource),
		panels.ClusterErrorBudget7d(datasource),
		panels.ClusterDowntimeRemaining7d(datasource),
	)
}

func withErrorBudget30dGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Error Budget for 30 Days",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(5),
		panels.ClusterDayOfMonth(datasource),
		panels.ClusterErrorBudget30d(datasource),
		panels.ClusterDowntimeRemaining30d(datasource),
	)
}

func withTrendGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Trend",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 24, H: 8},
			{X: 0, Y: 8, W: 24, H: 6},
		},
		panels.ClusterSLITrend(datasource),
		panels.ClusterSLITable(datasource),
	)
}

func BuildSLOAPIServerCluster(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-slo-api-server-cluster",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Service-Level Overview / API Server / Cluster"),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"sli:apiserver_request_duration_seconds:trend:1m",
							[]promql.LabelMatcher{},
						),
					),
				),
				listVar.DisplayName("Cluster"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		withSLOOverviewGroup(datasource),
		withErrorBudget7dGroup(datasource),
		withErrorBudget30dGroup(datasource),
		withTrendGroup(datasource),
	)
}
