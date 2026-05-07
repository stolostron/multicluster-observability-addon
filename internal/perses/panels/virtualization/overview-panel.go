package virtualization

import (
	"encoding/json"

	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/link"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// mergePluginSpecFields JSON-roundtrips the current plugin spec into a map and
// merges extra fields into it. It must be applied AFTER any typed panel builders
// (e.g. timeSeriesPanel.Chart) — typed builders overwrite Spec with a fresh
// struct, so any fields merged before them will be silently dropped.
// Keep mergePluginSpecFields last in a panel's option list.
func mergePluginSpecFields(extra map[string]any) panel.Option {
	return func(b *panel.Builder) error {
		raw, err := json.Marshal(b.Spec.Plugin.Spec)
		if err != nil {
			return err
		}
		base := map[string]any{}
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &base); err != nil {
				return err
			}
		}
		for k, v := range extra {
			base[k] = v
		}
		b.Spec.Plugin.Spec = base
		return nil
	}
}

// OverviewTotalClusters (0_0)
func OverviewTotalClusters(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Clusters",
		panel.Description("Total clusters with OpenShift Virtualization."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			overviewTotalClusters,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewClustersCriticalHealth (0_1)
func OverviewClustersCriticalHealth(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Clusters in Critical Health",
		panel.Description("Total number of clusters with critical issues that impact the operator health, which are based on alerts and the operator conditions. This means there is a risk of core functionality loss for your clusters."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0, Color: "#f2495c"},
				},
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "value"}),
		panel.AddQuery(query.PromQL(
			overviewClustersCriticalHealth,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewTotalAllocatableNodes (0_2)
func OverviewTotalAllocatableNodes(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Allocatable Nodes",
		panel.Description("Total number of nodes that are available to host virtual machines."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
			statPanel.Format(commonSdk.Format{Unit: &dashboards.DecimalUnit}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none", "metricLabel": ""}),
		panel.AddQuery(query.PromQL(
			overviewTotalAllocatableNodes,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewTotalVMsStat (0_3)
func OverviewTotalVMsStat(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Total VMs",
		panel.Description("Total number of virtual machines."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode":   "none",
			"metricLabel": "",
		}),
		panel.AddLink(vmsByTimeInStatusLinkURL(project, ""),
			link.Name("VMs by Time in Status"),
			link.TargetBlank(true),
		),
		panel.AddQuery(query.PromQL(
			overviewTotalVMs,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewVMsRunning (0_4a)
func OverviewVMsRunning(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Running VMs",
		panel.Description("Number of virtual machines currently running."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddLink(vmsByTimeInStatusLinkURL(project, "running"),
			link.Name("VMs by Time in Status"),
			link.TargetBlank(true),
		),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusStatRunning,
			query.SeriesNameFormat("Running"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewVMsInErrorStat (0_4b) — red when ≥1.
func OverviewVMsInErrorStat(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("VMs in Error",
		panel.Description("Number of virtual machines currently in an error state. Any value above zero requires attention."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
			statPanel.Thresholds(commonSdk.Thresholds{
				DefaultColor: "#808080",
				Steps: []commonSdk.StepOption{
					{Value: 1, Color: "#f2495c"},
				},
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "value"}),
		panel.AddLink(vmsByTimeInStatusLinkURL(project, "error"),
			link.Name("VMs by Time in Status"),
			link.TargetBlank(true),
		),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusStatError,
			query.SeriesNameFormat("Error"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewVMsStopped (0_4c)
func OverviewVMsStopped(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Stopped VMs",
		panel.Description("Number of virtual machines currently stopped."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddLink(vmsByTimeInStatusLinkURL(project, "non_running"),
			link.Name("VMs by Time in Status"),
			link.TargetBlank(true),
		),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusStatStopped,
			query.SeriesNameFormat("Stopped"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewVMsStarting (0_4d)
func OverviewVMsStarting(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Starting VMs",
		panel.Description("Number of virtual machines currently starting up."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddLink(vmsByTimeInStatusLinkURL(project, "starting"),
			link.Name("VMs by Time in Status"),
			link.TargetBlank(true),
		),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusStatStarting,
			query.SeriesNameFormat("Starting"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewVMsMigrating (0_4e)
func OverviewVMsMigrating(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Migrating VMs",
		panel.Description("Number of virtual machines currently being migrated."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddLink(vmsByTimeInStatusLinkURL(project, "migrating"),
			link.Name("VMs by Time in Status"),
			link.TargetBlank(true),
		),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusStatMigrating,
			query.SeriesNameFormat("Migrating"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewClustersWarningHealth (0_5)
func OverviewClustersWarningHealth(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Clusters in Warning Health",
		panel.Description("Total number of clusters with warning level issues that impact the operator health, which are based on alerts and the operator conditions. This means there is a risk of core functionality loss for your clusters."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.ValueFontSize(20),
			statPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0, Color: "#ff9830"},
				},
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "value"}),
		panel.AddQuery(query.PromQL(
			overviewClustersWarningHealth,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewVMsStartedLast7Days (0_6)
func OverviewVMsStartedLast7Days(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Number of VMs Started in the Last 7 Days",
		tablePanel.Table(),
		mergePluginSpecFields(map[string]any{
			"columnSettings": []any{
				map[string]any{
					"name":          "cluster",
					"enableSorting": true,
					"dataLink": map[string]any{
						"openNewTab": true,
						"title":      "Cluster Details",
						"url":        clusterDetailsDashboardLinkURL(project),
					},
				},
				map[string]any{"name": "timestamp", "hide": true},
				map[string]any{"name": "value", "header": "Total VMs Started", "enableSorting": true},
			},
		}),
		panel.AddQuery(query.PromQL(
			overviewVMsStartedLast7Days,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

var overviewTableMergeJoinByCluster = []commonSdk.Transform{
	{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
	{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"cluster"}}},
}

// OverviewClustersByOperatorVersion (0_7)
func OverviewClustersByOperatorVersion(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Clusters by Operator Version",
		tablePanel.Table(
			tablePanel.Transform([]commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"version"}}},
			}),
		),
		mergePluginSpecFields(map[string]any{
			"columnSettings": []any{
				map[string]any{"name": "timestamp", "hide": true},
				map[string]any{"name": "version", "header": "OpenShift Virtualization Operator Version", "enableSorting": true},
				map[string]any{"name": "phase", "hide": true},
				map[string]any{"name": "reason", "hide": true},
				map[string]any{"name": "value", "hide": true},
				map[string]any{"name": "value #1", "header": "# of Clusters", "enableSorting": true, "width": 104},
				map[string]any{"name": "value #2", "header": "% of Total Clusters", "enableSorting": true, "format": map[string]any{"unit": "percent-decimal"}, "width": 136},
				map[string]any{"name": "openshiftVersion", "hide": true},
				map[string]any{"name": "Time 1", "hide": true},
				map[string]any{"name": "Time 2", "hide": true},
			},
		}),
		panel.AddQuery(query.PromQL(
			overviewClustersByOperatorVersionCounts,
			query.SeriesNameFormat("{{version}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewClustersByOperatorVersionPercent,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewClustersByOpenShiftVersion (0_8)
func OverviewClustersByOpenShiftVersion(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Clusters by OpenShift Version",
		tablePanel.Table(
			tablePanel.Transform([]commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"openshiftVersion"}}},
			}),
		),
		mergePluginSpecFields(map[string]any{
			"columnSettings": []any{
				map[string]any{"name": "timestamp", "hide": true},
				map[string]any{"name": "openshiftVersion", "header": "OpenShift Version", "enableSorting": true},
				map[string]any{"name": "value", "hide": true},
				map[string]any{"name": "value #1", "header": "# of Clusters", "enableSorting": true, "width": 104},
				map[string]any{"name": "value #2", "header": "% of Total Clusters", "enableSorting": true, "format": map[string]any{"unit": "percent-decimal"}, "width": 136},
				map[string]any{"name": "Time 1", "hide": true},
				map[string]any{"name": "Time 2", "hide": true},
			},
		}),
		panel.AddQuery(query.PromQL(
			overviewClustersByOpenShiftVersionCounts,
			query.SeriesNameFormat("{{openshiftVersion}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewClustersByOpenShiftVersionPercent,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewOperatorHealthByCluster (1_0)
func OverviewOperatorHealthByCluster(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Operator Health by Cluster",
		tablePanel.Table(
			tablePanel.Transform(overviewTableMergeJoinByCluster),
		),
		mergePluginSpecFields(map[string]any{
			"columnSettings": []any{
				map[string]any{
					"name": "value #1", "header": "Operator Health", "enableSorting": true,
					"cellSettings": []any{
						map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "0"}}, "text": "Healthy", "backgroundColor": "#73BF69"},
						map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "1"}}, "text": "Warning", "backgroundColor": "#ff9830"},
						map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "2"}}, "text": "Critical", "backgroundColor": "#f2495c"},
					},
				},
				map[string]any{
					"name": "cluster", "header": "Cluster", "enableSorting": true,
					"dataLink": map[string]any{
						"openNewTab": true,
						"title":      "Cluster Details",
						"url":        clusterDetailsDashboardLinkURL(project),
					},
				},
				map[string]any{"name": "openshiftVersion", "hide": true},
				map[string]any{
					"name": "value #5", "header": "Operator Conditions Health", "enableSorting": true,
					"cellSettings": []any{
						map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "0"}}, "text": "Healthy", "textColor": "#73BF69"},
						map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "1"}}, "text": "Warning", "textColor": "#ff9830"},
						map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "2"}}, "text": "Error", "textColor": "#f2495c"},
					},
				},
				map[string]any{"name": "value #2", "header": "Alerts with Critical Impact", "enableSorting": true},
				map[string]any{"name": "value #3", "header": "Alerts with Warning Impact", "enableSorting": true},
				map[string]any{"name": "value #4", "header": "Number of Running VMs", "enableSorting": true},
				map[string]any{"name": "Time 1", "header": "", "hide": true},
				map[string]any{"name": "Time 2", "header": "", "hide": true},
				map[string]any{"name": "Time 3", "hide": true},
				map[string]any{"name": "Time 4", "hide": true},
				map[string]any{"name": "Time 5", "hide": true},
				map[string]any{"name": "Time 6", "hide": true},
				map[string]any{"name": "timestamp", "hide": true},
				map[string]any{"name": "__name__", "hide": true},
				map[string]any{"name": "clusterID", "hide": true},
				map[string]any{"name": "clusterType", "hide": true},
				map[string]any{"name": "endpoint", "hide": true},
				map[string]any{"name": "instance", "hide": true},
				map[string]any{"name": "job", "hide": true},
				map[string]any{"name": "namespace", "hide": true},
				map[string]any{"name": "pod", "hide": true},
				map[string]any{"name": "receive", "hide": true},
				map[string]any{"name": "service", "hide": true},
				map[string]any{"name": "tenant_id", "hide": true},
				map[string]any{"name": "value", "hide": true},
				map[string]any{"name": "version", "hide": true},
			},
		}),
		panel.AddQuery(query.PromQL(
			overviewOperatorHealthByClusterStatus,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewOperatorHealthByClusterCriticalAlerts,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewOperatorHealthByClusterWarningAlerts,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewOperatorHealthByClusterRunningVMs,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewOperatorHealthByClusterHCO,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func overviewPieChartPlugin() panel.Option {
	return func(b *panel.Builder) error {
		b.Spec.Plugin.Kind = "PieChart"
		b.Spec.Plugin.Spec = map[string]any{
			"calculation": "last-number",
			"format":      map[string]any{"unit": "decimal"},
			"legend":      map[string]any{"mode": "table", "position": "right", "values": []string{"abs", "relative"}},
			"radius":      50,
		}
		return nil
	}
}

// OverviewRunningVMsByOS (2_0)
func OverviewRunningVMsByOS(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Running VMs by OS",
		overviewPieChartPlugin(),
		panel.AddQuery(query.PromQL(
			overviewRunningVMsByOSKnown,
			query.SeriesNameFormat("{{os}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewRunningVMsByOSUnknown,
			query.SeriesNameFormat("{{os}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func overviewTSChartLegendLast() timeSeriesPanel.Legend {
	return timeSeriesPanel.Legend{
		Position: timeSeriesPanel.RightPosition,
		Mode:     timeSeriesPanel.TableMode,
		Values:   []commonSdk.Calculation{commonSdk.LastNumberCalculation},
	}
}

func overviewTSChartLegendLastMax() timeSeriesPanel.Legend {
	return timeSeriesPanel.Legend{
		Position: timeSeriesPanel.RightPosition,
		Mode:     timeSeriesPanel.TableMode,
		Values:   []commonSdk.Calculation{commonSdk.LastNumberCalculation, commonSdk.MaxCalculation},
	}
}

// OverviewRunningVMsByClusterTop20 (2_1)
func OverviewRunningVMsByClusterTop20(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Running VMs by Cluster - Top 20",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLast()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &dashboards.DecimalUnit},
				Min:    0,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    2,
				AreaOpacity:  0.11,
				ConnectNulls: true,
			}),
		),
		mergePluginSpecFields(map[string]any{
			"visual": map[string]any{
				"display": "line", "lineWidth": 2, "areaOpacity": 0.11, "connectNulls": true, "lineStyle": "solid",
			},
		}),
		panel.AddQuery(query.PromQL(
			overviewRunningVMsByClusterTop20,
			query.SeriesNameFormat("{{cluster}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewVMsByStatusTimeSeries (2_2)
func OverviewVMsByStatusTimeSeries(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Virtual Machines by Status",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLast()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &dashboards.DecimalUnit},
				Min:    0,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0,
				ConnectNulls: false,
			}),
			timeSeriesPanel.WithQuerySettings([]timeSeriesPanel.QuerySettingsItem{
				{QueryIndex: 3, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#f2495c"},
				{QueryIndex: 4, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#1f60c4"},
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusTSRunning,
			query.SeriesNameFormat("running"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusTSStarting,
			query.SeriesNameFormat("starting"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusTSMigrating,
			query.SeriesNameFormat("migrating"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusTSError,
			query.SeriesNameFormat("error"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			overviewVMsByStatusTSStopped,
			query.SeriesNameFormat("stopped"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewRunningVMsByNodeTop20 (2_3)
func OverviewRunningVMsByNodeTop20(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Running VMs by Node - Top 20",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLast()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit:          &dashboards.DecimalUnit,
					DecimalPlaces: 0,
				},
				Min: 0,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				Stack:        timeSeriesPanel.AllStack,
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewRunningVMsByNodeTop20,
			query.SeriesNameFormat("{{cluster}} | {{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewCPUUsageByCluster (3_0)
func OverviewCPUUsageByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage by Cluster",
		panel.Description("This panel displays the top 20 clusters based on their VMs CPU usage over the past 10 minutes. CPU usage is calculated as the total CPU time consumed by VMs. This provides insight into clusters with the highest CPU demand and helps identify potential resource bottlenecks."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &dashboards.SecondsUnit},
				Min:    0,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.1,
				ConnectNulls: false,
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewCPUUsageByClusterTop20,
			query.SeriesNameFormat("{{cluster}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewCPUUsagePercentByCluster (3_1)
func OverviewCPUUsagePercentByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Clusters by CPU Usage (%)",
		panel.Description("This panel displays the top 20 clusters based on their CPU usage percentage for clusters running virtual machines (VMs) over the past 10 minutes. CPU usage is calculated as the total CPU time consumed by VMs divided by the total CPU capacity of all nodes in the cluster. This provides insight into how much of the cluster's overall CPU capacity is consumed by VM workloads."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.1,
				ConnectNulls: false,
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewCPUUsagePercentByClusterTop20,
			query.SeriesNameFormat("{{cluster}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewMemoryUsageByCluster (4_0)
func OverviewMemoryUsageByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage by Cluster",
		panel.Description("Top 20 Clusters based on the VMs memory usage in the clusters"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &dashboards.BytesUnit},
				Min:    0,
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.3,
				ConnectNulls: false,
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewMemoryUsageByClusterTop20,
			query.SeriesNameFormat("{{cluster}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewMemoryUsagePercentByCluster (4_1)
func OverviewMemoryUsagePercentByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage by Cluster (%)",
		panel.Description("Top 20 clusters based on VM memory usage as a percentage of total physical memory across all nodes in the cluster."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.3,
				ConnectNulls: false,
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewMemoryUsagePercentByClusterTop20,
			query.SeriesNameFormat("{{cluster}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewNetworkReceivedByCluster (5_0)
func OverviewNetworkReceivedByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Clusters by Network Received Bytes",
		panel.Description("This panel displays the top 20 clusters by network received bytes per second over the past 10 minutes. The query measures the rate of incoming network traffic, helping identify clusters with the highest network activity. Use this information to monitor and optimize resource usage for workloads with significant data reception."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &decBytesPerSecUnit},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.1,
				ConnectNulls: true,
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewNetworkReceivedBytesByClusterTop20,
			query.SeriesNameFormat("{{cluster}} - Receive"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewNetworkTransmittedByCluster (5_1)
func OverviewNetworkTransmittedByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Clusters by Network Transmitted Bytes",
		panel.Description("This panel displays the top 20 clusters by network transmitted bytes per second over the past 10 minutes. The query measures the rate of outgoing network traffic, helping identify clusters with the highest network activity. Use this information to monitor and optimize resource usage for workloads with significant data transmission."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &decBytesPerSecUnit},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.1,
				ConnectNulls: true,
			}),
		),
		panel.AddQuery(query.PromQL(
			overviewNetworkTransmittedBytesByClusterTop20,
			query.SeriesNameFormat("{{cluster}} - Transmit"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewStorageTrafficByCluster (6_0)
func OverviewStorageTrafficByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Storage Traffic by Cluster",
		panel.Description("This panel displays the top 20 clusters based on their combined storage traffic (read + write) over the past 10 minutes."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &decBytesPerSecUnit},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.1,
				ConnectNulls: false,
			}),
		),
		mergePluginSpecFields(map[string]any{
			"visual": map[string]any{
				"display": "line", "lineStyle": "solid", "lineWidth": 1, "areaOpacity": 0.1, "connectNulls": false,
			},
		}),
		panel.AddQuery(query.PromQL(
			overviewStorageTrafficByClusterTop20,
			query.SeriesNameFormat("{{cluster}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// OverviewStorageIOPsByCluster (6_1)
func OverviewStorageIOPsByCluster(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Storage IOPS by Cluster",
		panel.Description("This panel displays the top 20 clusters based on their combined storage IOPS (read + write) over the past 10 minutes."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(overviewTSChartLegendLastMax()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &opsPerSecUnit, ShortValues: true},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				LineWidth:    1,
				AreaOpacity:  0.1,
				ConnectNulls: false,
			}),
		),
		mergePluginSpecFields(map[string]any{
			"visual": map[string]any{
				"display": "line", "lineStyle": "solid", "lineWidth": 1, "areaOpacity": 0.1, "connectNulls": false,
			},
		}),
		panel.AddQuery(query.PromQL(
			overviewStorageIOPsByClusterTop20,
			query.SeriesNameFormat("{{cluster}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
