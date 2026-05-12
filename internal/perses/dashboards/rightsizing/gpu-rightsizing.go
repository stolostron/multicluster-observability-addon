// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"time"

	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	staticListVar "github.com/perses/plugins/staticlistvariable/sdk/go"
	acmHelpers "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/rightsizing"
)

func withGPUSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("GPU",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 6, H: 3},
			{X: 0, Y: 3, W: 6, H: 3},
			{X: 0, Y: 6, W: 6, H: 3},
			{X: 0, Y: 9, W: 6, H: 3},
			{X: 6, Y: 0, W: 18, H: 12},
			{X: 0, Y: 12, W: 24, H: 10},
		},
		panels.GPURecommendationStatPanel(datasource),
		panels.GPUUsageStatPanel(datasource),
		panels.GPURequestStatPanel(datasource),
		panels.GPUUtilizationStatPanel(datasource),
		panels.GPUUtilizationTopNamespacesPanel(datasource),
		panels.GPUQuotaTable(datasource),
	)
}

func withGPUMemorySection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("GPU Memory",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 6, H: 3},
			{X: 0, Y: 3, W: 6, H: 3},
			{X: 0, Y: 6, W: 6, H: 3},
			{X: 0, Y: 9, W: 6, H: 3},
			{X: 6, Y: 0, W: 18, H: 12},
			{X: 0, Y: 12, W: 24, H: 10},
		},
		panels.GPUMemRecommendationStatPanel(datasource),
		panels.GPUMemUsedStatPanel(datasource),
		panels.GPUMemTotalStatPanel(datasource),
		panels.GPUMemUtilizationStatPanel(datasource),
		panels.GPUMemUtilizationTopNamespacesPanel(datasource),
		panels.GPUMemQuotaTable(datasource),
	)
}

func withGPUTelemetrySection(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("GPU Telemetry",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(10),
		panels.GPUTelemetryTable(datasource),
	)
}

func withGPUWorkloadsSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Workloads",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 24, H: 10},
			{X: 0, Y: 10, W: 24, H: 10},
		},
		panels.GPUWorkloadQuotaTable(datasource),
		panels.GPUPodQuotaTable(datasource),
	)
}

// BuildGPUUtilization creates the GPU utilization right-sizing dashboard
func BuildGPUUtilization(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-gpu-utilization",
		dashboard.ProjectName(project),
		dashboard.Name("ACM GPU Utilization"),
		dashboard.Duration(time.Hour*24*7),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_rs:namespace:gpu_request",
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
							`acm_rs:namespace:gpu_usage{cluster="$cluster"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Profile"),
				listVar.DefaultValue("Max OverAll"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("days",
			listVar.List(
				staticListVar.StaticList(
					staticListVar.Values("1d", "2d", "5d", "10d", "30d", "60d", "90d"),
				),
				listVar.DisplayName("Aggregation"),
				listVar.DefaultValue("5d"),
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
							`acm_rs:namespace:gpu_request{cluster="$cluster", profile="$profile"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Namespace"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		dashboard.AddVariable("workload_type",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("workload_type",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`acm_rs:workload:gpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Workload Type"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		dashboard.AddVariable("workload",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("workload",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`acm_rs:workload:gpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", workload_type=~"$workload_type"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Workload"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		withGPUSection(datasource),
		withGPUMemorySection(datasource),
		withGPUTelemetrySection(datasource),
		withGPUWorkloadsSection(datasource),
	)
}
