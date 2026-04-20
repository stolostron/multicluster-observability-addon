package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/link"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	"github.com/prometheus/prometheus/model/labels"
	dl "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/datalinks"
)

func TotalAlerts(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Alerts",
		panel.Description("Total number of alerts that are firing."),
		panel.AddLink(dl.DashboardURL("acm-clusters-by-alert", dl.StaticParam("alert", "$__all"), dl.StaticParam("severity", "$__all")), link.TargetBlank(true)),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "purple",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["TotalAlerts"],
					labelMatchers,
				).Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalCriticalAlerts(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Critical Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: critical."),
		panel.AddLink(dl.DashboardURL("acm-clusters-by-alert", dl.StaticParam("alert", "$__all"), dl.StaticParam("severity", "critical")), link.TargetBlank(true)),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "red",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["TotalCriticalAlerts"],
					labelMatchers,
				).Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalWarningAlerts(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Warning Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: warning."),
		panel.AddLink(dl.DashboardURL("acm-clusters-by-alert", dl.StaticParam("alert", "$__all"), dl.StaticParam("severity", "warning")), link.TargetBlank(true)),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "orange",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["TotalWarningAlerts"],
					labelMatchers,
				).Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalModerateAlerts(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Moderate Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: moderate."),
		panel.AddLink(dl.DashboardURL("acm-clusters-by-alert", dl.StaticParam("alert", "$__all"), dl.StaticParam("severity", "moderate")), link.TargetBlank(true)),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "yellow",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["TotalModerateAlerts"],
					labelMatchers,
				).Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalLowAlerts(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Low Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: low."),
		panel.AddLink(dl.DashboardURL("acm-clusters-by-alert", dl.StaticParam("alert", "$__all"), dl.StaticParam("severity", "low")), link.TargetBlank(true)),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "green",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["TotalLowAlerts"],
					labelMatchers,
				).Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func TotalImportantAlerts(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Total Important Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: important."),
		panel.AddLink(dl.DashboardURL("acm-clusters-by-alert", dl.StaticParam("alert", "$__all"), dl.StaticParam("severity", "important")), link.TargetBlank(true)),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "blue",
				Mode:         commonSdk.AbsoluteMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["TotalImportantAlerts"],
					labelMatchers,
				).Pretty(0)+" or vector(0)",
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func AlertTypeOverTime(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
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
					Palette:      &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["AlertTypeOverTime"],
					labelMatchers,
				).Pretty(0),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func ClusterAffectedOverTime(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
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
					Palette:      &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["ClusterAffectedOverTime"],
					labelMatchers,
				).Pretty(0),
				query.SeriesNameFormat("{{ cluster }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func AlertsAndClusters(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Alerts and Clusters",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name:   "value",
					Header: "",
				},
				{
					Name:     "alertname",
					Header:   "Alert",
					DataLink: dl.NewTableLinkCustomVar("acm-clusters-by-alert", "alert", "alertname", "Drill down to Clusters with this Alert"),
				},
				{
					Name:     "cluster",
					Header:   "Cluster",
					DataLink: dl.NewTableLinkNewTab("acm-alerts-by-cluster", "cluster", "Drill down to Alerts on this Cluster"),
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
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["AlertsAndClusters"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MostFiringAlerts(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Most Firing Alerts",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					Display: "bar",
					Palette: &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top10AlertsFiringByName"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MostAffectedClusters(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Most Affected Clusters",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					Display: "bar",
					Palette: &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["Top10AlertsFiringByCluster"],
					labelMatchers,
				).Pretty(0),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
