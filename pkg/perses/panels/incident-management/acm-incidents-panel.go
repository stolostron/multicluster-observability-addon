package incident_management

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func ActiveIncidents(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Active Incidents",
		panel.Description("Shows active incidents for the selected cluster"),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"max(cluster_health_components_map{cluster=\"$cluster\",src_alertname!~\"Watchdog\"}>0) by (group_id,cluster,component,src_alertname) * on (cluster) group_left(url) console_url",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"min_over_time(timestamp(max by (group_id) (cluster_health_components_map{cluster=\"$cluster\",src_alertname!~\"Watchdog\"}))[$__range:1m]) * 1000",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "cluster",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "value #1",
					Header: "severity",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "component",
					Header: "components",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name:   "src_alertname",
					Header: "alerts",
					Align:  tablePanel.LeftAlign,
				},
				{
					Name: "timestamp",
					Hide: true,
				},
				{
					Name: "url",
					Hide: true,
				},
				{
					Name: "group_id",
					Hide: true,
				},
				{
					Name:   "value #2",
					Header: "start time",
					Align:  tablePanel.LeftAlign,
				},
			}),
			tablePanel.WithCellSettings(
				[]tablePanel.CellSettings{
					{
						Condition: tablePanel.Condition{
							Kind: tablePanel.ValueConditionKind,
							Spec: &tablePanel.ValueConditionSpec{
								Value: "0",
							},
						},
						Text: "info",
					},
					{
						Condition: tablePanel.Condition{
							Kind: tablePanel.ValueConditionKind,
							Spec: &tablePanel.ValueConditionSpec{
								Value: "1",
							},
						},
						Text: "warning",
					},
					{
						Condition: tablePanel.Condition{
							Kind: tablePanel.ValueConditionKind,
							Spec: &tablePanel.ValueConditionSpec{
								Value: "2",
							},
						},
						Text: "critical",
					},
				},
			),
			tablePanel.Transform([]commonSdk.Transform{
				{
					Kind: commonSdk.MergeSeriesKind,
					Spec: commonSdk.MergeSeriesSpec{},
				},
				{
					Kind: commonSdk.JoinByColumValueKind,
					Spec: commonSdk.JoinByColumnValueSpec{
						Columns: []string{"group_id"},
					},
				},
			}),
		),
	)
}

func IncidentCount(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Number of incidents",
		panel.Description("Shows the number of incidents over time"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
				Size:     timeSeriesPanel.SmallSize,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.BarDisplay,
				ConnectNulls: false,
				LineWidth:    0.25,
				AreaOpacity:  1,
				Stack:        timeSeriesPanel.AllStack,
				Palette:      &timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"count(sum(count_over_time((cluster_health_components_map{cluster=\"$cluster\",src_severity!=\"none\",src_alertname!~\"Watchdog\"}>0)[$__interval:1m])) by (group_id))",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
