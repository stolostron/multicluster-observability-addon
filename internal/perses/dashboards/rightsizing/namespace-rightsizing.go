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
	staticListVar "github.com/perses/plugins/staticlistvariable/sdk/go"
	acmHelpers "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/rightsizing"
)

func withCPUSection(datasource string, project string, linkToWorkload bool) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("CPU",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 5, H: 4},
			{X: 0, Y: 4, W: 5, H: 4},
			{X: 0, Y: 8, W: 5, H: 4},
			{X: 0, Y: 12, W: 5, H: 4},
			{X: 5, Y: 12, W: 5, H: 4},
			{X: 5, Y: 0, W: 19, H: 12},
			{X: 0, Y: 16, W: 24, H: 10},
		},
		panels.CPURecommendationPanel(datasource),
		panels.CPUUsagePanel(datasource),
		panels.CPURequestPanel(datasource),
		panels.CPUUtilizationPanel(datasource),
		panels.CPUForecastPanel(datasource),
		panels.CPUTopNamespacesPanel(datasource),
		panels.CPUQuotaTablePanel(datasource, project, linkToWorkload),
	)
}

func withMemSection(datasource string, project string, linkToWorkload bool) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Memory",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 5, H: 4},
			{X: 0, Y: 4, W: 5, H: 4},
			{X: 0, Y: 8, W: 5, H: 4},
			{X: 0, Y: 12, W: 5, H: 4},
			{X: 5, Y: 12, W: 5, H: 4},
			{X: 5, Y: 0, W: 19, H: 12},
			{X: 0, Y: 16, W: 24, H: 10},
		},
		panels.MemRecommendationPanel(datasource),
		panels.MemUsagePanel(datasource),
		panels.MemRequestPanel(datasource),
		panels.MemUtilizationPanel(datasource),
		panels.MemForecastPanel(datasource),
		panels.MemTopNamespacesPanel(datasource),
		panels.MemQuotaTablePanel(datasource, project, linkToWorkload),
	)
}

func withForecastRecommendationSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Forecast Recommendations",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 5, H: 4},
			{X: 5, Y: 0, W: 5, H: 4},
			{X: 0, Y: 4, W: 24, H: 10},
			{X: 0, Y: 14, W: 24, H: 10},
		},
		panels.CPUClusterForecastPanel(datasource),
		panels.MemClusterForecastPanel(datasource),
		panels.CPUForecastRecommendationTablePanel(datasource),
		panels.MemForecastRecommendationTablePanel(datasource),
	)
}

func withForecastSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Forecasting",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 12, H: 6},
			{X: 12, Y: 0, W: 12, H: 6},
			{X: 0, Y: 6, W: 12, H: 4},
			{X: 12, Y: 6, W: 12, H: 4},
		},
		panels.ForecastCPUPanel(datasource),
		panels.ForecastMemoryPanel(datasource),
		panels.ModelAccuracyPanel(datasource),
		panels.AnomalyScorePanel(datasource),
	)
}

// BuildNamespaceRightSizing creates the namespace right-sizing dashboard.
// linkToWorkload controls whether namespace rows in the quota tables link to the workload dashboard.
func BuildNamespaceRightSizing(project string, datasource string, clusterLabelName string, linkToWorkload ...bool) (dashboard.Builder, error) {
	link := false
	if len(linkToWorkload) > 0 {
		link = linkToWorkload[0]
	}
	_ = link
	return dashboard.New("acm-rs-namespace-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing Namespace"),
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
				listVar.DisplayName("Days"),
				listVar.DefaultValue("10d"),
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

		dashboard.AddVariable("forecast_days",
			listVar.List(
				staticListVar.StaticList(
					staticListVar.Values("7d", "14d", "30d", "60d", "90d"),
				),
				listVar.DisplayName("Forecast Lookback"),
				listVar.DefaultValue("7d"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		withCPUSection(datasource, project, link),
		withMemSection(datasource, project, link),
		withForecastRecommendationSection(datasource),
		withForecastSection(datasource),
	)
}
