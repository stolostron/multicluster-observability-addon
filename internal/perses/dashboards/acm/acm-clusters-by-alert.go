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

func withRealTimeDataGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Real Time Data",
		panelgroup.PanelsPerLine(1),
		panels.ClustersWithAlertSeverity(datasource, labelMatcher),
	)
}

func BuildACMClustersByAlert(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
	return dashboard.New("acm-clusters-by-alert",
		dashboard.ProjectName(project),
		dashboard.Name("Clusters by Alert"),
		dashboard.AddVariable("alert",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("alertname",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"ALERTS",
							[]promql.LabelMatcher{},
						),
					),
				),
				listVar.DisplayName("Alert"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),
		dashboard.AddVariable("severity",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("severity",
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"ALERTS",
							[]promql.LabelMatcher{
								{Name: "alertname", Type: "=~", Value: "$alert"},
							},
						),
					),
					dashboards.AddVariableDatasource(datasource),
				),
				listVar.DisplayName("Severity"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(false),
			),
		),
		withRealTimeDataGroup(datasource, clusterLabelMatcher),
	)
}
