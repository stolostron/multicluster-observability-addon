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
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/rightsizing"
	acmHelpers "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm"
)

// BuildWorkloadDetail creates the workload detail drill-down dashboard.
func BuildWorkloadDetail(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return BuildWorkloadDetailForFeatures(project, datasource, clusterLabelName, true)
}

// BuildWorkloadDetailForFeatures omits pod drill-downs when pod right-sizing is disabled.
func BuildWorkloadDetailForFeatures(project string, datasource string, clusterLabelName string, includePods bool) (dashboard.Builder, error) {
	withCPUSection := func(includePods bool) dashboard.Option {
		items := []acmHelpers.GridItem{
			{X: 0, Y: 0, W: 5, H: 4},
			{X: 5, Y: 0, W: 5, H: 4},
			{X: 10, Y: 0, W: 5, H: 4},
			{X: 15, Y: 0, W: 5, H: 4},
			{X: 20, Y: 0, W: 4, H: 4},
			{X: 0, Y: 4, W: 24, H: 12},
		}
		panelOpts := []panelgroup.Option{
			panels.WorkloadDetailCPURecommendationStatPanel(datasource),
			panels.WorkloadDetailCPUUsageStatPanel(datasource),
			panels.WorkloadDetailCPURequestStatPanel(datasource),
			panels.WorkloadDetailCPUUtilizationStatPanel(datasource),
			panels.WorkloadDetailCPULimitStatPanel(datasource),
			panels.WorkloadDetailCPUTimeSeriesPanel(datasource),
		}
		if includePods {
			items = append(items, acmHelpers.GridItem{X: 0, Y: 16, W: 24, H: 8})
			panelOpts = append(panelOpts, panels.WorkloadDetailPodCPUTablePanel(datasource, project))
		}
		return acmHelpers.AddCustomPanelGroup("CPU", items, panelOpts...)
	}

	withMemSection := func(includePods bool) dashboard.Option {
		items := []acmHelpers.GridItem{
			{X: 0, Y: 0, W: 5, H: 4},
			{X: 5, Y: 0, W: 5, H: 4},
			{X: 10, Y: 0, W: 5, H: 4},
			{X: 15, Y: 0, W: 5, H: 4},
			{X: 20, Y: 0, W: 4, H: 4},
			{X: 0, Y: 4, W: 24, H: 12},
		}
		panelOpts := []panelgroup.Option{
			panels.WorkloadDetailMemRecommendationStatPanel(datasource),
			panels.WorkloadDetailMemUsageStatPanel(datasource),
			panels.WorkloadDetailMemRequestStatPanel(datasource),
			panels.WorkloadDetailMemUtilizationStatPanel(datasource),
			panels.WorkloadDetailMemLimitStatPanel(datasource),
			panels.WorkloadDetailMemTimeSeriesPanel(datasource),
		}
		if includePods {
			items = append(items, acmHelpers.GridItem{X: 0, Y: 16, W: 24, H: 8})
			panelOpts = append(panelOpts, panels.WorkloadDetailPodMemTablePanel(datasource, project))
		}
		return acmHelpers.AddCustomPanelGroup("Memory", items, panelOpts...)
	}

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
							`acm_rs:workload:cpu_usage{cluster="$cluster",profile="$cpu_profile"}`,
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
							`acm_rs:workload:cpu_usage{cluster="$cluster",profile="$cpu_profile",namespace="$namespace"}`,
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
							`acm_rs:workload:cpu_usage{cluster="$cluster",profile="$cpu_profile",namespace="$namespace",workload="$workload"}`,
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

		withCPUSection(includePods),
		withMemSection(includePods),
	)
}
