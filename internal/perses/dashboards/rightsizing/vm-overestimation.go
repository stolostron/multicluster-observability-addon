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

// BuildVMOverestimation creates the VM overestimation detail dashboard
func BuildVMOverestimation(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rightsizing-vm-overestimation",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing OpenShift Virtualization VM Overestimation"),
		dashboard.Duration(time.Hour*24*7),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`{__name__=~"acm_rs_vm:namespace:(cpu_request|cpu_usage|memory_request|memory_usage)"}`,
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
							`{__name__=~"acm_rs_vm:namespace:(cpu_request|cpu_usage|memory_request|memory_usage)",cluster="$cluster"}`,
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
					staticListVar.Values("1d", "2d", "5d", "10d", "15d", "30d", "60d", "90d"),
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
							`{__name__=~"acm_rs_vm:namespace:(cpu_request|cpu_usage|memory_request|memory_usage)",cluster="$cluster",profile="$profile"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Namespace"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddVariable("vm",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("name",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							`{__name__=~"acm_rs_vm:namespace:(cpu_request|cpu_usage|memory_request|memory_usage)",cluster="$cluster",profile="$profile",namespace="$namespace"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("VM"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		acmHelpers.AddCustomPanelGroup("",
			[]acmHelpers.GridItem{{X: 17, Y: 0, W: 7, H: 2}},
			panels.VMBackToMainDashboardPanel(datasource, project),
		),

		acmHelpers.AddCustomPanelGroup("CPU",
			[]acmHelpers.GridItem{
				{X: 0, Y: 0, W: 6, H: 4},
				{X: 0, Y: 4, W: 6, H: 4},
				{X: 0, Y: 8, W: 6, H: 4},
				{X: 0, Y: 12, W: 6, H: 4},
				{X: 6, Y: 0, W: 18, H: 16},
			},
			panels.VMCPUOverestimationStatPanel(datasource),
			panels.VMCPUUsageStatPanel(datasource),
			panels.VMCPURequestStatPanel(datasource),
			panels.VMCPUUtilizationStatPanel(datasource),
			panels.VMCPUUtilizationTimeSeriesPanel(datasource),
		),

		acmHelpers.AddCustomPanelGroup("Memory",
			[]acmHelpers.GridItem{
				{X: 0, Y: 0, W: 6, H: 4},
				{X: 0, Y: 4, W: 6, H: 4},
				{X: 0, Y: 8, W: 6, H: 4},
				{X: 0, Y: 12, W: 6, H: 4},
				{X: 6, Y: 0, W: 18, H: 16},
			},
			panels.VMMemoryOverestimationStatPanel(datasource),
			panels.VMMemoryUsageStatPanel(datasource),
			panels.VMMemoryRequestStatPanel(datasource),
			panels.VMMemoryUtilizationStatPanel(datasource),
			panels.VMMemoryUtilizationTimeSeriesPanel(datasource),
		),
	)
}
