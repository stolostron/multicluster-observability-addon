package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	commonSdk "github.com/perses/perses/go-sdk/common"
	panel "github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	query "github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	"github.com/prometheus/prometheus/model/labels"
)

func CPUOverestimationPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Overestimation",
		panel.Description("Highlights % differences between CPU requests commitments vs utilization. When this difference is large (>20%), it means that resources are reserved but unused."),
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["CPUOverestimation"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPUUsagePanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
				Size:     timeSeriesPanel.SmallSize,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				ConnectNulls: false,
				LineWidth:    0.25,
				AreaOpacity:  1,
				Stack:        timeSeriesPanel.AllStack,
				Palette:      &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["CPUUsageNodeNamespacePod"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPURequestsCommitmentPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Requests Commitment",
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["CPURequestsCommitment"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPUUtilizationPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilization",
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["CPUUtilization"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPUQuotaPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Quota",
		tablePanel.Table(
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
					Name: "test",
					Hide: true,
				},
				{
					Name:   "namespace",
					Header: "Namespace",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "value #1",
					Header: "CPU Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "CPU Usage",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "CPU Requests",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Pods",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["CPURequestsByNamespace"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["CPUUsageNodeNamespacePod"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["CPURequestsByNamespaceSum"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["PodsByNamespace"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryOverestimationPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Overestimation",
		panel.Description("Highlights % differences between memory requests commitments vs utilization. When this difference is large (>20%), it means that resources are reserved but unused."),
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["MemoryOverestimation"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryUsagePanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage (w/o cache)",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
				Size:     timeSeriesPanel.SmallSize,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				ConnectNulls: false,
				LineWidth:    0.25,
				AreaOpacity:  1,
				Stack:        timeSeriesPanel.AllStack,
				Palette:      &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["MemoryUsageNodeNamespacePod"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryRequestsCommitmentPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Requests Commitment",
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["MemoryRequestsCommitment"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryUtilizationPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Utilization",
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["MemoryUtilization"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryRequestsByNamespacePanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Requests by Namespace",
		panel.Description("Shows memory usage, requests, and pod counts per namespace."),
		tablePanel.Table(
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
					Name: "test",
					Hide: true,
				},
				{
					Name:   "namespace",
					Header: "Namespace",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "value #1",
					Header: "Memory Requests %",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentDecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "Memory Usage",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "Memory Requests",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Pods",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["MemoryRequestsByNamespace"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["MemoryUsageNodeNamespacePod"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["MemoryRequestsByNamespaceSum"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["PodsByNamespace"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func NetworkingCurrentStatusPanel(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Current Status",
		panel.Description("Shows network bandwidth metrics including received/transmitted bytes and packet drops."),
		tablePanel.Table(
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
					Name: "test",
					Hide: true,
				},
				{
					Name:   "instance",
					Header: "Instance",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "value #1",
					Header: "Current Bandwidth Received",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "Current Bandwidth Transmitted",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "Rate of Received Packets Dropped",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PacketsPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Rate of Transmitted Packets Dropped",
					Align:  tablePanel.LeftAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PacketsPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["NetworkReceiveByInstance"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["NetworkTransmitByInstance"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["NetworkReceiveDropByInstance"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["NetworkTransmitDropByInstance"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
