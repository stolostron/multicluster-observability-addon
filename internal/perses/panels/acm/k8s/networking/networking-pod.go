package networking

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	gaugePanel "github.com/perses/plugins/gaugechart/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// Pod dashboard panels

func PodCurrentBytesReceived(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Rate of Bytes Received",
		gaugePanel.Chart(
			gaugePanel.Calculation(common.LastNumberCalculation),
			gaugePanel.Max(10000000000),
			gaugePanel.Format(common.Format{
				Unit: &dashboards.BytesPerSecondsUnit,
			}),
			gaugePanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 5000000000, Color: "#EAB839"},
					{Value: 7000000000, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["ReceiveBandwidth"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PodCurrentBytesTransmitted(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Rate of Bytes Transmitted",
		gaugePanel.Chart(
			gaugePanel.Calculation(common.LastNumberCalculation),
			gaugePanel.Max(10000000000),
			gaugePanel.Format(common.Format{
				Unit: &dashboards.BytesPerSecondsUnit,
			}),
			gaugePanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 5000000000, Color: "#EAB839"},
					{Value: 7000000000, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["TransmitBandwidth"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PodReceiveBandwidth(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Receive Bandwidth",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["ReceiveBandwidth"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PodTransmitBandwidth(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Transmit Bandwidth",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["TransmitBandwidth"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PodReceivedPackets(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Rate of Received Packets",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.PacketsPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["ReceivedPackets"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PodTransmittedPackets(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Rate of Transmitted Packets",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.PacketsPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["TransmittedPackets"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PodReceivedPacketsDropped(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Rate of Received Packets Dropped",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.PacketsPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["ReceivedPacketsDropped"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PodTransmittedPacketsDropped(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Rate of Transmitted Packets Dropped",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.PacketsPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
				Stack:       tsPanel.AllStack,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				PodQueries["TransmittedPacketsDropped"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
