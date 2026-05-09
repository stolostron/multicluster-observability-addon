// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// --- GPU namespace-level stat panels ---

func GPUNamespaceRequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Request",
		Description: "Total GPU devices requested across all namespaces in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    48,
		Thresholds:  greenThreshold,
	})
}

func GPUNamespaceUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Utilization %",
		Description: "GPU utilization percentage across all namespaces in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_usage{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentUnit,
		Decimals:    1,
		FontSize:    48,
		Thresholds:  grayThreshold,
	})
}

func GPUNamespaceRecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Recommendation",
		Description: "Recommended GPU utilization after right-sizing in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentUnit,
		Decimals:    1,
		FontSize:    48,
		Thresholds:  greenThreshold,
	})
}

func GPUNamespaceMemoryUsedPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Memory Used",
		Description: "Total GPU memory used across all namespaces in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_memory_used{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    2,
		FontSize:    48,
		Thresholds:  grayThreshold,
	})
}

func GPUNamespaceMemoryTotalPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Memory Total",
		Description: "Total GPU memory capacity across all namespaces in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_memory_total{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    2,
		FontSize:    48,
		Thresholds:  grayThreshold,
	})
}

func GPUNamespacePowerPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Power (W)",
		Description: "Total GPU power usage across all namespaces in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_power_usage_watts{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    1,
		FontSize:    48,
		Thresholds:  grayThreshold,
	})
}

// --- GPU time series panels ---

func GPUNamespaceUtilizationTSPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Utilization by Namespace",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(acm_rs:namespace:gpu_usage{cluster="$cluster", profile="$profile"}[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}} Usage"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(acm_rs:namespace:gpu_recommendation{cluster="$cluster", profile="$profile"}[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}} Recommendation"),
			),
		),
	)
}

func GPUNamespaceMemoryTSPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Memory by Namespace",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit:          &dashboards.BytesUnit,
					DecimalPlaces: 2,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(acm_rs:namespace:gpu_memory_used{cluster="$cluster", profile="$profile"}[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}} Used"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(acm_rs:namespace:gpu_memory_total{cluster="$cluster", profile="$profile"}[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}} Total"),
			),
		),
	)
}

// --- GPU Top-K panels ---

func GPUTopKNamespacesByUsagePanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Top 10 Namespaces by GPU Utilization",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`topk(10, max_over_time(acm_rs:namespace:gpu_usage{cluster="$cluster", profile="$profile"}[$days:]))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}"),
			),
		),
	)
}

func GPUTopKWorkloadsByUsagePanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Top 10 Workloads by GPU Utilization",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`topk(10, max_over_time(acm_rs:workload:gpu_usage{cluster="$cluster", profile="$profile"}[$days:]))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}/{{workload}} ({{workload_type}})"),
			),
		),
	)
}

// --- GPU table panels ---

func GPUNamespaceOverviewTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Overview by Namespace",
		tablePanel.Table(),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace)(acm_rs:namespace:gpu_request{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Request"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace)(acm_rs:namespace:gpu_usage{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Utilization"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace)(acm_rs:namespace:gpu_recommendation{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Recommendation"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace)(acm_rs:namespace:gpu_memory_used{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Memory Used"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace)(acm_rs:namespace:gpu_power_usage_watts{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Power (W)"),
			),
		),
	)
}

func GPUWorkloadOverviewTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Overview by Workload",
		tablePanel.Table(),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace, workload, workload_type)(acm_rs:workload:gpu_request{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Request"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace, workload, workload_type)(acm_rs:workload:gpu_usage{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Utilization"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace, workload, workload_type)(acm_rs:workload:gpu_recommendation{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Recommendation"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace, workload, workload_type)(acm_rs:workload:gpu_memory_used{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Memory Used"),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max by (namespace, workload, workload_type)(acm_rs:workload:gpu_power_usage_watts{cluster="$cluster", profile="$profile"})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("GPU Power (W)"),
			),
		),
	)
}

// --- GPU cluster-level panels ---

func GPUClusterRequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Cluster GPU Request",
		Description: "Total GPU devices requested at cluster level",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:gpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    48,
		Thresholds:  greenThreshold,
	})
}

func GPUClusterUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Cluster GPU Utilization",
		Description: "Overall GPU utilization at cluster level",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:gpu_usage{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentUnit,
		Decimals:    1,
		FontSize:    48,
		Thresholds:  grayThreshold,
	})
}

func GPUClusterRecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Cluster GPU Recommendation",
		Description: "Recommended GPU utilization at cluster level",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:gpu_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentUnit,
		Decimals:    1,
		FontSize:    48,
		Thresholds:  greenThreshold,
	})
}
