// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// ForecastWorkloadMemoryPanel compares predicted workload memory usage to actual workload memory usage over time.
func ForecastWorkloadMemoryPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Workload Memory Forecast vs Actual",
		panel.Description("Forecasted workload memory from the prediction engine versus observed memory usage for the selected namespace and workload"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit:          &dashboards.BytesUnit,
					DecimalPlaces: 2,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				ConnectNulls: true,
				LineWidth:    1.25,
				AreaOpacity:  0.4,
				PointRadius:  2.75,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`max(acm_rs:prediction_forecast_workload_memory{namespace="$namespace", workload="$workload"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("Forecast"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max(acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile", namespace="$namespace", workload="$workload"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("Actual"),
			),
		),
	)
}
