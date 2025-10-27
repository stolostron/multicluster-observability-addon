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

func withControlPlaneHealthGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Control Plane Health",
		panelgroup.PanelsPerLine(2),
		panels.Top50MaxLatencyAPIServer(datasource, labelMatcher),
		panels.EtcdHealth(datasource, labelMatcher),
	)
}

func withOptimizationGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Optimization",
		panelgroup.PanelsPerLine(2),
		panels.Top50CPUOverEstimationClusters(datasource, labelMatcher),
		panels.Top50MemoryOverEstimationClusters(datasource, labelMatcher),
	)
}

func withCapacityGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Capacity",
		panelgroup.PanelsPerLine(2),
		panels.Top50MemoryUtilizedClusters(datasource, labelMatcher),
		panels.Top50CPUUtilizedClusters(datasource, labelMatcher),
		panels.Top5MemoryUtilizationGraph(datasource, labelMatcher),
		panels.Top5CPUUtilizationGraph(datasource, labelMatcher),
		panels.BandwidthUtilization(datasource, labelMatcher),
	)
}

func BuildACMClustersOverview(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
	return dashboard.New("acm-clusters-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Clusters Overview"),
		dashboard.AddVariable("acm_label_names",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("label_name",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						"acm_label_names",
					),
				),
				listVar.DisplayName("Label"),
				listVar.DefaultValue("name"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),
		dashboard.AddVariable("value",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("acm_label_names",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_managed_cluster_labels",
							[]promql.LabelMatcher{
								{Name: "acm_label_names", Type: "=", Value: "$acm_label_names"},
							},
						),
					),
				),
				listVar.DisplayName("Value"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		// Cluster variable - third level (depends on acm_label_names and value)
		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("name",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_managed_cluster_labels",
							[]promql.LabelMatcher{
								{Name: "acm_label_names", Type: "=~", Value: "$value"},
							},
						),
					),
				),
				listVar.DisplayName("Cluster"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
				listVar.Hidden(true),
			),
		),

		withControlPlaneHealthGroup(datasource, clusterLabelMatcher),
		withOptimizationGroup(datasource, clusterLabelMatcher),
		withCapacityGroup(datasource, clusterLabelMatcher),
	)
}
