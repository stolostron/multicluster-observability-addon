package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	dl "github.com/stolostron/multicluster-observability-addon/pkg/perses/panels/datalinks"
)

// Cluster Dashboard - By Namespace

func ClusterByNamespaceOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterByNamespaceOverTime"],
			query.SeriesNameFormat("{{namespace}}"),
		),
		panel.AddQuery(query.PromQL(CardinalityQueries["ClusterNonNamespacedOverTime"],
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Non Namespaced"),
		)),
	)
}

func ClusterByNamespaceTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:   "namespace",
					Header: "Namespace",
					DataLink: &tablePanel.DataLink{
						URL:   dl.DashboardURL("acm-metrics-cardinality-cluster", dl.StaticParam("cluster", "$cluster"), dl.FieldParam("namespace", "namespace")),
						Title: "Drill down",
					},
				},
				{Name: "value", Header: "Namespace Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterByNamespaceTable"]),
	)
}

// Cluster Dashboard - By Pod

func ClusterByPodOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterByPodOverTime"],
			query.SeriesNameFormat("{{pod}}"),
		),
		panel.AddQuery(query.PromQL(CardinalityQueries["ClusterNoPodOverTime"],
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("No Pod"),
		)),
	)
}

func ClusterByPodTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:   "pod",
					Header: "Pod",
					DataLink: &tablePanel.DataLink{
						URL:   dl.DashboardURL("acm-metrics-cardinality-cluster", dl.StaticParam("cluster", "$cluster"), dl.StaticParam("namespace", "$namespace"), dl.FieldParam("pod", "pod")),
						Title: "Drill down",
					},
				},
				{Name: "value", Header: "Pod Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterByPodTable"]),
	)
}

// Cluster Dashboard - In Pod (by metric name)

func ClusterInPodOverTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cardinality Over Time",
		cardinalityTimeSeriesOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterInPodOverTime"],
			query.SeriesNameFormat("{{__name__}}"),
		),
	)
}

func ClusterInPodTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Current Cardinality",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{
					Name:   "__name__",
					Header: "Metric Name",
					DataLink: &tablePanel.DataLink{
						URL:   dl.DashboardURL("acm-metrics-cardinality-cluster", dl.StaticParam("cluster", "$cluster"), dl.StaticParam("namespace", "$namespace"), dl.StaticParam("pod", "$pod"), dl.FieldParam("metric_name", "__name__")),
						Title: "Drill down",
					},
				},
				{Name: "value", Header: "Metric Cardinality"},
			}),
			tablePanel.WithDensity("compact"),
		),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterInPodTable"]),
	)
}

// Cluster Dashboard - Raw Timeseries

func ClusterRawTimeseriesTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Raw Timeseries",
		cardinalityTableOptions(),
		addCardinalityQuery(datasourceName, CardinalityQueries["ClusterRawTimeseries"]),
	)
}
