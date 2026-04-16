package networking

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	gaugePanel "github.com/perses/plugins/gaugechart/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// Namespace (Pods) dashboard panels

func NamespacePodsCurrentBytesReceived(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["TotalReceiveBandwidth"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsCurrentBytesTransmitted(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["TotalTransmitBandwidth"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsCurrentStatus(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Current Status",
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
					DataLink: &tablePanel.DataLink{
						URL:   "/monitoring/v2/dashboards/view?dashboard=k8s-networking-pod&project=$__project&var-pod=${__data.fields[\"pod\"]}",
						Title: "Drill down",
					},
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
				NamespacePodsQueries["ReceiveBandwidth"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TransmitBandwidth"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["ReceivedPackets"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TransmittedPackets"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["ReceivedPacketsDropped"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				NamespacePodsQueries["TransmittedPacketsDropped"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsReceiveBandwidth(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["ReceiveBandwidthTS"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsTransmitBandwidth(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["TransmitBandwidthTS"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsReceivedPackets(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["ReceivedPacketsTS"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsTransmittedPackets(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["TransmittedPacketsTS"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsReceivedPacketsDropped(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["ReceivedPacketsDroppedTS"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func NamespacePodsTransmittedPacketsDropped(datasource string) panelgroup.Option {
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
				NamespacePodsQueries["TransmittedPacketsDroppedTS"].Pretty(0),
				query.SeriesNameFormat("{{ pod }}"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}
