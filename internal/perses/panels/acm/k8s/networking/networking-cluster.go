package networking

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	barPanel "github.com/perses/plugins/barchart/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	dl "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/datalinks"
)

// Cluster dashboard panels

func ClusterCurrentBytesReceived(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Rate of Bytes Received",
		barPanel.Chart(
			barPanel.Calculation(common.LastCalculation),
			barPanel.Format(common.Format{
				Unit: &dashboards.BytesPerSecondsUnit,
			}),
			barPanel.SortingBy(barPanel.DescSort),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["ReceiveBandwidthByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterCurrentBytesTransmitted(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Rate of Bytes Transmitted",
		barPanel.Chart(
			barPanel.Calculation(common.LastCalculation),
			barPanel.Format(common.Format{
				Unit: &dashboards.BytesPerSecondsUnit,
			}),
			barPanel.SortingBy(barPanel.DescSort),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TransmitBandwidthByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterCurrentStatus(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Status",
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
					Name:     "namespace",
					Header:   "Namespace",
					Align:    tablePanel.LeftAlign,
					DataLink: dl.NewTableLink("k8s-networking-namespace-pods", "namespace", "Drill down to pods"),
				},
				{
					Name:   "value #1",
					Header: "Current Bandwidth Received",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #2",
					Header: "Current Bandwidth Transmitted",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.BytesPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #3",
					Header: "Rate of Received Packets",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PacketsPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
					Header: "Rate of Transmitted Packets",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PacketsPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #5",
					Header: "Rate of Received Packets Dropped",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PacketsPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #6",
					Header: "Rate of Transmitted Packets Dropped",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PacketsPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
			}),
			tablePanel.WithDensity("compact"),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableReceiveBandwidthByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableTransmitBandwidthByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableReceivedPacketsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableTransmittedPacketsByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableReceivedPacketsDroppedByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TableTransmittedPacketsDroppedByNamespace"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterReceiveBandwidth(datasource string) panelgroup.Option {
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
				ClusterQueries["ReceiveBandwidthByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterTransmitBandwidth(datasource string) panelgroup.Option {
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
				ClusterQueries["TransmitBandwidthByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterReceivedPackets(datasource string) panelgroup.Option {
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
				ClusterQueries["ReceivedPacketsByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterTransmittedPackets(datasource string) panelgroup.Option {
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
				ClusterQueries["TransmittedPacketsByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterReceivedPacketsDropped(datasource string) panelgroup.Option {
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
				ClusterQueries["ReceivedPacketsDroppedByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterTransmittedPacketsDropped(datasource string) panelgroup.Option {
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
				ClusterQueries["TransmittedPacketsDroppedByNamespace"].Pretty(0),
				query.SeriesNameFormat("{{ namespace }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterTCPRetransmits(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Rate of TCP Retransmits out of all sent segments",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TCPRetransmits"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterTCPSynRetransmits(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Rate of TCP SYN Retransmits out of all retransmits",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TCPSynRetransmits"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
