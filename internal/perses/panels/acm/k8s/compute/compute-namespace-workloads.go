package compute

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	dl "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/datalinks"
)

// Namespace (Workloads) dashboard panels

func NamespaceWorkloadsCPUUsage(datasource string) panelgroup.Option {
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
				NamespaceWorkloadsQueries["CPUUsageByWorkload"].Pretty(0),
				query.SeriesNameFormat("{{ workload }} - {{ workload_type }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespaceWorkloadsCPUQuota(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Quota",
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "workload",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"workload"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:     "workload",
					Header:   "Workload",
					Align:    tablePanel.LeftAlign,
					DataLink: dl.NewTableLink("k8s-compute-resources-workload", "workload", "Drill down to workload"),
				},
				{
					Name:   "value #1",
					Header: "Running Pods",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						DecimalPlaces: 0,
					},
				},
				{
					Name:   "value #2",
					Header: "CPU Usage",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "CPU Requests",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "CPU Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
					Header: "CPU Limits",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #6",
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
				NamespaceWorkloadsQueries["TableRunningPods"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableCPUUsageByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableCPURequestsByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableCPURequestsPercentByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableCPULimitsByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableCPULimitsPercentByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespaceWorkloadsMemoryUsage(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage",
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
				NamespaceWorkloadsQueries["MemoryUsageByWorkload"].Pretty(0),
				query.SeriesNameFormat("{{ workload }} - {{ workload_type }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespaceWorkloadsMemoryQuota(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Quota",
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "workload",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"workload"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:     "workload",
					Header:   "Workload",
					Align:    tablePanel.LeftAlign,
					DataLink: dl.NewTableLink("k8s-compute-resources-workload", "workload", "Drill down to workload"),
				},
				{
					Name:   "value #1",
					Header: "Running Pods",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						DecimalPlaces: 0,
					},
				},
				{
					Name:   "value #2",
					Header: "Memory Usage",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "Memory Requests",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Memory Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
					Header: "Memory Limits",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #6",
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
				NamespaceWorkloadsQueries["TableRunningPods"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableMemoryUsageByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableMemoryRequestsByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableMemoryRequestsPercentByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableMemoryLimitsByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespaceWorkloadsQueries["TableMemoryLimitsPercentByWorkload"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
