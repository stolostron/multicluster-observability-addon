package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func TotalAlerts(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Alerts",
		panel.Description("Total number of alerts that are firing."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "purple",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\"}) or vector(0)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalCriticalAlerts(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Critical Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: critical."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "red",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\",severity=\"critical\"}) or vector(0)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalWarningAlerts(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Warning Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: warning."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "orange",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\",severity=\"warning\"}) or vector(0)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalModerateAlerts(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Moderate Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: moderate."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "yellow",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\",severity=\"moderate\"}) or vector(0)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalLowAlerts(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Low Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: low."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "green",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\",severity=\"low\"}) or vector(0)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalImportantAlerts(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Important Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: important."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "blue",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\",severity=\"important\"}) or vector(0)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func AlertTypeOverTime(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("AlertType Over Time",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(
				timeSeriesPanel.Legend{
					Mode:     "list",
					Position: "bottom",
				},
			),
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					AreaOpacity:  0.35,
					ConnectNulls: false,
					Display:      "bar",
					LineWidth:    2,
					Stack:        "all",
					Palette:      timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\",severity=~\"$severity\"}) by (alertname)",
					labelMatchers,
				),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func ClusterAffectedOverTime(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Cluster Affected Over Time",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(
				timeSeriesPanel.Legend{
					Mode:     "list",
					Position: "bottom",
				},
			),
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					AreaOpacity:  0.35,
					ConnectNulls: false,
					Display:      "bar",
					LineWidth:    1,
					Stack:        "all",
					Palette:      timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\", cluster!=\"\", severity=~\"$severity\"}) by (cluster)",
					labelMatchers,
				),
				query.SeriesNameFormat("{{ cluster }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func AlertsAndClusters(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Alerts and Clusters",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "value",
					Header: "",
				},
				{
					Name:   "alertname",
					Header: "Alert",
				},
				{
					Name:   "cluster",
					Header: "Cluster",
				},
				{
					Name:   "severity",
					Header: "Severity",
				},
			}),
			tablePanel.WithDensity("compact"),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\", severity=~\"$severity\"}) by (cluster, alertname, severity)",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MostFiringAlerts(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Most Firing Alerts",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					Display: "bar",
					Palette: timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"topk(10, sum(ALERTS{alertstate=\"firing\", severity=~\"$severity\"}) by (alertname))",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MostAffectedClusters(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Most Affected Clusters",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					Display: "bar",
					Palette: timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"topk(10, sum(ALERTS{alertstate=\"firing\", cluster!=\"\", severity=~\"$severity\"}) by (cluster))",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
