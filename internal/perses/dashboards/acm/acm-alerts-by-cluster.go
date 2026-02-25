package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/model/labels"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm"
)

func withAlertSeverityGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("Alert Severity",
		panelgroup.PanelsPerLine(1),
		panels.AlertSeverity(datasource, labelMatcher),
	)
}

func withAlertsByClusterTrendsGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("Alert Trends",
		panelgroup.PanelsPerLine(2),
		panels.FiringAlertsTrend(datasource, labelMatcher),
		panels.PendingAlertsTrend(datasource, labelMatcher),
	)
}

func withAlertTimeSeriesGroup(datasource string, labelMatcher *labels.Matcher) dashboard.Option {
	return dashboard.AddPanelGroup("Alert Time Series",
		panelgroup.PanelsPerLine(1),
		panels.AlertsOverTime(datasource, labelMatcher),
	)
}

func BuildACMAlertsByCluster(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcherV2(clusterLabelName)
	return dashboard.New("acm-alerts-by-cluster",
		dashboard.ProjectName(project),
		dashboard.Name("Alerts by Cluster"),
		dashboard.AddVariable("acm_label_names",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("label_name",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_label_names",
							[]promql.LabelMatcher{},
						),
					),	
				),
				listVar.DisplayName("Label"),
				listVar.DefaultValue("cloud"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),
		dashboard.AddVariable("value",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("$acm_label_names",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchersV2(
							vector.New(vector.WithMetricName("acm_managed_cluster_labels")),
							[]*labels.Matcher{},
						).Pretty(0),
					),
				),
				listVar.DisplayName("Value"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),
		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("name",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_managed_cluster_labels",
							[]promql.LabelMatcher{
								{Name: "$acm_label_names", Type: "=~", Value: "$value"},
							},
						),
					),
				),
				listVar.DisplayName("Cluster"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),
		dashboard.AddVariable("severity",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("severity",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"ALERTS",
							[]promql.LabelMatcher{
								{Name: "cluster", Type: "=", Value: "$cluster"},
							},
						),
					),
				),
				listVar.DisplayName("Severity"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(false),
				listVar.Hidden(true),
			),
		),
		withAlertSeverityGroup(datasource, clusterLabelMatcher),
		withAlertsByClusterTrendsGroup(datasource, clusterLabelMatcher),
		withAlertTimeSeriesGroup(datasource, clusterLabelMatcher),
	)
}
