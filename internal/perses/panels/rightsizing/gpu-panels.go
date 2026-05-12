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

const gpuNsFilter = `cluster="$cluster", profile="$profile", namespace=~"$namespace"`
const gpuWlFilter = `cluster="$cluster", profile="$profile", namespace=~"$namespace", workload_type=~"$workload_type", workload=~"$workload"`

// ===================== GPU Section =====================

func GPURecommendationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Recommendation",
		Description: "Recommended GPU utilization across selected namespaces",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_recommendation{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  greenThreshold,
	})
}

func GPUUsageStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Usage",
		Description: "GPU usage across selected namespaces",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_usage{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  grayThreshold,
	})
}

func GPURequestStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Request",
		Description: "Total GPU devices requested across selected namespaces",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_request{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  grayThreshold,
	})
}

func GPUUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Utilization",
		Description: "GPU utilization percentage (usage / request)",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_usage{` + gpuNsFilter + `})[$days:]) / max_over_time(sum by (cluster)(acm_rs:namespace:gpu_request{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  percentThreshold,
	})
}

func GPUUtilizationTopNamespacesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Utilization of Top Namespaces",
		panel.Description("GPU utilization of top namespaces over time"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit:          &dashboards.PercentDecimalUnit,
					DecimalPlaces: 2,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				ConnectNulls: true,
				LineWidth:    1.25,
				AreaOpacity:  0.8,
				PointRadius:  0,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`topk(20, acm_rs:namespace:gpu_usage{`+gpuNsFilter+`})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}"),
			),
		),
	)
}

func GPUQuotaTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Quota",
		panel.Description("GPU utilization, usage, request, and recommendation per namespace."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				nsTblCol("namespace", "Namespace", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "GPU Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "GPU Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "GPU Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "GPU Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"namespace"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_usage{`+gpuNsFilter+`})[$days:]) / max_over_time(max by (namespace)(acm_rs:namespace:gpu_request{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_usage{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_request{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_recommendation{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

// ===================== GPU Memory Section =====================

func GPUMemRecommendationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Memory Recommendation",
		Description: "Recommended GPU memory across selected namespaces",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_memory_recommendation{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  greenThreshold,
	})
}

func GPUMemUsedStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Memory Used",
		Description: "GPU memory used across selected namespaces",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_memory_used{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  grayThreshold,
	})
}

func GPUMemTotalStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Memory Total",
		Description: "Total GPU memory capacity across selected namespaces",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_memory_total{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  grayThreshold,
	})
}

func GPUMemUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "GPU Memory Utilization",
		Description: "GPU memory utilization percentage (used / total)",
		Query:       `max_over_time(sum by (cluster)(acm_rs:namespace:gpu_memory_used{` + gpuNsFilter + `})[$days:]) / max_over_time(sum by (cluster)(acm_rs:namespace:gpu_memory_total{` + gpuNsFilter + `})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  percentThreshold,
	})
}

func GPUMemUtilizationTopNamespacesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Memory Utilization of Top Namespaces",
		panel.Description("GPU memory utilization of top namespaces over time"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{
					Unit:          &dashboards.PercentDecimalUnit,
					DecimalPlaces: 2,
				},
			}),
			timeSeriesPanel.WithLegend(timeSeriesPanel.Legend{
				Position: timeSeriesPanel.BottomPosition,
				Mode:     timeSeriesPanel.ListMode,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				ConnectNulls: true,
				LineWidth:    1.25,
				AreaOpacity:  0.8,
				PointRadius:  0,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`topk(20, acm_rs:namespace:gpu_memory_used{`+gpuNsFilter+`} / acm_rs:namespace:gpu_memory_total{`+gpuNsFilter+`})`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}"),
			),
		),
	)
}

func GPUMemQuotaTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Memory Quota",
		panel.Description("GPU memory utilization, used, total, and recommendation per namespace."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				nsTblCol("namespace", "Namespace", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "GPU Memory Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "GPU Memory Used", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "GPU Memory Total", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "GPU Memory Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"namespace"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_memory_used{`+gpuNsFilter+`})[$days:]) / max_over_time(max by (namespace)(acm_rs:namespace:gpu_memory_total{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_memory_used{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_memory_total{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_memory_recommendation{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

// ===================== GPU Telemetry Section =====================

func GPUTelemetryTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("GPU Telemetry (Power / Temperature / Clocks)",
		panel.Description("GPU power, temperature, SM clock, and memory clock per namespace."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				nsTblCol("namespace", "Namespace", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "GPU Power (watts)", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "GPU Temperature (\u00b0C)", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 1}),
				nsTblCol("value #3", "GPU SM Clock (Hz)", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 0}),
				nsTblCol("value #4", "GPU Memory Clock (Hz)", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 0}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"namespace"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_power_usage_watts{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_temperature_celsius{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_sm_clock_hertz{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (namespace)(acm_rs:namespace:gpu_memory_clock_hertz{`+gpuNsFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

// ===================== Workloads Section =====================

func GPUWorkloadQuotaTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Workload GPU Quota",
		panel.Description("GPU and memory metrics per workload."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				nsTblCol("workload", "Workload", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "GPU Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "GPU Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "GPU Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "GPU Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #5", "GPU Memory Used", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #6", "GPU Memory Total", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #7", "GPU Memory Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"workload"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (workload)(acm_rs:workload:gpu_usage{`+gpuWlFilter+`})[$days:]) / max_over_time(max by (workload)(acm_rs:workload:gpu_request{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (workload)(acm_rs:workload:gpu_usage{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (workload)(acm_rs:workload:gpu_request{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (workload)(acm_rs:workload:gpu_recommendation{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (workload)(acm_rs:workload:gpu_memory_used{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (workload)(acm_rs:workload:gpu_memory_total{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (workload)(acm_rs:workload:gpu_memory_recommendation{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

func GPUPodQuotaTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Pod GPU Quota",
		panel.Description("GPU and memory metrics per pod."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				nsTblCol("pod", "Pod", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "GPU Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "GPU Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "GPU Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "GPU Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #5", "GPU Memory Used", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #6", "GPU Memory Total", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #7", "GPU Memory Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"pod"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (pod)(acm_rs:pod:gpu_usage{`+gpuWlFilter+`})[$days:]) / max_over_time(max by (pod)(acm_rs:pod:gpu_request{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (pod)(acm_rs:pod:gpu_usage{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (pod)(acm_rs:pod:gpu_request{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (pod)(acm_rs:pod:gpu_recommendation{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (pod)(acm_rs:pod:gpu_memory_used{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (pod)(acm_rs:pod:gpu_memory_total{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(max by (pod)(acm_rs:pod:gpu_memory_recommendation{`+gpuWlFilter+`})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}
