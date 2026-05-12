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

// BuildVMUnderestimation creates the VM underestimation detail dashboard
func BuildVMUnderestimation(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rightsizing-vm-underestimation",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing OpenShift Virtualization VM Underestimation"),
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

		dashboard.AddPanelGroup("",
			panelgroup.PanelsPerLine(4),
			panelgroup.PanelHeight(4),
			panels.VMCPUUnderestimationStatPanel(datasource),
			panels.VMCPUUsageStatPanel(datasource),
			panels.VMCPURequestStatPanel(datasource),
			panels.VMCPUUtilizationStatPanel(datasource),
		),

		dashboard.AddPanelGroup("",
			panelgroup.PanelsPerLine(1),
			panelgroup.PanelHeight(12),
			panels.VMCPUUtilizationTimeSeriesPanel(datasource),
		),

		dashboard.AddPanelGroup("",
			panelgroup.PanelsPerLine(4),
			panelgroup.PanelHeight(4),
			panels.VMMemoryUnderestimationStatPanel(datasource),
			panels.VMMemoryUsageStatPanel(datasource),
			panels.VMMemoryRequestStatPanel(datasource),
			panels.VMMemoryUtilizationStatPanel(datasource),
		),

		dashboard.AddPanelGroup("",
			panelgroup.PanelsPerLine(1),
			panelgroup.PanelHeight(12),
			panels.VMMemoryUtilizationTimeSeriesPanel(datasource),
		),
	)
}
