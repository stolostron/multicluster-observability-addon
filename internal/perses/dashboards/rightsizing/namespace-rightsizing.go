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

func withCPUStatsAndChart(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(4),
		panelgroup.PanelHeight(5),
		panels.CPURecommendationPanel(datasource),
		panels.CPUUsagePanel(datasource),
		panels.CPURequestPanel(datasource),
		panels.CPUUtilizationPanel(datasource),
	)
}

func withCPUTopNamespaces(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.CPUTopNamespacesPanel(datasource),
	)
}

func withCPUQuotaTable(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.CPUQuotaTablePanel(datasource),
	)
}

func withMemStatsAndChart(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(4),
		panelgroup.PanelHeight(5),
		panels.MemRecommendationPanel(datasource),
		panels.MemUsagePanel(datasource),
		panels.MemRequestPanel(datasource),
		panels.MemUtilizationPanel(datasource),
	)
}

func withMemTopNamespaces(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.MemTopNamespacesPanel(datasource),
	)
}

func withMemQuotaTable(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.MemQuotaTablePanel(datasource),
	)
}

// BuildNamespaceRightSizing creates the namespace right-sizing dashboard
func BuildNamespaceRightSizing(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
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

		withCPUStatsAndChart(datasource),
		withCPUTopNamespaces(datasource),
		withCPUQuotaTable(datasource),
		withMemStatsAndChart(datasource),
		withMemTopNamespaces(datasource),
		withMemQuotaTable(datasource),
	)
}
