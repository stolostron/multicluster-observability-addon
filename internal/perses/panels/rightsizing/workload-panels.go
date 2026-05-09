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

// --- Workload-level stat panels ---

func WorkloadCPURecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Recommendation",
		Description: "CPU recommendation across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    48,
		Thresholds:  greenThreshold,
	})
}

func WorkloadCPUUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Usage",
		Description: "CPU usage across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    48,
		Thresholds:  grayThreshold,
	})
}

func WorkloadCPURequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Request",
		Description: "CPU request across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    48,
		Thresholds:  grayThreshold,
	})
}

func WorkloadCPUUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Utilization",
		Description: "CPU utilization percentage across all workloads",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (cluster)(acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    48,
		Thresholds:  percentThreshold,
	})
}

func WorkloadMemRecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Mem Recommendation",
		Description: "Memory recommendation across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  greenThreshold,
	})
}

func WorkloadMemUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Mem Usage",
		Description: "Memory usage across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  grayThreshold,
	})
}

func WorkloadMemRequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Mem Request",
		Description: "Memory request across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  grayThreshold,
	})
}

func WorkloadMemUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Mem Utilization",
		Description: "Memory utilization percentage across all workloads",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (cluster)(acm_rs:workload:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  percentThreshold,
	})
}

// --- Workload top-k time series ---

func WorkloadCPUTopWorkloadsPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilization of Top Workloads",
		panel.Description("CPU utilization of the top 20 workloads by usage/request ratio"),
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
				AreaOpacity:  0.4,
				PointRadius:  2.75,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`topk(20, sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile"}) / sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile"}))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}/{{workload}} ({{workload_type}})"),
			),
		),
	)
}

func WorkloadMemTopWorkloadsPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Utilization of Top Workloads",
		panel.Description("Memory utilization of the top 20 workloads by usage/request ratio"),
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
				AreaOpacity:  0.4,
				PointRadius:  2.75,
			}),
		),
		panel.AddQuery(
			query.PromQL(
				`topk(20, sum by (namespace, workload, workload_type) (acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile"}) / sum by (namespace, workload, workload_type) (acm_rs:workload:memory_request{cluster="$cluster", profile="$profile"}))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}/{{workload}} ({{workload_type}})"),
			),
		),
	)
}

// --- Workload table ---

func WorkloadCPUTablePanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Workload CPU Table",
		panel.Description("CPU utilization, usage, request, limit, and recommendation per workload"),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				nsTblCol("namespace", "Namespace", tablePanel.LeftAlign, nil),
				nsTblCol("workload", "Workload", tablePanel.LeftAlign, nil),
				nsTblCol("workload_type", "Type", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "CPU Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "CPU Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "CPU Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "CPU Limit", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #5", "CPU Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_limit{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

func WorkloadMemTablePanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Workload Memory Table",
		panel.Description("Memory utilization, usage, request, limit, and recommendation per workload"),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				nsTblCol("namespace", "Namespace", tablePanel.LeftAlign, nil),
				nsTblCol("workload", "Workload", tablePanel.LeftAlign, nil),
				nsTblCol("workload_type", "Type", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "Memory Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "Memory Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "Memory Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "Memory Limit", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #5", "Memory Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_limit{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}
