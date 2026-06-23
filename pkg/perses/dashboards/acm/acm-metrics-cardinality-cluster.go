package acm

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	panels "github.com/stolostron/multicluster-observability-addon/pkg/perses/panels/acm"
)

func BuildACMMetricsCardinalityCluster(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-metrics-cardinality-cluster",
		dashboard.ProjectName(project),
		dashboard.Name("Metrics Cardinality / Cluster / Namespace / Pod"),

		// Variables: cluster → namespace → pod → metric_name
		GetCardinalityClusterVariable(datasource),
		GetCardinalityNamespaceVariable(datasource),
		GetCardinalityPodVariable(datasource),
		GetCardinalityPodMetricVariable(datasource),

		// By Namespace
		dashboard.AddPanelGroup("Cardinality by Namespace",
			panelgroup.PanelsPerLine(2),
			panels.ClusterByNamespaceOverTime(datasource),
			panels.ClusterByNamespaceTable(datasource),
		),

		// By Pod
		dashboard.AddPanelGroup("Cardinality by Pod",
			panelgroup.PanelsPerLine(2),
			panels.ClusterByPodOverTime(datasource),
			panels.ClusterByPodTable(datasource),
		),

		// In Pod (by metric name)
		dashboard.AddPanelGroup("Cardinality in Pod",
			panelgroup.PanelsPerLine(2),
			panels.ClusterInPodOverTime(datasource),
			panels.ClusterInPodTable(datasource),
		),

		// Raw Timeseries
		dashboard.AddPanelGroup("Raw Timeseries",
			panelgroup.PanelsPerLine(1),
			panels.ClusterRawTimeseriesTable(datasource),
		),
	)
}
