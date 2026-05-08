package apiserver

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func APIServersUp(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("API Servers Up",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 80, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["APIServersUp"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func RequestLatency(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Request Latency (99th percentile)",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.SecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["RequestLatencyP99"].Pretty(0),
				query.SeriesNameFormat("{{ verb }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["LatencyThreshold"].Pretty(0),
				query.SeriesNameFormat("Latency Threshold"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func RequestRateByHTTPCode(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Request Rate by HTTP Return Code",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.OpsPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.3,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["RequestRate2xx"].Pretty(0),
				query.SeriesNameFormat("2xx"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["RequestRate3xx"].Pretty(0),
				query.SeriesNameFormat("3xx"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["RequestRate4xx"].Pretty(0),
				query.SeriesNameFormat("4xx"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["RequestRate5xx"].Pretty(0),
				query.SeriesNameFormat("5xx"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func WorkQueueLatency(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Work Queue Latency by Requestor",
		panel.Description("The time it takes to fulfill the different actions to keep the desired status of the cluster."),
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.SecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.3,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.RightPosition,
				Mode:     tsPanel.TableMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["WorkQueueLatency"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} {{ name }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func QueueDepth(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Queue Depth",
		panel.Description("Number of actions waiting in the queue to be performed."),
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.DecimalUnit,
				},
				Min: 0,
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.3,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["QueueDepth"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} {{ name }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func QueueAddRate(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Queue Add Rate",
		panel.Description("How fast we are scheduling new actions to be performed by controller."),
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.OpsPerSecondsUnit,
				},
				Min: 0,
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.3,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["QueueAddRate"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} {{ name }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func Memory(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Memory",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.3,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["Memory"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func CPUUsage(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.DecimalUnit,
				},
				Min: 0,
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.3,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["CPUUsage"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func Goroutines(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Goroutines",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.3,
				LineWidth:   1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["Goroutines"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
