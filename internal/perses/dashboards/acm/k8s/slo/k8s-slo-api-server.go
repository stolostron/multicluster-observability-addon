package slo

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/slo"
)

func withFleetOverviewGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Fleet Overview",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 12, H: 5},
			{X: 12, Y: 0, W: 12, H: 5},
			{X: 0, Y: 5, W: 24, H: 7},
		},
		panels.FleetClustersExceededSLO(datasource),
		panels.FleetClustersMeetingSLO(datasource),
		panels.FleetTopClusters(datasource),
	)
}

func withFleetSLITrendGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"API Server Request Duration - Status",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 24, H: 9},
		},
		panels.FleetSLITrend(datasource),
	)
}

func BuildSLOAPIServer(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-slo-api-server",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Service-Level Overview / API Server"),

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
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		acm.AddTextVariable("window", "7d", "Window"),
		acm.AddTextVariable("top", "20", "Top"),

		withFleetOverviewGroup(datasource),
		withFleetSLITrendGroup(datasource),
	)
}
