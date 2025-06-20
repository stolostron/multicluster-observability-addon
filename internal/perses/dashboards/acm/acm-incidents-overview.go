package acm

import (
	"github.com/perses/community-dashboards/pkg/dashboards"
	"github.com/perses/community-dashboards/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm"
)

func withIncidentsGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Incidents",
		panelgroup.PanelsPerLine(1),
		panels.ActiveIncidents(datasource, labelMatcher),
		panels.IncidentCount(datasource, labelMatcher),
	)
}
func BuildACMIncidentsOverview(project string, datasource string, clusterLabelName string) dashboards.DashboardResult {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
	return dashboards.NewDashboardResult(
		dashboard.New("acm-incidents-overview",
			dashboard.ProjectName(project),
			dashboard.Name("ACM Incidents Overview"),

			dashboard.AddVariable("cluster",
				listVar.List(
					labelValuesVar.PrometheusLabelValues("name",
						dashboards.AddVariableDatasource(datasource),
						labelValuesVar.Matchers(
							promql.SetLabelMatchers(
								"acm_managed_cluster_labels",
								[]promql.LabelMatcher{},
							)),
					),
					listVar.DisplayName("Cluster"),
					listVar.AllowAllValue(true),
					listVar.AllowMultiple(true),
				),
			),

			withIncidentsGroup(datasource, clusterLabelMatcher),
		),
	).Component("acm")
}
