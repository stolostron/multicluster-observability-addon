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

// Namespace (Pods) dashboard panels

func NamespacePodsCPUUtilisationFromRequests(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilization (from requests)",
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
				NamespacePodsQueries["CPUUtilisationFromRequests"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsCPUUtilisationFromLimits(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilization (from limits)",
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
				NamespacePodsQueries["CPUUtilisationFromLimits"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsMemoryUtilisationFromRequests(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Utilization (from requests)",
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
				NamespacePodsQueries["MemoryUtilisationFromRequests"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsMemoryUtilisationFromLimits(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Utilization (from limits)",
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
				NamespacePodsQueries["MemoryUtilisationFromLimits"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsCPUUsage(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["CPUUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["CPUQuotaRequests"].Pretty(0),
				query.SeriesNameFormat("quota - requests"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["CPUQuotaLimits"].Pretty(0),
				query.SeriesNameFormat("quota - limits"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsCPUQuota(datasource string) panelgroup.Option {
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
				},
				{
					Name:   "value #1",
					Header: "CPU Usage",
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "CPU Requests",
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "CPU Requests %",
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "CPU Limits",
					Format: &common.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
					Header: "CPU Limits %",
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
				NamespacePodsQueries["TableCPUUsageByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableCPURequestsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableCPURequestsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableCPULimitsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableCPULimitsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsMemoryUsage(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["MemoryUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["MemoryQuotaRequests"].Pretty(0),
				query.SeriesNameFormat("quota - requests"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["MemoryQuotaLimits"].Pretty(0),
				query.SeriesNameFormat("quota - limits"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsMemoryQuota(datasource string) panelgroup.Option {
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
				},
				{
					Name:   "value #1",
					Header: "Memory Usage",
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "Memory Requests",
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "Memory Requests %",
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Memory Limits",
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
					Header: "Memory Limits %",
					Format: &common.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #6",
					Header: "Memory Usage (RSS)",
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #7",
					Header: "Memory Usage (Cache)",
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #8",
					Header: "Memory Usage (Swap)",
					Format: &common.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
			}),
			tablePanel.WithDensity("compact"),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemoryUsageByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemoryRequestsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemoryRequestsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemoryLimitsByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemoryLimitsPercentByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemoryRSSByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemoryCacheByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TableMemorySwapByPod"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
