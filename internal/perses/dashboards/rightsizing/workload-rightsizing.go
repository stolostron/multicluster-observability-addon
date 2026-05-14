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

func withWorkloadCPUSection(datasource string, project string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("CPU",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 24, H: 12},
			{X: 0, Y: 12, W: 24, H: 10},
		},
		panels.WorkloadCPUTopWorkloadsPanel(datasource),
		panels.WorkloadCPUTablePanel(datasource, project),
	)
}

func withWorkloadMemSection(datasource string, project string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Memory",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 24, H: 12},
			{X: 0, Y: 12, W: 24, H: 10},
		},
		panels.WorkloadMemTopWorkloadsPanel(datasource),
		panels.WorkloadMemTablePanel(datasource, project),
	)
}

func withPodsSection(datasource string, project string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Pods",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 24, H: 10},
			{X: 0, Y: 10, W: 24, H: 10},
		},
		panels.PodCPUTablePanel(datasource, project),
		panels.PodMemTablePanel(datasource, project),
	)
}

func withWorkloadForecastSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Forecasting",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 12, H: 6},
			{X: 12, Y: 0, W: 12, H: 6},
		},
		panels.ForecastWorkloadCPUPanel(datasource),
		panels.ForecastWorkloadMemoryPanel(datasource),
	)
}

// BuildWorkloadPodRightSizing creates the workload-pod right-sizing dashboard
func BuildWorkloadPodRightSizing(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-workload-pod-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing Workloads & Pods"),
		dashboard.Duration(time.Hour*24*7),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_rs:workload:cpu_request",
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
							`acm_rs:workload:cpu_usage{cluster="$cluster"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Profile"),
				listVar.DefaultValue("P95"),
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
							`acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Namespace"),
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
							`acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Workload"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		withWorkloadCPUSection(datasource, project),
		withWorkloadMemSection(datasource, project),
		withPodsSection(datasource, project),
		withWorkloadForecastSection(datasource),
	)
}
