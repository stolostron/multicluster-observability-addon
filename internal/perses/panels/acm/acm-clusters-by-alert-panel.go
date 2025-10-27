package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func ClustersWithAlertSeverity(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Clusters with Alert - Severity ($severity)",
		panel.Description("Alert: $alert - $description"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.BarDisplay,
				ConnectNulls: false,
				LineWidth:    2,
				AreaOpacity:  0.7,
				Stack:        timeSeriesPanel.AllStack,
				Palette:      timeSeriesPanel.Palette{Mode: timeSeriesPanel.AutoMode},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(ALERTS{alertstate=\"firing\", alertname=~\"$alert\", severity=~\"$severity\"}) by (cluster)",
					labelMatchers,
				),
				query.SeriesNameFormat("{{ cluster }}"),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
