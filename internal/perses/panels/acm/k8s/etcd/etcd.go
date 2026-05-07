package etcd

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func Up(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Up",
		statPanel.Chart(
			statPanel.Calculation(common.MeanCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.DecimalUnit,
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
				Queries["Up"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func RPCRate(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("RPC Rate",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.OpsPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["RPCRate"].Pretty(0),
				query.SeriesNameFormat("RPC Rate"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["RPCFailedRate"].Pretty(0),
				query.SeriesNameFormat("RPC Failed Rate"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ActiveStreams(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Active Streams",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ActiveStreamsWatch"].Pretty(0),
				query.SeriesNameFormat("Watch Streams"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ActiveStreamsLease"].Pretty(0),
				query.SeriesNameFormat("Lease Streams"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func DBSize(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("DB Size",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["DBSize"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} DB Size"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func DiskSyncDuration(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Disk Sync Duration",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.SecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["DiskSyncWAL"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} WAL fsync"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["DiskSyncBackend"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} DB fsync"),
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
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["Memory"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} Resident Memory"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClientTrafficIn(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Client Traffic In",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.8,
				Stack:       tsPanel.AllStack,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ClientTrafficIn"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} Client Traffic In"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClientTrafficOut(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Client Traffic Out",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0.8,
				Stack:       tsPanel.AllStack,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ClientTrafficOut"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} Client Traffic Out"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PeerTrafficIn(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Peer Traffic In",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["PeerTrafficIn"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} Peer Traffic In"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func PeerTrafficOut(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Peer Traffic Out",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.BytesPerSecondsUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["PeerTrafficOut"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} Peer Traffic Out"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func RaftProposals(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Raft Proposals",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ProposalFailureRate"].Pretty(0),
				query.SeriesNameFormat("Proposal Failure Rate"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ProposalPending"].Pretty(0),
				query.SeriesNameFormat("Proposal Pending Total"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ProposalCommitRate"].Pretty(0),
				query.SeriesNameFormat("Proposal Commit Rate"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["ProposalApplyRate"].Pretty(0),
				query.SeriesNameFormat("Proposal Apply Rate"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func LeaderElections(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Total Leader Elections Per Day",
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				LineWidth:   2,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				Queries["LeaderElectionsPerDay"].Pretty(0),
				query.SeriesNameFormat("{{ instance }} Total Leader Elections Per Day"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}