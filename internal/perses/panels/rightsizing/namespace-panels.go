// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"fmt"

	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func namespaceToWorkloadLink(project, title string) *DataLink {
	return &DataLink{
		OpenNewTab: false,
		Title:      title,
		URL: fmt.Sprintf(
			"/monitoring/v2/dashboards/view?dashboard=acm-rs-workload-pod-overview&project=%s"+
				"&var-cluster=$cluster"+
				"&var-namespace=${__data.fields[\"namespace\"]}"+
				"&var-days=$days&var-profile=$profile",
			project,
		),
	}
}

// nsTblCol is a shorthand to create a ColumnSettingsWithLink for namespace tables (no DataLink).
func nsTblCol(name, header string, align tablePanel.Align, format *commonSdk.Format, opts ...func(*ColumnSettingsWithLink)) ColumnSettingsWithLink {
	c := ColumnSettingsWithLink{
		ColumnSettings: tablePanel.ColumnSettings{
			Name:          name,
			Header:        header,
			Align:         align,
			Format:        format,
			EnableSorting: true,
		},
	}
	for _, fn := range opts {
		fn(&c)
	}
	return c
}

var nsStatThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#0066cc"},
	},
}

var nsUtilizationThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#E02F44"},
		{Value: 0.8, Color: "#73BF69"},
		{Value: 1.0, Color: "#E0B400"},
	},
}

func CPURecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Recommendation",
		Description: "CPU recommendation for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:cpu_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func CPUUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Usage",
		Description: "CPU usage for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:cpu_usage{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func CPURequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Request",
		Description: "CPU request for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    2,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func CPUUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Utilization",
		Description: "CPU utilization percentage for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:cpu_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (cluster)(acm_rs:cluster:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsUtilizationThreshold,
	})
}

func MemRecommendationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Recommendation",
		Description: "Memory recommendation for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:memory_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func MemUsagePanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Usage",
		Description: "Memory usage for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:memory_usage{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func MemRequestPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Request",
		Description: "Memory request for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.BytesUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsStatThreshold,
	})
}

func MemUtilizationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Utilization",
		Description: "Memory utilization percentage for the selected cluster",
		Query:       `max_over_time(sum by (cluster)(acm_rs:cluster:memory_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (cluster)(acm_rs:cluster:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    1,
		FontSize:    40,
		Thresholds:  nsUtilizationThreshold,
	})
}

func CPUTopNamespacesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilization of Top Namespaces",
		panel.Description("CPU utilization of the top 20 namespaces by usage/request ratio"),
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
				`topk(20, sum by (namespace) (acm_rs:namespace:cpu_usage{cluster="$cluster", profile="$profile"}) / sum by (namespace) (acm_rs:namespace:cpu_request{cluster="$cluster", profile="$profile"}))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}"),
			),
		),
	)
}

func MemTopNamespacesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Utilization of Top Namespaces",
		panel.Description("Memory utilization of the top 20 namespaces by usage/request ratio"),
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
				`topk(20, sum by (namespace) (acm_rs:namespace:memory_usage{cluster="$cluster", profile="$profile"}) / sum by (namespace) (acm_rs:namespace:memory_request{cluster="$cluster", profile="$profile"}))`,
				dashboards.AddQueryDataSource(datasourceName),
				query.SeriesNameFormat("{{namespace}}"),
			),
		),
	)
}

func CPUQuotaTablePanel(datasourceName string, project string) panelgroup.Option {
	wlLink := namespaceToWorkloadLink(project, "View Workloads in Namespace")
	return panelgroup.AddPanel("CPU Quota Table",
		panel.Description("CPU utilization, usage, request, recommendation, and request hard per namespace.\nClick Namespace to drill down into workload-level details."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: wlLink},
				nsTblCol("value #1", "CPU Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "CPU Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "CPU Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "CPU Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2}),
				nsTblCol("value #5", "CPU Request Hard", tablePanel.RightAlign,
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
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:cpu_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (namespace) (acm_rs:namespace:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:cpu_usage{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:cpu_request{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:cpu_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:cpu_request_hard{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func MemQuotaTablePanel(datasourceName string, project string) panelgroup.Option {
	wlLink := namespaceToWorkloadLink(project, "View Workloads in Namespace")
	return panelgroup.AddPanel("Memory Quota Table",
		panel.Description("Memory utilization, usage, request, recommendation, and request hard per namespace.\nClick Namespace to drill down into workload-level details."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", Align: tablePanel.LeftAlign, EnableSorting: true}, DataLink: wlLink},
				nsTblCol("value #1", "Memory Utilization %", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
					func(c *ColumnSettingsWithLink) { c.Sort = tablePanel.DescSort }),
				nsTblCol("value #2", "Memory Usage", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #3", "Memory Request", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #4", "Memory Recommendation", tablePanel.RightAlign,
					&commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2}),
				nsTblCol("value #5", "Memory Request Hard", tablePanel.RightAlign,
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
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:memory_usage{cluster="$cluster", profile="$profile"})[$days:]) / max_over_time(sum by (namespace) (acm_rs:namespace:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:memory_usage{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:memory_request{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:memory_recommendation{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				`max_over_time(sum by (namespace) (acm_rs:namespace:memory_request_hard{cluster="$cluster", profile="$profile"})[$days:])`,
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
