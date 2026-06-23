package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	dl "github.com/stolostron/multicluster-observability-addon/pkg/perses/panels/datalinks"
)

// Name Dashboard - By Cluster for Metric

func NameByClusterOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["NameByClusterOverTime"]),
	)
}

func NameByClusterTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:   "cluster",
					Header: "Cluster",
					DataLink: &tablePanel.DataLink{
						URL:   dl.DashboardURL("acm-metrics-cardinality-name", dl.StaticParam("metric_name", "$metric_name"), dl.FieldParam("cluster", "cluster")),
						Title: "Drill down",
					},
				},
				{Name: "value", Header: "Cluster Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["NameByClusterTable"]),
	)
}

// Name Dashboard - By Namespace

func NameByNamespaceOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["NameByNamespaceOverTime"]),
		panel.AddQuery(query.PromQL(CardinalityQueries["NameNonNamespacedOverTime"],
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Non Namespaced"),
		)),
	)
}

func NameByNamespaceTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:   "namespace",
					Header: "Namespace",
					DataLink: &tablePanel.DataLink{
						URL:   dl.DashboardURL("acm-metrics-cardinality-name", dl.StaticParam("metric_name", "$metric_name"), dl.StaticParam("cluster", "$cluster"), dl.FieldParam("namespace", "namespace")),
						Title: "Drill down",
					},
				},
				{Name: "value", Header: "Namespace Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["NameByNamespaceTable"]),
	)
}

// Name Dashboard - By Pod

func NameByPodOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["NameByPodOverTime"],
			query.SeriesNameFormat("{{pod}}"),
		),
		panel.AddQuery(query.PromQL(CardinalityQueries["NameNoPodOverTime"],
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("No Pod"),
		)),
	)
}

func NameByPodTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:   "pod",
					Header: "Pod",
					DataLink: &tablePanel.DataLink{
						URL:   dl.DashboardURL("acm-metrics-cardinality-name", dl.StaticParam("metric_name", "$metric_name"), dl.StaticParam("cluster", "$cluster"), dl.StaticParam("namespace", "$namespace"), dl.FieldParam("pod", "pod")),
						Title: "Drill down",
					},
				},
				{Name: "value", Header: "Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["NameByPodTable"]),
	)
}

// Name Dashboard - Raw Timeseries

func NameRawTimeseriesTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Raw Timeseries",
		cardinalityTableOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["NameRawTimeseries"]),
	)
}
