package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func AlertSeverity(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Alert Severity",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\", cluster=\"$cluster\"}) by (severity)",
					labelMatchers,
				),
				query.SeriesNameFormat("{{ severity }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func FiringAlertsTrend(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
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
					Palette:      timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\", cluster=\"$cluster\", severity=~\"$severity\"}) by (alertname)",
					labelMatchers,
				),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func PendingAlertsTrend(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
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
					Palette:      timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"pending\", cluster=\"$cluster\", severity=~\"$severity\"}) by (alertname)",
					labelMatchers,
				),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func AlertsOverTime(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
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
					Palette:      timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
				},
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\", cluster=\"$cluster\", severity=~\"$severity\" }) by (alertname)",
					labelMatchers,
				),
				query.SeriesNameFormat("{{ alertname }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
