package slo

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	dl "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/datalinks"
)

func FleetClustersExceededSLO(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Number of cluster that has exceeded the SLO",
		panel.Description("A total number of the clusters that have exceeded their service-level objective (SLO) target."),
		statPanel.Chart(
			statPanel.Calculation(common.LastNumberCalculation),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 1, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				FleetQueries["ClustersExceededSLO"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func FleetClustersMeetingSLO(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Number of clusters that are meeting the SLO",
		panel.Description("A total number of clusters that haven't breached the service-level objective (SLO) target."),
		statPanel.Chart(
			statPanel.Calculation(common.LastNumberCalculation),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "red"},
					{Value: 1, Color: "green"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				FleetQueries["ClustersMeetingSLO"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func FleetTopClusters(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Top Clusters",
		panel.Description("List of the topk cluster over a $window period. The results are sorted from worst offending clusters to passing clusters."),
		tablePanel.Table(
			tablePanel.Transform([]common.Transform{
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "cluster",
					},
				},
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "clusterID",
					},
				},
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "prometheus",
					},
				},
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "receive",
					},
				},
				{
					Kind: common.MergeIndexedColumnsKind,
					Spec: common.MergeIndexedColumnsSpec{
						Column: "tenant_id",
					},
				},
				{
					Kind: common.JoinByColumValueKind,
					Spec: common.JoinByColumnValueSpec{
						Columns: []string{"cluster"},
					},
				},
			}),
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name: "clusterID",
					Hide: true,
				},
				{
					Name: "prometheus",
					Hide: true,
				},
				{
					Name: "receive",
					Hide: true,
				},
				{
					Name: "tenant_id",
					Hide: true,
				},
				{
					Name:     "cluster",
					Header:   "Cluster",
					Align:    tablePanel.LeftAlign,
					DataLink: dl.NewTableLinkNewTab("k8s-slo-api-server-cluster", "cluster", "Kubernetes / Service-Level Overview / API Server / Cluster"),
				},
				{
					Name:   "value #1",
					Header: "SLO",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit: &dashboards.PercentDecimalUnit,
					},
				},
				{
					Name:   "value #2",
					Header: "Error Budget",
					Align:  tablePanel.LeftAlign,
					Format: &common.Format{
						Unit: &dashboards.PercentDecimalUnit,
					},
				},
			}),
			tablePanel.WithDensity("compact"),
		),
		panel.AddQuery(
			query.PromQL(
				FleetQueries["TopClustersSLO"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				FleetQueries["TopClustersErrorBudget"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func FleetSLITrend(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Top Cluster's SLI Trend",
		panel.Description("The service-level indicator (SLI) trend of the topk clusters over a relative time. The results are sorted from worst offending clusters to passing clusters."),
		tsPanel.Chart(
			tsPanel.WithYAxis(tsPanel.YAxis{
				Show: true,
				Format: &common.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
				Min: 0.8,
				Max: 1,
			}),
			tsPanel.WithVisual(tsPanel.Visual{
				AreaOpacity: 0,
				PointRadius: 1,
				ShowPoints:  tsPanel.AlwaysShowPoints,
			}),
			tsPanel.WithLegend(tsPanel.Legend{
				Position: tsPanel.BottomPosition,
				Mode:     tsPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				FleetQueries["TopClustersSLITrend"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				FleetQueries["TargetThreshold"].Pretty(0),
				query.SeriesNameFormat("Target Threshold"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

var minutesUnit = string(common.MinutesUnit)
