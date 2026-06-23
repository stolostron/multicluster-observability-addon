package acm

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	panels "github.com/stolostron/multicluster-observability-addon/pkg/perses/panels/acm"
)

func BuildACMMetricsCardinalityName(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-metrics-cardinality-name",
		dashboard.ProjectName(project),
		dashboard.Name("Metric Name Cardinality / Cluster / Namespace / Pod"),

		// Variables: metric_name → cluster → namespace → pod
		GetCardinalityMetricNameVariable(datasource),
		GetCardinalityClusterForMetricVariable(datasource),
		GetCardinalityNamespaceForNameVariable(datasource),
		GetCardinalityPodForNameVariable(datasource),

		// By Cluster for Metric
		dashboard.AddPanelGroup("Metric Cardinality by Cluster",
			panelgroup.PanelsPerLine(2),
			panels.NameByClusterOverTime(datasource),
			panels.NameByClusterTable(datasource),
		),

		// By Namespace
		dashboard.AddPanelGroup("Metric Cardinality by Namespace",
			panelgroup.PanelsPerLine(2),
			panels.NameByNamespaceOverTime(datasource),
			panels.NameByNamespaceTable(datasource),
		),

		// By Pod
		dashboard.AddPanelGroup("Metric Cardinality by Pod",
			panelgroup.PanelsPerLine(2),
			panels.NameByPodOverTime(datasource),
			panels.NameByPodTable(datasource),
		),

		// Raw Timeseries
		dashboard.AddPanelGroup("Raw Timeseries",
			panelgroup.PanelsPerLine(1),
			panels.NameRawTimeseriesTable(datasource),
		),
	)
}
