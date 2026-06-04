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

func vmDataLink(project, targetDashboard, title string) *DataLink {
	return &DataLink{
		OpenNewTab: false,
		Title:      title,
		URL: fmt.Sprintf(
			"/monitoring/v2/dashboards/view?dashboard=%s&project=%s&var-cluster=${__data.fields[\"cluster\"]}&var-namespace=${__data.fields[\"namespace\"]}&var-vm=${__data.fields[\"name\"]}&var-days=$days&var-profile=${__data.fields[\"profile\"]}",
			targetDashboard, project,
		),
	}
}

// PromQL sub-expressions used to filter table rows so that overestimation
// tables only show overestimated VMs and underestimation tables only show
// underestimated VMs (replicates Grafana's filterByValue transform).
const cpuOverestCond = `(floor(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})-` +
	`sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) > 0)`

const cpuUnderestCond = `(floor(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})-` +
	`sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) < 0)`

const memOverestCond = `(floor((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824) - ` +
	`(sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824)) > 0)`

const memUnderestCond = `(floor((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824) - ` +
	`(sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824)) < 0)`

var overestRedThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#1A7311"},
		{Value: 0, Color: "#E02F44"},
	},
}

var cpuUnderestYellowThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#535353"},
		{Value: 0, Color: "#E0B400"},
	},
}

var memUnderestYellowThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#1A7311"},
		{Value: 0, Color: "#E0B400"},
	},
}

// --- VM Overview Dashboard (main) panels ---

func VMTotalCPUOverestimationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Total CPU Overestimation",
		Description: "Total number of overestimated CPU cores across all VMs in the selected namespace(s).\nRepresents the total CPU cores that can be reclaimed.",
		Query: `sum(floor(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})` +
			"\n" + `- sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) > 0)`,
		Unit:       &dashboards.DecimalUnit,
		Decimals:   0,
		FontSize:   40,
		Thresholds: overestRedThreshold,
	})
}

func VMTotalCPUUnderestimationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Total CPU Underestimation",
		Description: "Total number of underestimated CPU cores across all VMs in the selected namespace(s).\nRepresents the total additional CPU cores needed.",
		Query: `sum(floor(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})` +
			"\n" + `- sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) < 0) * (-1)`,
		Unit:       &dashboards.DecimalUnit,
		Decimals:   0,
		FontSize:   40,
		Thresholds: cpuUnderestYellowThreshold,
	})
}

func VMTotalMemOverestimationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Total Memory Overestimation",
		Description: "Total overestimated memory across all VMs in the selected namespace(s).\nRepresents the total memory that can be reclaimed.",
		Query: `(sum(floor((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / 1073741824)` +
			"\n" + `- (sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / 1073741824)) > 0)) * 1073741824`,
		Unit:       &dashboards.BytesUnit,
		Decimals:   2,
		FontSize:   40,
		Thresholds: overestRedThreshold,
	})
}

func VMTotalMemUnderestimationPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Total Memory Underestimation",
		Description: "Total underestimated memory across all VMs in the selected namespace(s).\nRepresents the total additional memory needed.",
		Query: `(sum(floor((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / 1073741824)` +
			"\n" + `- (sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / 1073741824)) < 0) * (-1)) * 1073741824`,
		Unit:       &dashboards.BytesUnit,
		Decimals:   2,
		FontSize:   40,
		Thresholds: memUnderestYellowThreshold,
	})
}

func VMCPUOverestimationTablePanel(datasourceName string, project string) panelgroup.Option {
	overestLink := vmDataLink(project, "acm-rightsizing-vm-overestimation", "VM Detailed View")

	return panelgroup.AddPanel("CPU Overestimation",
		panel.Description("VMs with CPU overestimation - where allocated CPU (Request) exceeds the recommended amount.\nClick VM Name or Namespace to see detailed overestimation view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true, EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name_namespace", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name", Header: "VM Name", HeaderDescription: "Name of the Virtual Machine", EnableSorting: true}, DataLink: overestLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", HeaderDescription: "Kubernetes namespace where the VM is deployed", EnableSorting: true}, DataLink: overestLink},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #1", Header: "CPU Utilization %", HeaderDescription: "Ratio of CPU usage to CPU request as a percentage",
					EnableSorting: true, Sort: tablePanel.DescSort,
					Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #2", Header: "CPU Usage", HeaderDescription: "Actual CPU cores consumed by the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #3", Header: "CPU Request", HeaderDescription: "CPU cores requested/allocated for the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #4", Header: "CPU Recommendation", HeaderDescription: "Recommended CPU cores based on usage profile",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 0},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #5", Header: "CPU Overestimation", HeaderDescription: "Excess CPU cores allocated beyond recommendation (Request - Recommendation)",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 0},
				}},
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(`label_join(((sum by (name, namespace) (acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) > 0) and on (name, namespace) `+cpuOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(sum by (name, namespace) (acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) and on (name, namespace) `+cpuOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) and on (name, namespace) `+cpuOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(ceil(sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) and on (name, namespace) `+cpuOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(floor(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})-sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"}))> 0, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
	)
}

func VMCPUUnderestimationTablePanel(datasourceName string, project string) panelgroup.Option {
	underestLink := vmDataLink(project, "acm-rightsizing-vm-underestimation", "VM Detailed View")

	return panelgroup.AddPanel("CPU Underestimation",
		panel.Description("VMs with CPU underestimation - where allocated CPU (Request) is below the recommended amount.\nClick VM Name or Namespace to see detailed underestimation view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true, EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name_namespace", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name", Header: "VM Name", HeaderDescription: "Name of the Virtual Machine", EnableSorting: true}, DataLink: underestLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", HeaderDescription: "Kubernetes namespace where the VM is deployed", EnableSorting: true}, DataLink: underestLink},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #1", Header: "CPU Utilization %", HeaderDescription: "Ratio of CPU usage to CPU request as a percentage",
					EnableSorting: true, Sort: tablePanel.DescSort,
					Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #2", Header: "CPU Usage", HeaderDescription: "Actual CPU cores consumed by the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #3", Header: "CPU Request", HeaderDescription: "CPU cores requested/allocated for the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #4", Header: "CPU Recommendation", HeaderDescription: "Recommended CPU cores based on usage profile",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 0},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #5", Header: "CPU Underestimation", HeaderDescription: "Deficit of CPU cores below recommendation (Recommendation - Request)",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 0},
				}},
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(`label_join(((sum by (name, namespace) (acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) > 0) and on (name, namespace) `+cpuUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(sum by (name, namespace) (acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) and on (name, namespace) `+cpuUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) and on (name, namespace) `+cpuUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(ceil(sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) and on (name, namespace) `+cpuUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((floor(sum by (name, namespace) (acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})-sum by (name, namespace) (acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) * (-1)) > 0, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
	)
}

func VMMemOverestimationTablePanel(datasourceName string, project string) panelgroup.Option {
	overestLink := vmDataLink(project, "acm-rightsizing-vm-overestimation", "VM Detailed View")

	return panelgroup.AddPanel("Memory Overestimation",
		panel.Description("VMs with Memory overestimation - where allocated Memory (Request) exceeds the recommended amount.\nClick VM Name or Namespace to see detailed overestimation view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true, EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name_namespace", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name", Header: "VM Name", HeaderDescription: "Name of the Virtual Machine", EnableSorting: true}, DataLink: overestLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", HeaderDescription: "Kubernetes namespace where the VM is deployed", EnableSorting: true}, DataLink: overestLink},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #1", Header: "Memory Utilization %", HeaderDescription: "Ratio of memory usage to memory request as a percentage",
					EnableSorting: true, Sort: tablePanel.DescSort,
					Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #2", Header: "Memory Usage", HeaderDescription: "Actual memory consumed by the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #3", Header: "Memory Request", HeaderDescription: "Memory requested/allocated for the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #4", Header: "Memory Recommendation", HeaderDescription: "Recommended memory allocation based on usage profile",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #5", Header: "Memory Overestimation", HeaderDescription: "Excess memory allocated beyond recommendation (Request - Recommendation)",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(`label_join((sum by (name, namespace) (acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) and on (name, namespace) `+memOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((sum by (name, namespace) (acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) /1073741824) * 1073741824 and on (name, namespace) `+memOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824) * 1073741824 and on (name, namespace) `+memOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((ceil(sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/ 1073741824)) * 1073741824 and on (name, namespace) `+memOverestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((floor((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) / 1073741824 - (sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) / 1073741824)) * 1073741824 > 0, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
	)
}

func VMMemUnderestimationTablePanel(datasourceName string, project string) panelgroup.Option {
	underestLink := vmDataLink(project, "acm-rightsizing-vm-underestimation", "VM Detailed View")

	return panelgroup.AddPanel("Memory Underestimation",
		panel.Description("VMs with Memory underestimation - where allocated Memory (Request) is below the recommended amount.\nClick VM Name or Namespace to see detailed underestimation view."),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true, EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name_namespace", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name", Header: "VM Name", HeaderDescription: "Name of the Virtual Machine", EnableSorting: true}, DataLink: underestLink},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", HeaderDescription: "Kubernetes namespace where the VM is deployed", EnableSorting: true}, DataLink: underestLink},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #1", Header: "Memory Utilization %", HeaderDescription: "Ratio of memory usage to memory request as a percentage",
					EnableSorting: true, Sort: tablePanel.DescSort,
					Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #2", Header: "Memory Usage", HeaderDescription: "Actual memory consumed by the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #3", Header: "Memory Request", HeaderDescription: "Memory requested/allocated for the VM",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #4", Header: "Memory Recommendation", HeaderDescription: "Recommended memory allocation based on usage profile",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #5", Header: "Memory Underestimation", HeaderDescription: "Deficit of memory below recommendation (Recommendation - Request)",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(`label_join((sum by (name, namespace) (acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"}) / sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) and on (name, namespace) `+memUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((sum by (name, namespace) (acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824) * 1073741824 and on (name, namespace) `+memUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824) * 1073741824 and on (name, namespace) `+memUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join((ceil(sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})/1073741824)) * 1073741824 and on (name, namespace) `+memUnderestCond+`, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(((floor((sum by (name, namespace) (acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) / 1073741824 - (sum by (name, namespace) (acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"})) / 1073741824)) * (-1)) * 1073741824 > 0, "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
	)
}

func VMCPUForecastRecommendationTablePanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM CPU Forecast Recommendation",
		panel.Description("CPU actual usage, forecasted usage, and recommendation per VM over the selected forecast lookback period"),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name_namespace", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name", Header: "VM Name", HeaderDescription: "Name of the Virtual Machine", EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", HeaderDescription: "Kubernetes namespace", EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #1", Header: "Actual CPU Usage", HeaderDescription: "Average CPU usage over the forecast lookback period",
					EnableSorting: true, Sort: tablePanel.DescSort,
					Format: &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #2", Header: "Forecasted CPU", HeaderDescription: "Predicted CPU usage from the forecasting engine",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #3", Header: "CPU Recommendation", HeaderDescription: "Recommended CPU based on forecast with safety margin",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 2},
				}},
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(`label_join(avg_over_time(sum by (name, namespace)(acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$forecast_days:]), "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(max by (name, namespace)(acm_rs:prediction_forecast_vm_cpu{cluster="$cluster", namespace=~"$namespace"}), "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(sum by (name, namespace)(acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"}), "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
	)
}

func VMMemForecastRecommendationTablePanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM Memory Forecast Recommendation",
		panel.Description("Memory actual usage, forecasted usage, and recommendation per VM over the selected forecast lookback period"),
		TableWithLinks(TablePluginSpec{
			ColumnSettings: []ColumnSettingsWithLink{
				{ColumnSettings: tablePanel.ColumnSettings{Name: "timestamp", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name_namespace", Hide: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "name", Header: "VM Name", HeaderDescription: "Name of the Virtual Machine", EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{Name: "namespace", Header: "Namespace", HeaderDescription: "Kubernetes namespace", EnableSorting: true}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #1", Header: "Actual Memory Usage", HeaderDescription: "Average memory usage over the forecast lookback period",
					EnableSorting: true, Sort: tablePanel.DescSort,
					Format: &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #2", Header: "Forecasted Memory", HeaderDescription: "Predicted memory usage from the forecasting engine",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
				{ColumnSettings: tablePanel.ColumnSettings{
					Name: "value #3", Header: "Memory Recommendation", HeaderDescription: "Recommended memory based on forecast with safety margin",
					EnableSorting: true,
					Format:        &commonSdk.Format{Unit: &dashboards.BytesUnit, DecimalPlaces: 2},
				}},
			},
			CellSettings: []tablePanel.CellSettings{
				{Condition: tablePanel.Condition{Kind: tablePanel.MiscConditionKind, Spec: &tablePanel.MiscConditionSpec{Value: tablePanel.NullValue}}, Text: "N/A"},
			},
			Transforms: []commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
			},
			EnableFiltering: true,
		}),
		panel.AddQuery(query.PromQL(`label_join(avg_over_time(sum by (name, namespace)(acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace"})[$forecast_days:]), "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(max by (name, namespace)(acm_rs:prediction_forecast_vm_memory{cluster="$cluster", namespace=~"$namespace"}), "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(`label_join(sum by (name, namespace)(acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace"}), "name_namespace", "-", "name", "namespace")`, dashboards.AddQueryDataSource(datasourceName))),
	)
}

// --- VM Overestimation / Underestimation Detail Dashboard panels ---

var overestDetailRedThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#1A7311"},
		{Value: 0, Color: "#E02F44"},
	},
}

var underestDetailYellowThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#1A7311"},
		{Value: 0, Color: "#E0B400"},
	},
}

var memOverestDetailThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#FF780A"},
		{Value: 0, Color: "#E02F44"},
	},
}

var memUnderestDetailThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#FF780A"},
		{Value: 0, Color: "#E0B400"},
	},
}

var detailGrayThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#808080"},
	},
}

var detailPercentThreshold = &commonSdk.Thresholds{
	Steps: []commonSdk.StepOption{
		{Value: 0, Color: "#808080"},
	},
}

// --- Overestimation Detail Panels ---

func VMCPUOverestimationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Overestimation",
		Description: "Overestimated CPU Cores for the selected VM.\n- CPU cores you can save that are not being utilized.\n- A negative value indicates underestimation.",
		Query:       `max by (cluster, profile, namespace, name)(floor(acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}-` + "\n" + `acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  overestDetailRedThreshold,
	})
}

func VMCPUUsageStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Usage",
		Description: "Actual CPU cores consumed by the selected VM over the aggregation period.\nBased on max_over_time of the CPU usage metric.",
		Query:       `max by (cluster, profile, namespace, name)(acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"})`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func VMCPURequestStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Request",
		Description: "CPU cores requested (allocated) for the selected VM.\nThis is the resource request configured for the VM.",
		Query:       `max by (cluster, profile, namespace, name)(acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"})`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func VMCPUUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Utilization",
		Description: "CPU utilization ratio for the selected VM.\nCalculated as CPU Usage / CPU Request.",
		Query:       `max by (cluster, profile, namespace, name)(acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"} / acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"})`,
		Unit:        &dashboards.PercentDecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  detailPercentThreshold,
	})
}

func VMCPUUtilizationTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Utilization(%) VM Name (Namespace)",
		panel.Description("CPU Utilization percentage over time for the selected VM.\nShows the ratio of actual CPU usage to CPU request as a time series."),
		timeSeriesPanel.Chart(
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
		),
		panel.AddQuery(query.PromQL(
			"(\n"+
				`  acm_rs_vm:namespace:cpu_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}`+"\n"+
				"  / \n"+
				`  acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}`+"\n"+
				")",
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{name}} ({{namespace}})"),
		)),
	)
}

func VMMemoryOverestimationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Overestimation",
		Description: "Overestimated Memory for the selected VM.\n- Memory you can save that is not being utilized.\n- A negative value indicates underestimation.",
		Query:       `max by (cluster, profile, namespace, name)((floor((acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}/ 1073741824) -` + "\n" + `(acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}/ 1073741824))) * 1073741824)`,
		Unit:        &dashboards.BytesUnit,
		FontSize:    40,
		Thresholds:  memOverestDetailThreshold,
	})
}

func VMMemoryUsageStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Usage",
		Description: "Actual memory consumed by the selected VM over the aggregation period.\nBased on max_over_time of the memory usage metric.",
		Query:       `max by (cluster, profile, namespace, name)((acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}/ 1073741824) * 1073741824)`,
		Unit:        &dashboards.BytesUnit,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func VMMemoryRequestStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Request",
		Description: "Memory requested (allocated) for the selected VM.\nThis is the resource request configured for the VM.",
		Query:       `max by (cluster, profile, namespace, name)((acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}/ 1073741824) * 1073741824)`,
		Unit:        &dashboards.BytesUnit,
		FontSize:    40,
		Thresholds:  detailGrayThreshold,
	})
}

func VMMemoryUtilizationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Utilization",
		Description: "Memory utilization ratio for the selected VM.\nCalculated as Memory Usage / Memory Request.",
		Query:       `max by (cluster, profile, namespace, name)(acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"} / acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"})`,
		Unit:        &dashboards.PercentDecimalUnit,
		FontSize:    40,
		Thresholds:  detailPercentThreshold,
	})
}

func VMMemoryUtilizationTimeSeriesPanel(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Utilization(%) - VM Name (Namespace)",
		panel.Description("Memory Utilization percentage over time for the selected VM.\nShows the ratio of actual memory usage to memory request as a time series."),
		timeSeriesPanel.Chart(
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
		),
		panel.AddQuery(query.PromQL(
			"(\n"+
				`  acm_rs_vm:namespace:memory_usage{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}`+"\n"+
				"  / \n"+
				`  acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}`+"\n"+
				")",
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("{{name}} ({{namespace}})"),
		)),
	)
}

// --- Underestimation Detail Panels ---

func VMCPUUnderestimationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "CPU Underestimation",
		Description: "Underestimated CPU Cores for the selected VM.\n- CPU cores that need more resources.\n- A negative value indicates overestimation.",
		Query:       `max by (cluster, profile, namespace, name)(floor(acm_rs_vm:namespace:cpu_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}-` + "\n" + `acm_rs_vm:namespace:cpu_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"})* (-1))`,
		Unit:        &dashboards.DecimalUnit,
		Decimals:    0,
		FontSize:    40,
		Thresholds:  underestDetailYellowThreshold,
	})
}

func VMMemoryUnderestimationStatPanel(datasourceName string) panelgroup.Option {
	return BuildStatPanel(datasourceName, StatPanelConfig{
		Title:       "Memory Underestimation",
		Description: "Underestimated Memory for the selected VM.\n- Memory that needs more resources.\n- A negative value indicates overestimation.",
		Query:       `max by (cluster, profile, namespace, name)((floor((acm_rs_vm:namespace:memory_request{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}/ 1073741824) -` + "\n" + `(acm_rs_vm:namespace:memory_recommendation{cluster="$cluster", profile="$profile", namespace=~"$namespace", name=~"$vm"}/ 1073741824))* (-1)) * 1073741824)`,
		Unit:        &dashboards.BytesUnit,
		FontSize:    40,
		Thresholds:  memUnderestDetailThreshold,
	})
}

// VMBackToMainDashboardPanel creates a "Back to Main Dashboard" markdown panel with a link
func VMBackToMainDashboardPanel(datasourceName string, project string) panelgroup.Option {
	backURL := fmt.Sprintf("/monitoring/v2/dashboards/view?dashboard=acm-rightsizing-openshift-virtualization&project=%s", project)
	return panelgroup.AddPanel("Back to Main Dashboard",
		panel.Description("Back to Main Dashboard"),
		markdownPanel.Markdown(fmt.Sprintf("[Back to Main Dashboard](%s)", backURL)),
		panel.AddLink(backURL,
			link.Name("Back to Main Dashboard"),
			link.Tooltip("Back to Main Dashboard"),
		),
	)
}
