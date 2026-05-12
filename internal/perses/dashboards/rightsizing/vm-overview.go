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
)

func withVMOverviewStatsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(4),
		panelgroup.PanelHeight(4),
		panels.VMTotalCPUOverestimationPanel(datasource),
		panels.VMTotalCPUUnderestimationPanel(datasource),
		panels.VMTotalMemOverestimationPanel(datasource),
		panels.VMTotalMemUnderestimationPanel(datasource),
	)
}

func withVMCPUOverestimationTableGroup(datasource string, project string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(10),
		panels.VMCPUOverestimationTablePanel(datasource, project),
	)
}

func withVMCPUUnderestimationTableGroup(datasource string, project string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(10),
		panels.VMCPUUnderestimationTablePanel(datasource, project),
	)
}

func withVMMemOverestimationTableGroup(datasource string, project string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(10),
		panels.VMMemOverestimationTablePanel(datasource, project),
	)
}

func withVMMemUnderestimationTableGroup(datasource string, project string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(10),
		panels.VMMemUnderestimationTablePanel(datasource, project),
	)
}

// BuildVMOverview creates the main VM right-sizing overview dashboard
func BuildVMOverview(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rightsizing-openshift-virtualization",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing OpenShift Virtualization"),
		dashboard.Duration(time.Hour*24*7),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_rs_vm:namespace:cpu_request",
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
							`acm_rs_vm:namespace:cpu_usage{cluster="$cluster"}`,
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
							`acm_rs_vm:namespace:cpu_usage{cluster="$cluster",profile="$profile"}`,
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Namespace"),
				listVar.DefaultValue("$__all"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),

		withVMOverviewStatsGroup(datasource),
		withVMCPUOverestimationTableGroup(datasource, project),
		withVMCPUUnderestimationTableGroup(datasource, project),
		withVMMemOverestimationTableGroup(datasource, project),
		withVMMemUnderestimationTableGroup(datasource, project),
	)
}
