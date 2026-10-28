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
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/rightsizing"
	acmHelpers "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm"
)

func withWorkloadCPUSection(datasource string, project string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("CPU",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 5, H: 4},
			{X: 5, Y: 0, W: 5, H: 4},
			{X: 10, Y: 0, W: 5, H: 4},
			{X: 15, Y: 0, W: 5, H: 4},
			{X: 20, Y: 0, W: 4, H: 4},
			{X: 0, Y: 4, W: 24, H: 16},
			{X: 0, Y: 20, W: 24, H: 10},
		},
		panels.WorkloadCPURecommendationPanel(datasource),
		panels.WorkloadCPUUsagePanel(datasource),
		panels.WorkloadCPURequestPanel(datasource),
		panels.WorkloadCPUUtilizationPanel(datasource),
		panels.WorkloadCPULimitPanel(datasource),
		panels.WorkloadCPUTopWorkloadsPanel(datasource),
		panels.WorkloadCPUTablePanel(datasource, project),
	)
}

func withWorkloadMemSection(datasource string, project string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Memory",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 5, H: 4},
			{X: 5, Y: 0, W: 5, H: 4},
			{X: 10, Y: 0, W: 5, H: 4},
			{X: 15, Y: 0, W: 5, H: 4},
			{X: 20, Y: 0, W: 4, H: 4},
			{X: 0, Y: 4, W: 24, H: 16},
			{X: 0, Y: 20, W: 24, H: 10},
		},
		panels.WorkloadMemRecommendationPanel(datasource),
		panels.WorkloadMemUsagePanel(datasource),
		panels.WorkloadMemRequestPanel(datasource),
		panels.WorkloadMemUtilizationPanel(datasource),
		panels.WorkloadMemLimitPanel(datasource),
		panels.WorkloadMemTopWorkloadsPanel(datasource),
		panels.WorkloadMemTablePanel(datasource, project),
	)
}

// BuildWorkloadRightSizing creates the workload right-sizing overview dashboard.
func BuildWorkloadRightSizing(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-workload-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing Workloads"),
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

		dashboard.AddVariable("cpu_profile",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("profile",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`acm_rs:workload:cpu_usage{cluster="$cluster"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("CPU Profile"),
				listVar.DefaultValue("Max OverAll"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("memory_profile",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("profile",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`acm_rs:workload:memory_usage{cluster="$cluster"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Memory Profile"),
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
							`acm_rs:workload:cpu_usage{cluster="$cluster",profile="$cpu_profile"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Namespace"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		withWorkloadCPUSection(datasource, project),
		withWorkloadMemSection(datasource, project),
	)
}
