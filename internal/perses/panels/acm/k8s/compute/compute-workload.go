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

// Workload dashboard panels

func WorkloadCPUUsage(datasource string) panelgroup.Option {
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
				WorkloadQueries["CPUUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func WorkloadCPUQuota(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Quota",
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "pod",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"pod"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:   "pod",
					Header: "Pod",
					Align:  tablePanel.LeftAlign,
					DataLink: dl.NewTableLinkNewTab("k8s-compute-resources-pod", "pod", "Drill down to pod"),
				},
				{
					Name:   "value #1",
					Header: "CPU Usage",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "CPU Requests",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "CPU Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "CPU Limits",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
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
				WorkloadQueries["TableCPUUsageByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableCPURequestsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableCPURequestsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableCPULimitsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableCPULimitsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func WorkloadMemoryUsage(datasource string) panelgroup.Option {
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
				WorkloadQueries["MemoryUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func WorkloadMemoryQuota(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Quota",
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "pod",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"pod"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:   "pod",
					Header: "Pod",
					Align:  tablePanel.LeftAlign,
					DataLink: dl.NewTableLinkNewTab("k8s-compute-resources-pod", "pod", "Drill down to pod"),
				},
				{
					Name:   "value #1",
					Header: "Memory Usage",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "Memory Requests",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "Memory Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Memory Limits",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
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
				WorkloadQueries["TableMemoryUsageByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableMemoryRequestsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableMemoryRequestsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableMemoryLimitsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				WorkloadQueries["TableMemoryLimitsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
