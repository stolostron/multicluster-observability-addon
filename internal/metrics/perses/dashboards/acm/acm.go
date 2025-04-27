package acm

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"

	"github.com/perses/community-dashboards/pkg/dashboards"
	"github.com/perses/community-dashboards/pkg/promql"

	labelValuesVar "github.com/perses/perses/go-sdk/prometheus/variable/label-values"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	panels "github.com/stolostron/multicluster-observability-addon/internal/metrics/perses/panels/acm"
)

func withControlPlaneHealthGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Control Plane Health",
		panelgroup.PanelsPerLine(2),
		panels.Top50MaxLatencyAPIServer(datasource, labelMatcher),
		panels.EtcdHealth(datasource, labelMatcher),
	)
}

// TODO: (@saswatamcode) Hook it into reconcile.
func BuildACMClustersOverview(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
	return dashboard.New("acm-clusters-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM / Clusters / Overview"),

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
				listVar.DisplayName("cluster"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		withControlPlaneHealthGroup(datasource, clusterLabelMatcher),
	)
}
