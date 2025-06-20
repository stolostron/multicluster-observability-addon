package acm

import (
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"

	"github.com/perses/community-dashboards/pkg/dashboards"
	"github.com/perses/community-dashboards/pkg/promql"
)

func ActiveIncidents(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Active Incidents",
		panel.Description("Shows active incidents for the selected cluster"),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
					Width:  152,
				},
				{
					Name:   "severity",
					Header: "Severity",
					Width:  92,
				},
				{
					Name:   "components",
					Header: "Components",
				},
				{
					Name:   "alerts",
					Header: "Alerts",
					Width:  438,
				},
				{
					Name:   "start_time",
					Header: "Start Time",
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"max(cluster:health:components:map{cluster=\"$cluster\"}>0) by (group_id,cluster,component,src_alertname) * on (cluster) group_left(url) console_url",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"min_over_time(timestamp(max by (group_id) (cluster:health:components:map{cluster=\"$cluster\"}))[$__interval:1m]) * 1000",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func IncidentCount(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Number of incidents",
		panel.Description("Shows the number of incidents over time"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit:          string(commonSdk.DecimalUnit),
					DecimalPlaces: 0,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
				Size:     timeSeriesPanel.SmallSize,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				ConnectNulls: false,
				LineWidth:    0.25,
				AreaOpacity:  1,
				Stack:        timeSeriesPanel.AllStack,
				Palette:      timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"count(sum(count_over_time((cluster:health:components:map{cluster=\"$cluster\",src_severity!=\"none\"}>0)[$__interval:1m])) by (group_id))",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
