package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
	"github.com/prometheus/prometheus/model/labels"
)

func AlertSeverity(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Alert Severity",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["AlertsFiringSeverity"],
					labelMatchers,
				).Pretty(0),
				query.SeriesNameFormat("{{ severity }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func FiringAlertsTrend(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Firing Alerts Trend",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(
				timeSeriesPanel.Legend{
					Mode:     "list",
					Position: "bottom",
				},
			),
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					AreaOpacity:  0,
					ConnectNulls: false,
					Display:      "line",
					LineWidth:    1,
					Palette:      &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["AlertsFiringByName"],
					labelMatchers,
				).Pretty(0),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func PendingAlertsTrend(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Pending Alerts Trend",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(
				timeSeriesPanel.Legend{
					Mode:     "list",
					Position: "bottom",
				},
			),
			timeSeriesPanel.WithVisual(
				timeSeriesPanel.Visual{
					AreaOpacity:  0,
					ConnectNulls: false,
					Display:      "line",
					LineWidth:    1,
					Palette:      &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchersV2(
					ACMCommonPanelQueries["AlertsPendingByName"],
					labelMatchers,
				).Pretty(0),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func AlertsOverTime(datasourceName string, labelMatchers ...*labels.Matcher) panelgroup.Option {
	return panelgroup.AddPanel("Alerts over time",
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
					ACMCommonPanelQueries["AlertsFiringByName"],
					labelMatchers,
				).Pretty(0),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
