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

// ForecastVMCPUPanel compares predicted VM CPU usage to actual VM CPU usage over time.
func ForecastVMCPUPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM CPU Forecast vs Actual",
		panel.Description("Forecasted VM CPU from the prediction engine versus observed CPU usage for the selected namespace and VM"),
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
				`acm_rs:prediction_forecast_vm_cpu{namespace="$namespace",name="$name"}`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("Forecast"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`acm_rs_vm:namespace:cpu_usage{namespace="$namespace",name="$name"}`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("Actual"),
			),
		),
	)
}
