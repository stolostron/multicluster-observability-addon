package compute

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// Cluster dashboard panels

func ClusterCPUUtilisation(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilisation",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 0.7, Color: "#EAB839"},
					{Value: 0.8, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["CPUUtilisation"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterCPURequestsCommitment(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Requests Commitment",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 0.7, Color: "#EAB839"},
					{Value: 0.8, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["CPURequestsCommitment"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterCPULimitsCommitment(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Limits Commitment",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 0.7, Color: "#EAB839"},
					{Value: 0.8, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["CPULimitsCommitment"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterMemoryUtilisation(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Utilisation",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 0.7, Color: "#EAB839"},
					{Value: 0.8, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["MemoryUtilisation"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterMemoryRequestsCommitment(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Requests Commitment",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 0.7, Color: "#EAB839"},
					{Value: 0.8, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["MemoryRequestsCommitment"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterMemoryLimitsCommitment(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Limits Commitment",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 0.7, Color: "#EAB839"},
					{Value: 0.8, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["MemoryLimitsCommitment"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterCPUUsage(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
				LineWidth:   0,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["CPUUsageByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterCPUQuota(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Quota",
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "namespace",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"namespace"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name: "__name__",
					Hide: true,
				},
				{
					Name: "cluster",
					Hide: true,
				},
				{
					Name: "clusterID",
					Hide: true,
				},
				{
					Name: "prometheus",
					Hide: true,
				},
				{
					Name: "receive",
					Hide: true,
				},
				{
					Name: "tenant_id",
					Hide: true,
				},
				{
					Name:   "namespace",
					Header: "Namespace",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "value #1",
					Header: "Pods",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 0,
					},
				},
				{
					Name:   "value #2",
					Header: "Workloads",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 0,
					},
				},
				{
					Name:   "value #3",
					Header: "CPU Usage",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "CPU Requests",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
					Header: "CPU Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #6",
					Header: "CPU Limits",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #7",
					Header: "CPU Limits %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
			}),
			tablePanel.WithDensity("compact"),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TablePodsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableWorkloadsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableCPUUsageByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableCPURequestsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableCPURequestsPercentByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableCPULimitsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableCPULimitsPercentByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterMemoryUsage(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage (w/o cache)",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
				LineWidth:   0,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["MemoryUsageByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterMemoryQuota(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Requests by Namespace",
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "namespace",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"namespace"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name: "__name__",
					Hide: true,
				},
				{
					Name: "cluster",
					Hide: true,
				},
				{
					Name: "clusterID",
					Hide: true,
				},
				{
					Name: "prometheus",
					Hide: true,
				},
				{
					Name: "receive",
					Hide: true,
				},
				{
					Name: "tenant_id",
					Hide: true,
				},
				{
					Name:   "namespace",
					Header: "Namespace",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "value #1",
					Header: "Pods",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 0,
					},
				},
				{
					Name:   "value #2",
					Header: "Workloads",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 0,
					},
				},
				{
					Name:   "value #3",
					Header: "Memory Usage",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Memory Requests",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
					Header: "Memory Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #6",
					Header: "Memory Limits",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #7",
					Header: "Memory Limits %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
			}),
			tablePanel.WithDensity("compact"),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TablePodsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableWorkloadsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableMemoryUsageByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableMemoryRequestsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableMemoryRequestsPercentByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableMemoryLimitsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableMemoryLimitsPercentByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
