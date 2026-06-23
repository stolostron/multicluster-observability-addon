package acm

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	panels "github.com/stolostron/multicluster-observability-addon/pkg/perses/panels/acm"
)

func BuildACMMetricsCardinalityOverview(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-metrics-cardinality-overview",
		dashboard.ProjectName(project),
		dashboard.Name("Metrics Cardinality Overview"),

		// Cardinality Outliers — custom grid: stat(3) + table(9) + stat(3) + table(9)
		AddCustomPanelGroup("Cardinality Outliers",
			[]GridItem{
				{X: 0, Y: 0, W: 3, H: 6},
				{X: 3, Y: 0, W: 9, H: 6},
				{X: 12, Y: 0, W: 3, H: 6},
				{X: 15, Y: 0, W: 9, H: 6},
			},
			panels.ClusterOutliersCount(datasource),
			panels.ClusterOutliersTable(datasource),
			panels.MetricOutliersCount(datasource),
			panels.MetricOutliersTable(datasource),
		),

		// Cluster Cardinality
		dashboard.AddPanelGroup("Cluster Cardinality",
			panelgroup.PanelsPerLine(2),
			panels.ClusterCardinalityOverTime(datasource),
			panels.ClusterCardinalityTable(datasource),
		),

		// Metric Cardinality
		dashboard.AddPanelGroup("Metric Cardinality",
			panelgroup.PanelsPerLine(2),
			panels.MetricCardinalityOverTime(datasource),
			panels.MetricCardinalityTable(datasource),
		),

		// Global Recording Rules Cardinality (collapsed)
		AddCustomPanelGroupCollapsed("Global Recording Rules Cardinality",
			[]GridItem{
				{X: 0, Y: 0, W: 10, H: 10},
				{X: 10, Y: 0, W: 14, H: 10},
			},
			panels.GlobalRecordingRulesOverTime(datasource),
			panels.GlobalRecordingRulesTable(datasource),
		),

		// Total Cardinality (collapsed)
		AddCustomPanelGroupCollapsed("Total Cardinality",
			[]GridItem{
				{X: 0, Y: 0, W: 24, H: 11},
			},
			panels.TotalCardinalityOverTime(datasource),
		),
	)
}
