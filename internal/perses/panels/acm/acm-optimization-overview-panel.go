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
)

func CPUOverestimationPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Overestimation",
		panel.Description("Highlights % differences between CPU requests commitments vs utilization. When this difference is large (>20%), it means that resources are reserved but unused."),
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"(sum(cluster:kube_pod_container_resource_requests:cpu:sum{cluster=\"$cluster\"}) / sum(kube_node_status_allocatable{cluster=\"$cluster\", resource=\"cpu\"})) - (1 - node_cpu_seconds_total:mode_idle:avg_rate5m)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPUUsagePanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage",
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
				promql.SetLabelMatchers(
					"node_namespace_pod_container:container_cpu_usage_seconds_total:sum{cluster=\"$cluster\"}",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPURequestsCommitmentPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Requests Commitment",
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(cluster:kube_pod_container_resource_requests:cpu:sum{cluster=\"$cluster\"}) / sum(kube_node_status_allocatable{cluster=\"$cluster\", resource=\"cpu\"})",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPUUtilizationPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilization",
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"1 - node_cpu_seconds_total:mode_idle:avg_rate5m",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func CPUQuotaPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("CPU Quota",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
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
				promql.SetLabelMatchers(
					"sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum{cluster=\"$cluster\"}) by (namespace) / sum(kube_pod_container_resource_requests{cluster=\"$cluster\", resource=\"cpu\"}) by (namespace)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"node_namespace_pod_container:container_cpu_usage_seconds_total:sum{cluster=\"$cluster\"}",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(kube_pod_container_resource_requests{cluster=\"$cluster\", resource=\"cpu\"}) by (namespace)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(kube_pod_info{cluster=\"$cluster\"}) by (namespace)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryOverestimationPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Overestimation",
		panel.Description("Highlights % differences between memory requests commitments vs utilization. When this difference is large (>20%), it means that resources are reserved but unused."),
		statPanel.Chart(
			statPanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentUnit,
				DecimalPlaces: 2,
			}),
			statPanel.ValueFontSize(50),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"(sum(cluster:kube_pod_container_resource_requests:memory:sum{cluster=\"$cluster\"}) / sum(kube_node_status_allocatable{cluster=\"$cluster\", resource=\"memory\"})) - (1 - (node_memory_MemAvailable_bytes{cluster=\"$cluster\"} / node_memory_MemTotal_bytes{cluster=\"$cluster\"}))",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryUsagePanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage",
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
				promql.SetLabelMatchers(
					"node_namespace_pod_container:container_memory_working_set_bytes:sum{cluster=\"$cluster\"}",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryRequestsCommitmentPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
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
				promql.SetLabelMatchers(
					"sum(cluster:kube_pod_container_resource_requests:memory:sum{cluster=\"$cluster\"}) / sum(kube_node_status_allocatable{cluster=\"$cluster\", resource=\"memory\"})",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryUtilizationPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
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
				promql.SetLabelMatchers(
					"1 - (node_memory_MemAvailable_bytes{cluster=\"$cluster\"} / node_memory_MemTotal_bytes{cluster=\"$cluster\"})",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemoryRequestsByNamespacePanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Memory Requests by Namespace",
		panel.Description("Shows memory usage, requests, and pod counts per namespace."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
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
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "Memory Requests",
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
				promql.SetLabelMatchers(
					"sum(node_namespace_pod_container:container_memory_working_set_bytes:sum{cluster=\"$cluster\"}) by (namespace) / sum(kube_pod_container_resource_requests{cluster=\"$cluster\", resource=\"memory\"}) by (namespace)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"node_namespace_pod_container:container_memory_working_set_bytes:sum{cluster=\"$cluster\"}",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(kube_pod_container_resource_requests{cluster=\"$cluster\", resource=\"memory\"}) by (namespace)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(kube_pod_info{cluster=\"$cluster\"}) by (namespace)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func NetworkingCurrentStatusPanel(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Current Status",
		panel.Description("Shows network bandwidth metrics including received/transmitted bytes and packet drops."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
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
						Unit:          &dashboards.BytesUnit,
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
				promql.SetLabelMatchers(
					"sum(instance:node_network_receive_bytes_excluding_lo:rate1m{cluster=\"$cluster\"}) by (instance)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(instance:node_network_transmit_bytes_excluding_lo:rate1m{cluster=\"$cluster\"}) by (instance)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(instance:node_network_receive_drop_excluding_lo:rate1m{cluster=\"$cluster\"}) by (instance)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(instance:node_network_transmit_drop_excluding_lo:rate1m{cluster=\"$cluster\"}) by (instance)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
