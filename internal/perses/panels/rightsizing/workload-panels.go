// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"fmt"
	"strings"

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

const (
	cpuSelector = `cluster="$cluster", profile="$cpu_profile", namespace=~"$namespace"`
	memSelector = `cluster="$cluster", profile="$memory_profile", namespace=~"$namespace"`
	// Detail selectors use =~ like VM detail dashboards so a single selected
	// value still matches, including when a variable is set to $__all.
	wlCPUDetail = `cluster="$cluster", profile="$cpu_profile", namespace=~"$namespace", workload=~"$workload", workload_type=~"$workload_type"`
	wlMemDetail = `cluster="$cluster", profile="$memory_profile", namespace=~"$namespace", workload=~"$workload", workload_type=~"$workload_type"`
)

func workloadDataLink(project, title string) *DataLink {
	return &DataLink{
		OpenNewTab: false,
		Title:      title,
		URL: fmt.Sprintf(
			"/monitoring/v2/dashboards/view?dashboard=acm-rs-workload-detail&project=%s"+
				"&var-cluster=$cluster"+
				"&var-namespace=${__data.fields[\"namespace\"]}"+
				"&var-workload=${__data.fields[\"workload\"]}"+
				"&var-workload_type=${__data.fields[\"workload_type\"]}"+
				"&var-days=$days&var-cpu_profile=$cpu_profile&var-memory_profile=$memory_profile&start=$__range",
			project,
		),
	}
}

func podDataLink(project, title string) *DataLink {
	return &DataLink{OpenNewTab: false, Title: title, URL: fmt.Sprintf(
		"/monitoring/v2/dashboards/view?dashboard=acm-rs-pod-detail&project=%s&var-cluster=$cluster&var-namespace=${__data.fields[\"namespace\"]}&var-pod=${__data.fields[\"pod\"]}&var-days=$days&var-cpu_profile=$cpu_profile&var-memory_profile=$memory_profile&start=$__range", project)}
}

func wlWindow(metric, selector, by string) string {
	if by == "" {
		return fmt.Sprintf(`max_over_time(sum by (cluster)(%s{%s})[$days:])`, metric, selector)
	}
	return fmt.Sprintf(`max_over_time(sum by (%s) (%s{%s})[$days:])`, by, metric, selector)
}

func wlUtil(usageMetric, requestMetric, selector, by string) string {
	return wlWindow(usageMetric, selector, by) + " / " + wlWindow(requestMetric, selector, by)
}

func WorkloadCPURecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Recommendation",
		Description: "CPU recommendation across all workloads in the selected cluster",
		Query:       wlWindow("acm_rs:workload:cpu_recommendation", cpuSelector, ""),
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadCPUUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Usage",
		Description: "CPU usage across all workloads in the selected cluster",
		Query:       wlWindow("acm_rs:workload:cpu_usage", cpuSelector, ""),
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadCPURequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Request",
		Description: "CPU request across all workloads in the selected cluster",
		Query:       wlWindow("acm_rs:workload:cpu_request", cpuSelector, ""),
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadCPULimitPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Limit",
		Description: "CPU limit across all workloads in the selected cluster",
		Query:       wlWindow("acm_rs:workload:cpu_limit", cpuSelector, ""),
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadCPUUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Utilization",
		Description: "CPU utilization percentage across all workloads (usage / request)",
		Query:       wlUtil("acm_rs:workload:cpu_usage", "acm_rs:workload:cpu_request", cpuSelector, ""),
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
		Query:       wlWindow("acm_rs:workload:memory_recommendation", memSelector, ""),
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
		Query:       wlWindow("acm_rs:workload:memory_usage", memSelector, ""),
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
		Query:       wlWindow("acm_rs:workload:memory_request", memSelector, ""),
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadMemLimitPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Limit",
		Description: "Memory limit across all workloads in the selected cluster",
		Query:       wlWindow("acm_rs:workload:memory_limit", memSelector, ""),
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadMemUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Utilization",
		Description: "Memory utilization percentage across all workloads (usage / request)",
		Query:       wlUtil("acm_rs:workload:memory_usage", "acm_rs:workload:memory_request", memSelector, ""),
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsUtilizationThreshold,
	})
}

func timeSeriesPercentChart() panel.Option {
	return timeSeriesPanel.Chart(
		timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
			Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
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
	)
}

func WorkloadCPUTopWorkloadsPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage to Request Ratio of Top Workloads",
		panel.Description("CPU utilization of the top 20 workloads by usage/request ratio"),
		timeSeriesPercentChart(),
		panel.AddQuery(
			query.PromQL(
				`topk(20, sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_usage{cluster="$cluster", profile="$cpu_profile", namespace=~"$namespace"}) / sum by (namespace, workload, workload_type) (acm_rs:workload:cpu_request{cluster="$cluster", profile="$cpu_profile", namespace=~"$namespace"}))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}/{{workload}} ({{workload_type}})"),
			),
		),
	)
}

func WorkloadMemTopWorkloadsPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage to Request Ratio of Top Workloads",
		panel.Description("Memory utilization of the top 20 workloads by usage/request ratio"),
		timeSeriesPercentChart(),
		panel.AddQuery(
			query.PromQL(
				`topk(20, sum by (namespace, workload, workload_type) (acm_rs:workload:memory_usage{cluster="$cluster", profile="$memory_profile", namespace=~"$namespace"}) / sum by (namespace, workload, workload_type) (acm_rs:workload:memory_request{cluster="$cluster", profile="$memory_profile", namespace=~"$namespace"}))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}/{{workload}} ({{workload_type}})"),
			),
		),
	)
}

func resourceTable(title, description, datasourceName, selector, usage, request, limit, rec string, cpu bool, project string, includePod bool) panelgroup.Option {
	detailLink := workloadDataLink(project, "Workload Detailed View")
	if includePod {
		detailLink = podDataLink(project, "Pod Detailed View")
	}
	by := "namespace, workload, workload_type"
	joinCols := []string{"namespace", "workload", "workload_type"}
	cellSettings := []tablePanel.CellSettings{
		{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
	}
	usageUnit := &dashboards.DecimalUnit
	if !cpu {
		usageUnit = &dashboards.BytesUnit
	}
	cols := []ColumnSettingsWithLink{
		{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
		{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
		{ColumnSettings: tablePanel.ColumnSettings{Name: "workload", Header: "Workload", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
		nsTblCol("workload_type", "Type", tablePanel.LeftAlign, nil),
	}
	if includePod {
		// Leave absent identity/resource fields blank for pods. Without a text
		// override, the table renders missing values as empty cells.
		cellSettings = nil
		by = "namespace, pod, workload, workload_type"
		joinCols = []string{"namespace", "pod", "workload", "workload_type"}
		cols = []ColumnSettingsWithLink{
			{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
			{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
			{ColumnSettings: tablePanel.ColumnSettings{Name: "pod", Header: "Pod", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
			{ColumnSettings: tablePanel.ColumnSettings{Name: "workload", Header: "Workload", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: detailLink},
			nsTblCol("workload_type", "Type", tablePanel.LeftAlign, nil),
		}
	}
	utilHeader, usageHeader, reqHeader, limHeader, recHeader := "CPU Utilization %", "CPU Usage", "CPU Request", "CPU Limit", "CPU Recommendation"
	if !cpu {
		utilHeader, usageHeader, reqHeader, limHeader, recHeader = "Memory Utilization %", "Memory Usage", "Memory Request", "Memory Limit", "Memory Recommendation"
	}
	cols = append(cols,
		nsTblCol("value #1", utilHeader, tablePanel.RightAlign,
			&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
			func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
		nsTblCol("value #2", usageHeader, tablePanel.RightAlign,
			&commonSdk.Format{Unit: usageUnit, DecimalPlaces: 2}),
		nsTblCol("value #3", reqHeader, tablePanel.RightAlign,
			&commonSdk.Format{Unit: usageUnit, DecimalPlaces: 2}),
		nsTblCol("value #4", limHeader, tablePanel.RightAlign,
			&commonSdk.Format{Unit: usageUnit, DecimalPlaces: 2}),
		nsTblCol("value #5", recHeader, tablePanel.RightAlign,
			&commonSdk.Format{Unit: usageUnit, DecimalPlaces: 2}),
	)
	if includePod {
		cols = append(cols, ColumnSettingsWithLink{ColumnSettings: tablePanel.ColumnSettings{Name: "value #6", Hide: true}})
	}
	options := []panel.Option{
		panel.Description(description),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: cols,
			CellSettings:   cellSettings,
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: joinCols}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(wlUtil(usage, request, selector, by), dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(wlWindow(usage, selector, by), dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(wlWindow(request, selector, by), dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(wlWindow(limit, selector, by), dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(wlWindow(rec, selector, by), dashboards.AddQueryDataSource(datasourceName))),
	}
	if includePod {
		// Inventory has no profile: keep rows even when CPU/memory measurements
		// or resource requests/limits are unavailable (for example, pending pods).
		inventorySelector := strings.NewReplacer(`profile="$cpu_profile", `, "", `profile="$memory_profile", `, "").Replace(selector)
		options = append(options, panel.AddQuery(query.PromQL(wlWindow("acm_rs:pod:info", inventorySelector, by), dashboards.AddQueryDataSource(datasourceName))))
	}
	return panelgroup.AddPanel(title, options...)
}

func WorkloadCPUTablePanel(datasourceName string, project string) panelgroup.Option {
	return resourceTable("Workload CPU Table",
		"CPU utilization, usage, request, limit, and recommendation per workload.\nClick Workload or Namespace to see detailed view.",
		datasourceName, cpuSelector,
		"acm_rs:workload:cpu_usage", "acm_rs:workload:cpu_request", "acm_rs:workload:cpu_limit", "acm_rs:workload:cpu_recommendation",
		true, project, false)
}

func WorkloadMemTablePanel(datasourceName string, project string) panelgroup.Option {
	return resourceTable("Workload Memory Table",
		"Memory utilization, usage, request, limit, and recommendation per workload.\nClick Workload or Namespace to see detailed view.",
		datasourceName, memSelector,
		"acm_rs:workload:memory_usage", "acm_rs:workload:memory_request", "acm_rs:workload:memory_limit", "acm_rs:workload:memory_recommendation",
		false, project, false)
}

func PodCPUTablePanel(datasourceName string, project string) panelgroup.Option {
	return resourceTable("Pod CPU Table",
		"CPU utilization, usage, request, limit, and recommendation per pod.",
		datasourceName, cpuSelector,
		"acm_rs:pod:cpu_usage", "acm_rs:pod:cpu_request", "acm_rs:pod:cpu_limit", "acm_rs:pod:cpu_recommendation",
		true, project, true)
}

func PodMemTablePanel(datasourceName string, project string) panelgroup.Option {
	return resourceTable("Pod Memory Table",
		"Memory utilization, usage, request, limit, and recommendation per pod.",
		datasourceName, memSelector,
		"acm_rs:pod:memory_usage", "acm_rs:pod:memory_request", "acm_rs:pod:memory_limit", "acm_rs:pod:memory_recommendation",
		false, project, true)
}

const (
	podCPUDetail = `cluster="$cluster", profile="$cpu_profile", namespace="$namespace", pod="$pod"`
	podMemDetail = `cluster="$cluster", profile="$memory_profile", namespace="$namespace", pod="$pod"`
)

func podDetailStat(datasource, title, description, metric, selector string, unit *string, decimals int, thresholds *commonSdk.Thresholds) panelgroup.Option {
	return BuildStatPanel(datasource, StatPanelConfig{
		Title: title, Description: description,
		Query: `max by (cluster, profile, namespace, pod, workload, workload_type)(max_over_time(` + metric + `{` + selector + `}[$days:]))`,
		Unit:  unit, Decimals: decimals, FontSize: 40, Thresholds: thresholds,
	})
}

func podDetailRatioStat(datasource, title, description, usage, request, selector string, unit *string, thresholds *commonSdk.Thresholds) panelgroup.Option {
	return BuildStatPanel(datasource, StatPanelConfig{
		Title: title, Description: description,
		Query: `max by (cluster, profile, namespace, pod, workload, workload_type)(max_over_time(` + usage + `{` + selector + `}[$days:]) / max_over_time(` + request + `{` + selector + `}[$days:]))`,
		Unit:  unit, Decimals: 1, FontSize: 40, Thresholds: thresholds,
	})
}

func PodDetailCPURecommendationStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "CPU Recommendation", "Recommended CPU cores for the selected pod.", "acm_rs:pod:cpu_recommendation", podCPUDetail, &dashboards.DecimalUnit, 2, nsStatThreshold)
}

func PodDetailCPUUsageStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "CPU Usage", "Actual CPU cores consumed by the selected pod.", "acm_rs:pod:cpu_usage", podCPUDetail, &dashboards.DecimalUnit, 2, detailGrayThreshold)
}

func PodDetailCPURequestStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "CPU Request", "CPU cores requested by the selected pod.", "acm_rs:pod:cpu_request", podCPUDetail, &dashboards.DecimalUnit, 2, detailGrayThreshold)
}

func PodDetailCPULimitStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "CPU Limit", "CPU cores limit for the selected pod.", "acm_rs:pod:cpu_limit", podCPUDetail, &dashboards.DecimalUnit, 2, detailGrayThreshold)
}

func PodDetailCPUUtilizationStatPanel(datasource string) panelgroup.Option {
	return podDetailRatioStat(datasource, "CPU Utilization", "CPU usage divided by CPU request for the selected pod.", "acm_rs:pod:cpu_usage", "acm_rs:pod:cpu_request", podCPUDetail, &dashboards.PercentDecimalUnit, detailPercentThreshold)
}

func PodDetailMemRecommendationStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "Memory Recommendation", "Recommended memory for the selected pod.", "acm_rs:pod:memory_recommendation", podMemDetail, &dashboards.BytesUnit, 1, nsStatThreshold)
}

func PodDetailMemUsageStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "Memory Usage", "Actual memory consumed by the selected pod.", "acm_rs:pod:memory_usage", podMemDetail, &dashboards.BytesUnit, 1, detailGrayThreshold)
}

func PodDetailMemRequestStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "Memory Request", "Memory requested by the selected pod.", "acm_rs:pod:memory_request", podMemDetail, &dashboards.BytesUnit, 1, detailGrayThreshold)
}

func PodDetailMemLimitStatPanel(datasource string) panelgroup.Option {
	return podDetailStat(datasource, "Memory Limit", "Memory limit for the selected pod.", "acm_rs:pod:memory_limit", podMemDetail, &dashboards.BytesUnit, 1, detailGrayThreshold)
}

func PodDetailMemUtilizationStatPanel(datasource string) panelgroup.Option {
	return podDetailRatioStat(datasource, "Memory Utilization", "Memory usage divided by memory request for the selected pod.", "acm_rs:pod:memory_usage", "acm_rs:pod:memory_request", podMemDetail, &dashboards.PercentDecimalUnit, detailPercentThreshold)
}

func PodDetailCPUTimeSeriesPanel(datasource string) panelgroup.Option {
	return detailTimeSeries("CPU Over Time - Pod", "CPU usage, request, limit, and recommendation for the selected pod.", datasource, &dashboards.DecimalUnit,
		"acm_rs:pod:cpu_usage", "acm_rs:pod:cpu_request", "acm_rs:pod:cpu_limit", "acm_rs:pod:cpu_recommendation", podCPUDetail)
}

func PodDetailMemTimeSeriesPanel(datasource string) panelgroup.Option {
	return detailTimeSeries("Memory Over Time - Pod", "Memory usage, request, limit, and recommendation for the selected pod.", datasource, &dashboards.BytesUnit,
		"acm_rs:pod:memory_usage", "acm_rs:pod:memory_request", "acm_rs:pod:memory_limit", "acm_rs:pod:memory_recommendation", podMemDetail)
}

func WorkloadDetailCPURecommendationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Recommendation",
		Description: "Recommended CPU cores for the selected workload based on usage profile.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_recommendation{` + wlCPUDetail + `}[$days:]))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func WorkloadDetailCPUUsageStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Usage",
		Description: "Actual CPU cores consumed by the selected workload over the aggregation period.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_usage{` + wlCPUDetail + `}[$days:]))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func WorkloadDetailCPURequestStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Request",
		Description: "CPU cores requested (allocated) for the selected workload.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_request{` + wlCPUDetail + `}[$days:]))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func WorkloadDetailCPULimitStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Limit",
		Description: "CPU cores limit for the selected workload.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_limit{` + wlCPUDetail + `}[$days:]))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func WorkloadDetailCPUUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Utilization",
		Description: "CPU utilization ratio for the selected workload.\nCalculated as CPU Usage / CPU Request.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:cpu_usage{` + wlCPUDetail + `}[$days:]) / max_over_time(acm_rs:workload:cpu_request{` + wlCPUDetail + `}[$days:]))`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  detailPercentThreshold,
	})
}

func detailTimeSeries(title, description, datasourceName string, unit *string, usage, request, limit, rec, selector string) panelgroup.Option {
	return panelgroup.AddPanel(title,
		panel.Description(description),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Format: &commonSdk.Format{Unit: unit, DecimalPlaces: 2},
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
		panel.AddQuery(query.PromQL(usage+`{`+selector+`}`, dashboards.AddQueryDataSource(datasourceName), query.SeriesNameFormat("Usage"))),
		panel.AddQuery(query.PromQL(request+`{`+selector+`}`, dashboards.AddQueryDataSource(datasourceName), query.SeriesNameFormat("Request"))),
		panel.AddQuery(query.PromQL(limit+`{`+selector+`}`, dashboards.AddQueryDataSource(datasourceName), query.SeriesNameFormat("Limit"))),
		panel.AddQuery(query.PromQL(rec+`{`+selector+`}`, dashboards.AddQueryDataSource(datasourceName), query.SeriesNameFormat("Recommendation"))),
	)
}

func WorkloadDetailCPUTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return detailTimeSeries("CPU Over Time - Workload",
		"CPU usage, request, limit, and recommendation over time for the selected workload.",
		datasourceName, &dashboards.DecimalUnit,
		"acm_rs:workload:cpu_usage", "acm_rs:workload:cpu_request", "acm_rs:workload:cpu_limit", "acm_rs:workload:cpu_recommendation",
		wlCPUDetail)
}

func WorkloadDetailMemRecommendationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Recommendation",
		Description: "Recommended memory for the selected workload based on usage profile.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_recommendation{` + wlMemDetail + `}[$days:]))`,
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
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_usage{` + wlMemDetail + `}[$days:]))`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func WorkloadDetailMemRequestStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Request",
		Description: "Memory requested (allocated) for the selected workload.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_request{` + wlMemDetail + `}[$days:]))`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func WorkloadDetailMemLimitStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Limit",
		Description: "Memory limit for the selected workload.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_limit{` + wlMemDetail + `}[$days:]))`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func WorkloadDetailMemUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Utilization",
		Description: "Memory utilization ratio for the selected workload.\nCalculated as Memory Usage / Memory Request.",
		Query:       `max by (cluster, profile, namespace, workload, workload_type)(max_over_time(acm_rs:workload:memory_usage{` + wlMemDetail + `}[$days:]) / max_over_time(acm_rs:workload:memory_request{` + wlMemDetail + `}[$days:]))`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  detailPercentThreshold,
	})
}

func WorkloadDetailMemTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return detailTimeSeries("Memory Over Time - Workload",
		"Memory usage, request, limit, and recommendation over time for the selected workload.",
		datasourceName, &dashboards.BytesUnit,
		"acm_rs:workload:memory_usage", "acm_rs:workload:memory_request", "acm_rs:workload:memory_limit", "acm_rs:workload:memory_recommendation",
		wlMemDetail)
}

func WorkloadDetailPodCPUTablePanel(datasourceName string, project string) panelgroup.Option {
	return resourceTable("Pod CPU Table",
		"CPU usage, request, limit, and recommendation for pods in the selected workload.",
		datasourceName, wlCPUDetail,
		"acm_rs:pod:cpu_usage", "acm_rs:pod:cpu_request", "acm_rs:pod:cpu_limit", "acm_rs:pod:cpu_recommendation",
		true, project, true)
}

func WorkloadDetailPodMemTablePanel(datasourceName string, project string) panelgroup.Option {
	return resourceTable("Pod Memory Table",
		"Memory usage, request, limit, and recommendation for pods in the selected workload.",
		datasourceName, wlMemDetail,
		"acm_rs:pod:memory_usage", "acm_rs:pod:memory_request", "acm_rs:pod:memory_limit", "acm_rs:pod:memory_recommendation",
		false, project, true)
}

func WorkloadBackToMainDashboardPanel(_ string, project string) panelgroup.Option {
	backURL := fmt.Sprintf(
		"/monitoring/v2/dashboards/view?dashboard=acm-rs-workload-overview&project=%s"+
			"&var-cluster=$cluster&var-namespace=$namespace&var-days=$days"+
			"&var-cpu_profile=$cpu_profile&var-memory_profile=$memory_profile&start=$__range",
		project,
	)
	return panelgroup.AddPanel("Back to Main Dashboard",
		panel.Description("Back to Main Dashboard"),
		markdownPanel.Markdown(fmt.Sprintf("[Back to Main Dashboard](%s)", backURL)),
		panel.AddLink(backURL,
			link.Name("Back to Main Dashboard"),
			link.Tooltip("Back to Main Dashboard"),
		),
	)
}

func PodBackToMainDashboardPanel(_ string, project string) panelgroup.Option {
	backURL := fmt.Sprintf("/monitoring/v2/dashboards/view?dashboard=acm-rs-pod-overview&project=%s&var-cluster=$cluster&var-namespace=$namespace&var-days=$days&var-cpu_profile=$cpu_profile&var-memory_profile=$memory_profile&start=$__range", project)
	return panelgroup.AddPanel("Back to Pod Dashboard", panel.Description("Back to Pod Dashboard"), markdownPanel.Markdown(fmt.Sprintf("[Back to Pod Dashboard](%s)", backURL)), panel.AddLink(backURL, link.Name("Back to Pod Dashboard"), link.Tooltip("Back to Pod Dashboard")))
}
