package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	dl "github.com/stolostron/multicluster-observability-addon/pkg/perses/panels/datalinks"
)

func addCardinalityQuery(datasourceName string, expr string, opts ...query.Option) panel.Option {
	allOpts := append([]query.Option{dashboards.AddQueryDataSource(datasourceName)}, opts...)
	return panel.AddQuery(query.PromQL(expr, allOpts...))
}

func cardinalityStatOptions(threshold float64) panel.Option {
	return statPanel.Chart(
		statPanel.Calculation("last-number"),
		statPanel.Thresholds(commonSdk.Thresholds{
			DefaultColor: "green",
			Mode:         commonSdk.AbsoluteMode,
			Steps: []commonSdk.StepOption{
				{Value: threshold, Color: "red"},
			},
		}),
	)
}

func cardinalityTimeSeriesOptions() panel.Option {
	return timeSeriesPanel.Chart(
		timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
			Mode:     "list",
			Position: "bottom",
		}),
		timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
			Display:   "line",
			Palette:   &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
			LineWidth: 1,
		}),
	)
}

func cardinalityTableOptions() panel.Option {
	return tablePanel.Table(tablePanel.WithDensity("compact"))
}

// Overview - Outlier stat panels

func ClusterOutliersCount(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster Outliers",
		cardinalityStatOptions(1),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterOutliersCount"]),
	)
}

func MetricOutliersCount(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Metric Outliers",
		cardinalityStatOptions(1),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricOutliersCount"]),
	)
}

// Overview - Outlier table panels

func ClusterOutliersTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Top Clusters",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "cluster",
					Header:   "Cluster",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-cluster", "cluster", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterOutliersTable"]),
	)
}

func MetricOutliersTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Top Metrics",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "metric_name",
					Header:   "Metric Name",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-name", "metric_name", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricOutliersTable"]),
	)
}

// Overview - Cluster Cardinality section

func ClusterCardinalityOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterCardinalityOverTime"],
			query.SeriesNameFormat("{{cluster}}"),
		),
	)
}

func ClusterCardinalityTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "cluster",
					Header:   "Cluster",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-cluster", "cluster", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterCardinalityNow"]),
	)
}

// Overview - Metric Cardinality section

func MetricCardinalityOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricCardinalityOverTime"],
			query.SeriesNameFormat("{{metric_name}}"),
		),
	)
}

func MetricCardinalityTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "metric_name",
					Header:   "Metric Name",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-name", "metric_name", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["MetricCardinalityNow"]),
	)
}

// Overview - Global Recording Rules section

func GlobalRecordingRulesOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["GlobalRulesOverTime"],
			query.SeriesNameFormat("{{metric_name}}"),
		),
	)
}

func GlobalRecordingRulesTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:     "metric_name",
					Header:   "Metric Name",
					DataLink: dl.NewTableLinkNewTab("acm-metrics-cardinality-name", "metric_name", "Drill down"),
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["GlobalRulesNow"]),
	)
}

// Overview - Total Cardinality section

func TotalCardinalityOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["TotalCardinalityOverTime"],
			query.SeriesNameFormat("Total Cardinality"),
		),
	)
}
