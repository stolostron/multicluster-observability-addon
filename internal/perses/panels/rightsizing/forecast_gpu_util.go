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

// ForecastGPUUtilPanel compares predicted GPU utilization to actual namespace GPU usage over time.
func ForecastGPUUtilPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Utilization Forecast vs Actual",
		panel.Description("Forecasted GPU utilization from the prediction engine versus observed namespace GPU usage for the selected namespace"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit:          &dashboards.DecimalUnit,
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
				`acm_rs:prediction_forecast_gpu_utilization{namespace="$namespace"}`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("Forecast"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`acm_rs:namespace:gpu_usage{namespace="$namespace"}`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("Actual"),
			),
		),
	)
}
