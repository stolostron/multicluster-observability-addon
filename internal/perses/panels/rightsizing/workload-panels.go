// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"fmt"

	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/link"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	markdownPanel "github.com/perses/plugins/markdown/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func workloadDataLink(project, title string) *DataLink {
	return &DataLink{
		OpenNewTab: false,
		Title:      title,
		URL: fmt.Sprintf(
			"/monitoring/v2/dashboards/view?dashboard=acm-rs-workload-detail&project=%s"+
				"&var-cluster=${__data.fields[\"cluster\"]}"+
				"&var-namespace=${__data.fields[\"namespace\"]}"+
				"&var-workload=${__data.fields[\"workload\"]}"+
				"&var-workload_type=${__data.fields[\"workload_type\"]}"+
				"&var-days=$days&var-profile=${__data.fields[\"profile\"]}",
			project,
		),
	}
}

// --- Workload-level stat panels ---

func WorkloadCPURecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Recommendation",
		Description: "CPU recommendation across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    5,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadCPUUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Usage",
		Description: "CPU usage across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    5,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadCPURequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Request",
		Description: "CPU request across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    5,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadCPUUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Utilization",
		Description: "CPU utilization percentage across all workloads",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:]) / max_over_time(sum by (cluster)(acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsUtilizationThreshold,
	})
}

func WorkloadMemRecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Recommendation",
		Description: "Memory recommendation across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadMemUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Usage",
		Description: "Memory usage across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadMemRequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Request",
		Description: "Memory request across all workloads in the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadMemUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Utilization",
		Description: "Memory utilization percentage across all workloads",
		Query:       `max_over_time(sum by (cluster)(acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:]) / max_over_time(sum by (cluster)(acm_rs:workload:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsUtilizationThreshold,
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
				`topk(20, sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"}))`,
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
				`topk(20, sum by (namespace, workload, workload_type) (acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / sum by (namespace, workload, workload_type) (acm_rs:workload:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"}))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}/{{workload}} ({{workload_type}})"),
			),
		),
	)
}

// --- Workload table ---

func WorkloadCPUTablePanel(datasourceName string, project string) panelgroup.Option {
	detailLink := workloadDataLink(project, "Workload Detailed View")
	return panelgroup.AddPanel("Workload CPU Table",
		panel.Description("CPU utilization, usage, request, limit, and recommendation per workload.\nClick Workload or Namespace to see detailed view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "workload", Header: "Workload", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				nsTblCol("workload_type", "Type", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "CPU Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "CPU Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
				nsTblCol("value #3", "CPU Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
				nsTblCol("value #4", "CPU Limit", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
				nsTblCol("value #5", "CPU Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"namespace", "workload", "workload_type"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:]) / max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_limit{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

func WorkloadMemTablePanel(datasourceName string, project string) panelgroup.Option {
	detailLink := workloadDataLink(project, "Workload Detailed View")
	return panelgroup.AddPanel("Workload Memory Table",
		panel.Description("Memory utilization, usage, request, limit, and recommendation per workload.\nClick Workload or Namespace to see detailed view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "workload", Header: "Workload", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
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
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"namespace", "workload", "workload_type"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:]) / max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_limit{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (namespace, workload, workload_type) (acm_rs:workload:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

// --- Workload Detail Dashboard Panels ---

const wlDetailFilter = `cluster="$cluster", profile="$profile", namespace="$namespace", workload="$workload", workload_type="$workload_type"`

func WorkloadDetailCPURecommendationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Recommendation",
		Description: "Recommended CPU cores for the selected workload based on usage profile.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_recommendation{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    5,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadDetailCPUUsageStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Usage",
		Description: "Actual CPU cores consumed by the selected workload over the aggregation period.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_usage{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    5,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadDetailCPURequestStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Request",
		Description: "CPU cores requested (allocated) for the selected workload.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_request{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    5,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadDetailCPUUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Utilization",
		Description: "CPU utilization ratio for the selected workload.\nCalculated as CPU Usage / CPU Request.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_usage{` + wlDetailFilter + `}[$days:]) / max_over_time(acm_rs:workload:cpu_request{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsUtilizationThreshold,
	})
}

func WorkloadDetailCPUTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Over Time - Workload (Namespace)",
		panel.Description("CPU usage, request, limit, and recommendation over time for the selected workload."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5},
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
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:cpu_usage{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Usage"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:cpu_request{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Request"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:cpu_limit{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Limit"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:cpu_recommendation{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Recommendation"),
		)),
	)
}

func WorkloadDetailMemRecommendationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Recommendation",
		Description: "Recommended memory for the selected workload based on usage profile.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_recommendation{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadDetailMemUsageStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Usage",
		Description: "Actual memory consumed by the selected workload over the aggregation period.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_usage{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadDetailMemRequestStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Request",
		Description: "Memory requested (allocated) for the selected workload.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_request{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadDetailMemUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Utilization",
		Description: "Memory utilization ratio for the selected workload.\nCalculated as Memory Usage / Memory Request.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_usage{` + wlDetailFilter + `}[$days:]) / max_over_time(acm_rs:workload:memory_request{` + wlDetailFilter + `}[$days:]))`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsUtilizationThreshold,
	})
}

func WorkloadDetailMemTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Over Time - Workload (Namespace)",
		panel.Description("Memory usage, request, limit, and recommendation over time for the selected workload."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
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
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:memory_usage{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Usage"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:memory_request{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Request"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:memory_limit{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Limit"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:workload:memory_recommendation{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Recommendation"),
		)),
	)
}

func WorkloadBackToMainDashboardPanel(datasourceName string, project string) panelgroup.Option {
	backURL := fmt.Sprintf("/monitoring/v2/dashboards/view?dashboard=acm-rs-workload-pod-overview&project=%s", project)
	return panelgroup.AddPanel("Back to Main Dashboard",
		panel.Description("Back to Main Dashboard"),
		markdownPanel.Markdown(fmt.Sprintf("[Back to Main Dashboard](%s)", backURL)),
		panel.AddLink(backURL,
			link.Name("Back to Main Dashboard"),
			link.Tooltip("Back to Main Dashboard"),
		),
	)
}

// --- Pod table ---

func PodCPUTablePanel(datasourceName string, project string) panelgroup.Option {
	detailLink := workloadDataLink(project, "Workload Detailed View")
	return panelgroup.AddPanel("Pod CPU Table",
		panel.Description("CPU utilization, usage, request, limit, and recommendation per pod.\nClick Pod, Workload, or Namespace to see detailed workload view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "pod", Header: "Pod", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "workload", Header: "Workload", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				nsTblCol("workload_type", "Type", tablePanel.LeftAlign, nil),
				nsTblCol("value #1", "CPU Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "CPU Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
				nsTblCol("value #3", "CPU Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
				nsTblCol("value #4", "CPU Limit", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
				nsTblCol("value #5", "CPU Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5}),
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"pod", "namespace", "workload", "workload_type"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:]) / max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:cpu_limit{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

func PodMemTablePanel(datasourceName string, project string) panelgroup.Option {
	detailLink := workloadDataLink(project, "Workload Detailed View")
	return panelgroup.AddPanel("Pod Memory Table",
		panel.Description("Memory utilization, usage, request, limit, and recommendation per pod.\nClick Pod, Workload, or Namespace to see detailed workload view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "pod", Header: "Pod", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "workload", Header: "Workload", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
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
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"pod", "namespace", "workload", "workload_type"}}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:]) / max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:memory_limit{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(
			`max_over_time(sum by (pod, namespace, workload, workload_type) (acm_rs:pod:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$days:])`,
			dashboards.AddQueryDataSource(datasourceName))),
	)
}

// --- Pod-level detail time series (for workload detail dashboard) ---

func WorkloadDetailPodCPUTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Over Time - Pods",
		panel.Description("CPU usage, request, and recommendation per pod for the selected workload."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 5},
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
		panel.AddQuery(query.PromQL(
			`acm_rs:pod:cpu_usage{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{pod}} Usage"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:pod:cpu_request{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{pod}} Request"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:pod:cpu_recommendation{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{pod}} Recommendation"),
		)),
	)
}

func WorkloadDetailPodMemTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Over Time - Pods",
		panel.Description("Memory usage, request, and recommendation per pod for the selected workload."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
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
		panel.AddQuery(query.PromQL(
			`acm_rs:pod:memory_usage{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{pod}} Usage"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:pod:memory_request{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{pod}} Request"),
		)),
		panel.AddQuery(query.PromQL(
			`acm_rs:pod:memory_recommendation{`+wlDetailFilter+`}`,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{pod}} Recommendation"),
		)),
	)
}
