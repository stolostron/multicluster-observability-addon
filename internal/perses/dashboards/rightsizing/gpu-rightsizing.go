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

func withGPUClusterStats(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Cluster Overview",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(4),
		panels.GPUClusterRequestPanel(datasource),
		panels.GPUClusterUsagePanel(datasource),
		panels.GPUClusterRecommendationPanel(datasource),
	)
}

func withGPUNamespaceStats(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Namespace Overview",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(4),
		panels.GPUNamespaceRequestPanel(datasource),
		panels.GPUNamespaceUsagePanel(datasource),
		panels.GPUNamespaceRecommendationPanel(datasource),
	)
}

func withGPUNamespaceDetails(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Namespace Details",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(4),
		panels.GPUNamespaceMemoryUsedPanel(datasource),
		panels.GPUNamespaceMemoryTotalPanel(datasource),
		panels.GPUNamespacePowerPanel(datasource),
	)
}

func withGPUTimeSeries(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("GPU Trends",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(10),
		panels.GPUNamespaceUtilizationTSPanel(datasource),
		panels.GPUNamespaceMemoryTSPanel(datasource),
	)
}

func withGPUTopK(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Top Consumers",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(10),
		panels.GPUTopKNamespacesByUsagePanel(datasource),
		panels.GPUTopKWorkloadsByUsagePanel(datasource),
	)
}

func withGPUNamespaceTable(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(10),
		panels.GPUNamespaceOverviewTable(datasource),
	)
}

func withGPUWorkloadTable(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(10),
		panels.GPUWorkloadOverviewTable(datasource),
	)
}

// BuildGPUUtilization creates the GPU utilization right-sizing dashboard
func BuildGPUUtilization(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-gpu-utilization",
		dashboard.ProjectName(project),
		dashboard.Name("ACM GPU Right-Sizing"),
		dashboard.Duration(time.Hour*24*7),

		dashboard.AddVariable("cluster",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("cluster",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"acm_rs:namespace:gpu_request",
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
							`acm_rs:namespace:gpu_usage{cluster="$cluster"}`,
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

		withGPUClusterStats(datasource),
		withGPUNamespaceStats(datasource),
		withGPUNamespaceDetails(datasource),
		withGPUTimeSeries(datasource),
		withGPUTopK(datasource),
		withGPUNamespaceTable(datasource),
		withGPUWorkloadTable(datasource),
	)
}
