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

var hcpStatThresholds = commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#73bf69"},
		{Value: 80, Color: "#f2495c"},
	},
}

func HCPPodCount(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Number of pods",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(hcpStatThresholds),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPPodCount"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPCPUUsageGraph(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU usage",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				AreaOpacity: 0.8,
				Display:     timeSeriesPanel.LineDisplay,
				LineWidth:   0.25,
				Stack:       "all",
			}),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Mode:     "list",
				Position: timeSeriesPanel.BottomPosition,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPCPUUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{pod}}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPCPURequestsPercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Requests %",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(hcpStatThresholds),
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
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(hcpStatThresholds),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPCPURequests"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPCPUUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(hcpStatThresholds),
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
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(hcpStatThresholds),
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
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				AreaOpacity: 0.8,
				Display:     timeSeriesPanel.LineDisplay,
				LineWidth:   0.25,
				Stack:       "all",
			}),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Mode:     "list",
				Position: timeSeriesPanel.BottomPosition,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPMemoryUsageByPod"].Pretty(0),
				query.SeriesNameFormat("{{pod}}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

var decBytesUnit = string(commonSdk.DecimalBytesUnit)

func HCPMemoryRequests(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Requests",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.BytesUnit,
			}),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(hcpStatThresholds),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPMemoryRequests"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func HCPMemoryUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &decBytesUnit,
			}),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(hcpStatThresholds),
		),
		panel.AddQuery(
			query.PromQL(
				HCPPanelQueries["HCPMemoryUsage"].Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
