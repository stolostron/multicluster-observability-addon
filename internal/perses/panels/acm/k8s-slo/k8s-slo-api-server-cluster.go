package slo

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	gaugePanel "github.com/perses/plugins/gaugechart/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	tsPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// Service-Level Overview group panels

func ClusterTarget(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Target",
		panel.Description("The service-level target for the API server request duration service-level objective (SLO)."),
		statPanel.Chart(
			statPanel.Calculation(common.LastNumberCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["SLIBinTrend"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterPast7Days(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Past 7 Days",
		panel.Description("Service-level objective (SLO) status from over a 7 days period. (The SLO is calculated from # of request duration >= target / total count of request durations)"),
		statPanel.Chart(
			statPanel.Calculation(common.LastNumberCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "red"},
					{Value: 95, Color: "#EAB839"},
					{Value: 99, Color: "green"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["SLO7d"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterPast30Days(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Past 30 Days",
		panel.Description("Service-level objective (SLO) status from over a 30 days period. (The SLO is calculated from # of request duration >= target / total count of request durations)"),
		statPanel.Chart(
			statPanel.Calculation(common.LastNumberCalculation),
			statPanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "red"},
					{Value: 95, Color: "#EAB839"},
					{Value: 99, Color: "green"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["SLO30d"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

// Error Budget for 7 Days group panels

func ClusterDayOfWeek(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Day of the week",
		panel.Description("The current day within the week period."),
		gaugePanel.Chart(
			gaugePanel.Calculation(common.LastNumberCalculation),
			gaugePanel.Max(7),
			gaugePanel.Format(common.Format{
				Unit: &dashboards.DecimalUnit,
			}),
			gaugePanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "orange"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["DayOfWeek"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterErrorBudget7d(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Error Budget (Past 7 Days)",
		panel.Description("The amount of error budget that has been consumed for the API server request duration service-level objective (SLO)."),
		gaugePanel.Chart(
			gaugePanel.Calculation(common.LastNumberCalculation),
			gaugePanel.Max(1),
			gaugePanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			gaugePanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 50, Color: "#EAB839"},
					{Value: 80, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["ErrorBudget7d"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterDowntimeRemaining7d(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Downtime Remaining (Past 7-days)",
		panel.Description("The time remaining within the 7d period in which the cluster can afford downtime."),
		statPanel.Chart(
			statPanel.Calculation(common.LastNumberCalculation),
			statPanel.Format(common.Format{
				Unit: &minutesUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["DowntimeRemaining7d"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

// Error Budget for 30 Days group panels

func ClusterDayOfMonth(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Day of the month",
		panel.Description("The current day within the month period."),
		gaugePanel.Chart(
			gaugePanel.Calculation(common.LastNumberCalculation),
			gaugePanel.Max(31),
			gaugePanel.Format(common.Format{
				Unit: &dashboards.DecimalUnit,
			}),
			gaugePanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "orange"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["DayOfMonth"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterErrorBudget30d(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Error Budget (Past 30 Days)",
		panel.Description("The amount of error budget that has been consumed for the API server request duration service-level objective (SLO)."),
		gaugePanel.Chart(
			gaugePanel.Calculation(common.LastNumberCalculation),
			gaugePanel.Max(1),
			gaugePanel.Format(common.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
			gaugePanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "green"},
					{Value: 50, Color: "#EAB839"},
					{Value: 80, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["ErrorBudget30d"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterDowntimeRemaining30d(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("Downtime Remaining (Past 30-days)",
		panel.Description("The time remaining within the 30d period in which the cluster can afford downtime."),
		statPanel.Chart(
			statPanel.Calculation(common.LastNumberCalculation),
			statPanel.Format(common.Format{
				Unit: &minutesUnit,
			}),
			statPanel.Thresholds(common.Thresholds{
				Steps: []common.StepOption{
					{Value: 0, Color: "red"},
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["DowntimeRemaining30d"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

// Trend group panels

func ClusterSLITrend(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("API Server Request Duration - SLI",
		panel.Description("Trending graph of the service-level indicators (SLI) over relative time period used to compute the service-level objective (SLO)."),
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
				ClusterQueries["SLITrend"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
		panel.AddQuery(
			query.PromQL(
				ClusterQueries["TargetThreshold"].Pretty(0),
				query.SeriesNameFormat("Target Threshold"),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}

func ClusterSLITable(datasource string) panelgroup.Option {
	return panelgroup.AddPanel("API Server Request Duration - SLI ",
		panel.Description("The collected service-level indicator (SLI) values for the API server request duration service-level objective (SLO), over the relative time range."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "timestamp",
					Header: "Time",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "cluster",
					Header: "Cluster",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "clusterID",
					Header: "ClusterID",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "value",
					Header: "SLI",
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
				ClusterQueries["SLITrend"].Pretty(0),
				dashboards.AddQueryDataSource(datasource),
			),
		),
	)
}