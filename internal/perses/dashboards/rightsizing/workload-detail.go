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

func withDetailCPUSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("CPU",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 6, H: 3},
			{X: 0, Y: 3, W: 6, H: 3},
			{X: 0, Y: 6, W: 6, H: 3},
			{X: 0, Y: 9, W: 6, H: 3},
			{X: 6, Y: 0, W: 18, H: 12},
			{X: 0, Y: 12, W: 24, H: 12},
		},
		panels.WorkloadDetailCPURecommendationStatPanel(datasource),
		panels.WorkloadDetailCPUUsageStatPanel(datasource),
		panels.WorkloadDetailCPURequestStatPanel(datasource),
		panels.WorkloadDetailCPUUtilizationStatPanel(datasource),
		panels.WorkloadDetailCPUTimeSeriesPanel(datasource),
		panels.WorkloadDetailPodCPUTimeSeriesPanel(datasource),
	)
}

func withDetailMemSection(datasource string) dashboard.Option {
	return acmHelpers.AddCustomPanelGroup("Memory",
		[]acmHelpers.GridItem{
			{X: 0, Y: 0, W: 6, H: 3},
			{X: 0, Y: 3, W: 6, H: 3},
			{X: 0, Y: 6, W: 6, H: 3},
			{X: 0, Y: 9, W: 6, H: 3},
			{X: 6, Y: 0, W: 18, H: 12},
			{X: 0, Y: 12, W: 24, H: 12},
		},
		panels.WorkloadDetailMemRecommendationStatPanel(datasource),
		panels.WorkloadDetailMemUsageStatPanel(datasource),
		panels.WorkloadDetailMemRequestStatPanel(datasource),
		panels.WorkloadDetailMemUtilizationStatPanel(datasource),
		panels.WorkloadDetailMemTimeSeriesPanel(datasource),
		panels.WorkloadDetailPodMemTimeSeriesPanel(datasource),
	)
}

// BuildWorkloadDetail creates the workload detail drill-down dashboard.
// It shows CPU and Memory stat panels and time series for a single workload,
// reached by clicking a row in the overview table.
func BuildWorkloadDetail(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-workload-detail",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing Workload Detail"),
		dashboard.Duration(time.Hour*24*7),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`{__name__=~"acm_rs:workload:(cpu_request|cpu_usage|memory_request|memory_usage)"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Cluster"),
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
							`{__name__=~"acm_rs:workload:(cpu_request|cpu_usage|memory_request|memory_usage)",cluster="$cluster"}`,
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
					staticListVar.Values("1d", "2d", "5d", "10d", "15d", "30d", "60d", "90d"),
				),
				listVar.DisplayName("Aggregation"),
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
							`{__name__=~"acm_rs:workload:(cpu_request|cpu_usage|memory_request|memory_usage)",cluster="$cluster",profile="$profile"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Namespace"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("workload",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("workload",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`{__name__=~"acm_rs:workload:(cpu_request|cpu_usage|memory_request|memory_usage)",cluster="$cluster",profile="$profile",namespace="$namespace"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Workload"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("workload_type",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("workload_type",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`{__name__=~"acm_rs:workload:(cpu_request|cpu_usage|memory_request|memory_usage)",cluster="$cluster",profile="$profile",namespace="$namespace",workload="$workload"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Type"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		acmHelpers.AddCustomPanelGroup("",
			[]acmHelpers.GridItem{{X: 17, Y: 0, W: 7, H: 2}},
			panels.WorkloadBackToMainDashboardPanel(datasource, project),
		),

		withDetailCPUSection(datasource),
		withDetailMemSection(datasource),
	)
}
