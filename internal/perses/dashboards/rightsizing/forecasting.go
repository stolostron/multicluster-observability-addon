// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"time"

	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	acmHelpers "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/rightsizing"
)

func withForecastDashNSSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Namespace Forecasting",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 6, H: 4},
			{X: 6, Y: 0, W: 6, H: 4},
			{X: 0, Y: 4, W: 12, H: 7},
			{X: 12, Y: 4, W: 12, H: 7},
			{X: 0, Y: 11, W: 12, H: 5},
			{X: 12, Y: 11, W: 12, H: 5},
		},
		panels.CPUForecastPanel(datasource),
		panels.MemForecastPanel(datasource),
		panels.ForecastCPUPanel(datasource),
		panels.ForecastMemoryPanel(datasource),
		panels.ModelAccuracyPanel(datasource),
		panels.AnomalyScorePanel(datasource),
	)
}

func withForecastDashVMSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("VM Forecasting",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 12, H: 7},
			{X: 12, Y: 0, W: 12, H: 7},
		},
		panels.ForecastVMCPUPanel(datasource),
		panels.ForecastVMMemoryPanel(datasource),
	)
}

func withForecastDashWLSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Workload Forecasting",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 12, H: 7},
			{X: 12, Y: 0, W: 12, H: 7},
		},
		panels.ForecastWorkloadCPUPanel(datasource),
		panels.ForecastWorkloadMemoryPanel(datasource),
	)
}

func withForecastDashGPUSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("GPU Forecasting",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 12, H: 7},
			{X: 12, Y: 0, W: 12, H: 7},
		},
		panels.ForecastGPUUtilPanel(datasource),
		panels.ForecastGPUMemoryPanel(datasource),
	)
}

// BuildForecasting creates a dedicated forecasting dashboard aggregating all prediction panels.
func BuildForecasting(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-forecasting",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing Forecasting"),
		dashboard.Duration(time.Hour*24*7),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_rs:cluster:cpu_request",
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Cluster"),
				listVar.DefaultValue("local-cluster"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("profile",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("profile",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`acm_rs:namespace:cpu_usage{cluster="$cluster"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Profile"),
				listVar.DefaultValue("P95"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("namespace",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("namespace",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`acm_rs:namespace:cpu_usage{cluster="$cluster", profile="$profile"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Namespace"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("name",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("name",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`acm_rs_vm:namespace:cpu_usage{cluster="$cluster", namespace="$namespace"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("VM"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		withForecastDashNSSection(datasource),
		withForecastDashVMSection(datasource),
		withForecastDashWLSection(datasource),
		withForecastDashGPUSection(datasource),
	)
}
