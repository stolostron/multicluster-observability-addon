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

func withWorkloadCPUStats(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(4),
		panelgroup.PanelHeight(4),
		panels.WorkloadCPURecommendationPanel(datasource),
		panels.WorkloadCPUUsagePanel(datasource),
		panels.WorkloadCPURequestPanel(datasource),
		panels.WorkloadCPUUtilizationPanel(datasource),
	)
}

func withWorkloadCPUTopWorkloads(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.WorkloadCPUTopWorkloadsPanel(datasource),
	)
}

func withWorkloadCPUTable(datasource string, project string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(8),
		panels.WorkloadCPUTablePanel(datasource, project),
	)
}

func withWorkloadMemStats(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(4),
		panelgroup.PanelHeight(4),
		panels.WorkloadMemRecommendationPanel(datasource),
		panels.WorkloadMemUsagePanel(datasource),
		panels.WorkloadMemRequestPanel(datasource),
		panels.WorkloadMemUtilizationPanel(datasource),
	)
}

func withWorkloadMemTopWorkloads(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.WorkloadMemTopWorkloadsPanel(datasource),
	)
}

func withWorkloadMemTable(datasource string, project string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(8),
		panels.WorkloadMemTablePanel(datasource, project),
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

		withWorkloadCPUStats(datasource),
		withWorkloadCPUTopWorkloads(datasource),
		withWorkloadCPUTable(datasource, project),
		withWorkloadMemStats(datasource),
		withWorkloadMemTopWorkloads(datasource),
		withWorkloadMemTable(datasource, project),
	)
}
