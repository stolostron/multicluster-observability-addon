package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	"github.com/prometheus/prometheus/model/labels"
)

func Top50MaxLatencyAPIServer(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 50 Max Latency API Server",
		panel.Description("Shows the top 50 clusters with highest API server latency, their API server status, and error rates."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "value #1",
					Header: "Max Latency (99th percentile)",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.MilliSecondsUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "api_up",
					Header: "API Server UP",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.PercentDecimalUnit,
					},
				},
				{
					Name:   "value #2",
					Header: "API Error[1h]",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top50MaxLatencyAPIServer_MaxLatency"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["APIServerRequestTotal_ErrorRate"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func EtcdHealth(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("etcd Health",
		panel.Description("Shows etcd health metrics including leader status, leader changes, and database size."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "has_leader",
					Header: "Has a leader",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "value",
					Header: "Leader election change",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "db_size",
					Header: "DB size",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["EtcdHealth_LeaderChanges"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func Top50CPUOverEstimationClusters(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 50 CPU Overestimation Clusters",
		panel.Description("Highlights % differences between CPU requests commitments vs utilization. When this difference is large (>20%), it means that resources are reserved but unused."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "Value",
					Header: "Overestimation",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "cpu_requested",
					Header: "Requested",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "cpu_utilized",
					Header: "Utilized",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top50CPUOverestimation"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func Top50MemoryOverEstimationClusters(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 50 Memory Overestimation Clusters",
		panel.Description("Highlights % differences between Memory requests commitments vs utilization. When this difference is large (>20%), it means that resources are reserved but unused."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "Value",
					Header: "Overestimation",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "memory_requested",
					Header: "Requested",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "memory_utilized",
					Header: "Utilized",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "clusterID",
					Header: "Cluster ID",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "prometheus",
					Header: "Prometheus",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "receive",
					Header: "Receive",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "tenant_id",
					Header: "Tenant ID",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top50MemoryOverestimation"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func Top50CPUUtilizedClusters(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 50 CPU Utilized Clusters",
		panel.Description("Shows CPU utilization metrics including total cores, allocatable cores, and utilization percentage."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "machine_cpu_cores_sum",
					Header: "Total Cores",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "node_allocatable_cpu_cores_sum",
					Header: "Allocatable Cores",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "cpu_requested",
					Header: "Requested",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.BytesUnit,
					},
				},
				{
					Name:   "Value",
					Header: "Utilized",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "clusterID",
					Header: "Cluster ID",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "prometheus",
					Header: "Prometheus",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "receive",
					Header: "Receive",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "tenant_id",
					Header: "Tenant ID",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top50CPUUtilized"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func Top5CPUUtilizationGraph(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 5 Utilized Clusters (% CPU usage)",
		panel.Description("Shows CPU utilization trends for the top 5 clusters."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "Value",
					Header: "CPU Usage %",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top5CPUUtilized"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func Top50MemoryUtilizedClusters(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 50 Memory Utilized Clusters",
		panel.Description("Shows memory utilization metrics including available memory, requested memory, and utilization percentage."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "machine_memory_sum",
					Header: "Available Memory",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "machine_memory_requested",
					Header: "Requested",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "Value",
					Header: "Utilized",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "clusterID",
					Header: "Cluster ID",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "prometheus",
					Header: "Prometheus",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "receive",
					Header: "Receive",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit: &dashboards.DecimalUnit,
					},
				},
				{
					Name:   "tenant_id",
					Header: "Tenant ID",
					Align:  tablePanel.RightAlign,
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top50MemoryUtilized"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func Top5MemoryUtilizationGraph(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 5 Utilized Clusters (% Memory usage)",
		panel.Description("Shows memory utilization trends for the top 5 clusters."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "Value",
					Header: "Memory Usage %",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.PercentUnit,
						DecimalPlaces: 2,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top5MemoryUtilized"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func BandwidthUtilization(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Bandwidth Utilization",
		panel.Description("Shows network bandwidth metrics including received/transmitted bytes and packet drops."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.RightAlign,
				},
				{
					Name:   "Value",
					Header: "Current Bandwidth Received",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "node_transmit",
					Header: "Current Bandwidth Transmitted",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.BytesUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "node_receive_drop",
					Header: "Rate of Received Packets Dropped",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
				{
					Name:   "node_transmit_drop",
					Header: "Rate of Transmitted Packets Dropped",
					Align:  tablePanel.RightAlign,
					Format: &commonSdk.Format{
						Unit:          &dashboards.DecimalUnit,
						DecimalPlaces: 2,
					},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["BandwidthUtilization_ReceiveRate"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["BandwidthUtilization_TransmitRate"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["BandwidthUtilization_ReceiveDropRate"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["BandwidthUtilization_TransmitDropRate"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
