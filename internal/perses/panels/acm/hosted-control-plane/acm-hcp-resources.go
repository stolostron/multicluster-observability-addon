package hosted_control_plane

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func HCPPodCount(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Number of pods",
		panel.Description("Total number of pods in the HCP namespace."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "blue",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPPodCount"].Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPCPUUsageGraph(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU usage",
		panel.Description("CPU usage by pod in the HCP namespace."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				AreaOpacity: 0.5,
				Display:     timeSeriesPanel.LineDisplay,
				Stack:       "all",
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPCPUUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPCPURequestsPercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Requests %",
		panel.Description("CPU usage as a percentage of CPU requests in the HCP namespace."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPCPURequestsPercent"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPCPURequests(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Requests",
		panel.Description("Total CPU requests in the HCP namespace."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "blue",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPCPURequests"].Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPCPUUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage",
		panel.Description("Current CPU usage in the HCP namespace."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "blue",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPCPUUsage"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPMemoryRequestsPercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Requests %",
		panel.Description("Memory usage as a percentage of memory requests in the HCP namespace."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPMemoryRequestsPercent"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPMemoryUsageGraph(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage (w/o cache)",
		panel.Description("Memory RSS usage by pod in the HCP namespace, excluding cache."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				AreaOpacity: 0.5,
				Display:     timeSeriesPanel.LineDisplay,
				Stack:       "all",
			}),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPMemoryUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPMemoryRequests(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Requests",
		panel.Description("Total memory requests in the HCP namespace."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.BytesUnit,
			}),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "blue",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPMemoryRequests"].Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPMemoryUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage",
		panel.Description("Current memory RSS usage in the HCP namespace."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.BytesUnit,
			}),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "blue",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPMemoryUsage"].Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
