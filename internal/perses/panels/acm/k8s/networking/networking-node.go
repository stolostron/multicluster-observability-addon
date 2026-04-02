package networking

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// Node dashboard panels

func NodeCurrentBytesReceived(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Rate of Bytes Received",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				Display:     tsPanel.BarDisplay,
				LineWidth:   1,
				AreaOpacity: 1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.RightPosition,
				Mode:     tsPanel.TableMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				NodeQueries["ReceiveBandwidthByInstance"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NodeCurrentBytesTransmitted(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Rate of Bytes Transmitted",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				Display:     tsPanel.BarDisplay,
				LineWidth:   1,
				AreaOpacity: 1,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.RightPosition,
				Mode:     tsPanel.TableMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				NodeQueries["TransmitBandwidthByInstance"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NodeCurrentStatus(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Status",
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "instance",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"instance"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
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
					Header: "Rate of Received Packets Dropped",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit:          &dashboards.PacketsPerSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "value #4",
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
				NodeQueries["ReceiveBandwidthByInstance"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NodeQueries["TransmitBandwidthByInstance"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NodeQueries["ReceivedDropByInstance"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NodeQueries["TransmittedDropByInstance"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NodeReceivedPacketsDropped(datasource string) panelgroup.Option {
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
				NodeQueries["ReceivedDropByInstance"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NodeTransmittedPacketsDropped(datasource string) panelgroup.Option {
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
				NodeQueries["TransmittedDropByInstance"].Pretty(0),
				query.SeriesNameFormat("{{ instance }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NodeTCPRetransmits(datasource string) panelgroup.Option {
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

func NodeTCPSynRetransmits(datasource string) panelgroup.Option {
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
