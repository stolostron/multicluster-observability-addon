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

func withAlertSummaryGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Alert Summary",
		panelgroup.PanelsPerLine(6),
		panels.TotalAlerts(datasource, labelMatcher),
		panels.TotalCriticalAlerts(datasource, labelMatcher),
		panels.TotalWarningAlerts(datasource, labelMatcher),
		panels.TotalModerateAlerts(datasource, labelMatcher),
		panels.TotalLowAlerts(datasource, labelMatcher),
		panels.TotalImportantAlerts(datasource, labelMatcher),
	)
}

func withAlertTrendsGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Alert Trends",
		panelgroup.PanelsPerLine(2),
		panels.AlertTypeOverTime(datasource, labelMatcher),
		panels.ClusterAffectedOverTime(datasource, labelMatcher),
	)
}

func withAlertDetailsGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Alert Details",
		panelgroup.PanelsPerLine(1),
		panels.AlertsAndClusters(datasource, labelMatcher),
	)
}

func withHistoricalAnalysisGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Historical Analysis",
		panelgroup.PanelsPerLine(2),
		panels.MostFiringAlerts(datasource, labelMatcher),
		panels.MostAffectedClusters(datasource, labelMatcher),
	)
}

func BuildACMAlertAnalysis(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
	return dashboard.New("acm-alert-analysis",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Alert Analysis"),
		dashboard.AddVariable("severity",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("severity",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"ALERTS",
							[]promql.LabelMatcher{},
						),
					),
				),
				listVar.DisplayName("Severity"),
				listVar.Description("Policy severity level"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),
		withAlertSummaryGroup(datasource, clusterLabelMatcher),
		withAlertTrendsGroup(datasource, clusterLabelMatcher),
		withAlertDetailsGroup(datasource, clusterLabelMatcher),
		withHistoricalAnalysisGroup(datasource, clusterLabelMatcher),
	)
}
