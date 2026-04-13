package virtualization

import (
	"encoding/json"

	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	apicommon "github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

var singleClusterOperatorHealthMappings = []any{
	map[string]any{
		"kind": "Value",
		"spec": map[string]any{
			"value": "0",
			"result": map[string]any{
				"color": "#73bf69",
				"value": "OK",
			},
		},
	},
	map[string]any{
		"kind": "Value",
		"spec": map[string]any{
			"value": "1",
			"result": map[string]any{
				"color": "#fade2a",
				"value": "Warning",
			},
		},
	},
	map[string]any{
		"kind": "Value",
		"spec": map[string]any{
			"value": "2",
			"result": map[string]any{
				"color": "#f2495c",
				"value": "Critical",
			},
		},
	},
}

func singleClusterRecentVMsColumnSettings(project string) []any {
	return []any{
		map[string]any{
			"name":          "name",
			"header":        "VM Name",
			"enableSorting": true,
			"dataLink":      vmDetailsDashboardLinkByField(project),
		},
		map[string]any{"name": "timestamp", "hide": true},
		map[string]any{"name": "value", "header": "Uptime ", "enableSorting": true},
		map[string]any{"name": "namespace", "header": "Namespace", "enableSorting": true},
	}
}

func mergeTimeSeriesVisual(patch map[string]any) panel.Option {
	return func(b *panel.Builder) error {
		data, err := json.Marshal(b.Spec.Plugin.Spec)
		if err != nil {
			return err
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		existing, _ := m["visual"].(map[string]any)
		if existing == nil {
			existing = map[string]any{}
		}
		for k, v := range patch {
			existing[k] = v
		}
		m["visual"] = existing
		b.Spec.Plugin.Spec = m
		return nil
	}
}

func singleClusterTSLegend(values ...commonSdk.Calculation) timeSeriesPanel.Legend {
	return timeSeriesPanel.Legend{
		Position: timeSeriesPanel.RightPosition,
		Mode:     timeSeriesPanel.TableMode,
		Values:   values,
	}
}

func singleClusterTSVisual(connectNulls bool) timeSeriesPanel.Visual {
	return timeSeriesPanel.Visual{
		Display:      timeSeriesPanel.LineDisplay,
		LineWidth:    1,
		AreaOpacity:  0.1,
		ConnectNulls: connectNulls,
	}
}

// SingleClusterClusterName shows the selected cluster label on the hyperconverged health metric.
func SingleClusterClusterName(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster Name",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode":   "none",
			"metricLabel": "cluster",
		}),
		panel.AddQuery(query.PromQL(
			singleClusterClusterName,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterOpenshiftVirtVersion shows the virt operator CSV version.
func SingleClusterOpenshiftVirtVersion(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("OpenShift Virt Version",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode":   "none",
			"metricLabel": "version",
		}),
		panel.AddQuery(query.PromQL(
			singleClusterOpenshiftVirtVersion,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterTotalNodes shows allocatable nodes exposing kubevirt resources.
func SingleClusterTotalNodes(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Nodes",
		panel.Description("Total node count in OpenShift Virtualization clusters"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{Unit: &dashboards.DecimalUnit}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			singleClusterTotalNodes,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterTotalVMs counts the total number of distinct VMs in the selected cluster.
func SingleClusterTotalVMs(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total VMs",
		panel.Description("Total number of virtual machines in the selected cluster."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{Unit: &dashboards.DecimalUnit}),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode":   "none",
			"metricLabel": "",
		}),
		panel.AddQuery(query.PromQL(
			singleClusterTotalVMs,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVirtualMachinesByStatus shows running / stopped / error / starting / migrating counts.
func SingleClusterVirtualMachinesByStatus(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Virtual Machines by Status",
		panel.Description("Breakdown of virtual machines by their current status: Running, Stopped, Error, Starting, and Migrating."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMStatusRunning,
			query.SeriesNameFormat("Running"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMStatusStopped,
			query.SeriesNameFormat("Stopped"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMStatusError,
			query.SeriesNameFormat("Error"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMStatusStarting,
			query.SeriesNameFormat("Starting"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMStatusMigrating,
			query.SeriesNameFormat("Migrating"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterProvider shows cloud provider from ACM managed cluster labels.
func SingleClusterProvider(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Provider",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode":   "none",
			"metricLabel": "cloud",
		}),
		panel.AddQuery(query.PromQL(
			singleClusterProviderByCloud,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterOpenshiftVersion shows OpenShift version from CSV metrics.
func SingleClusterOpenshiftVersion(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("OpenShift Version",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode":   "none",
			"metricLabel": "version",
		}),
		panel.AddQuery(query.PromQL(
			singleClusterOpenshiftVersion,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterOperatorStatus maps hyperconverged operator health codes to OK / Warning / Critical.
func SingleClusterOperatorStatus(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Operator Status",
		panel.Description("Inspect the Operator Conditions and the Alerts tab for additional details"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode": "value",
			"mappings":  singleClusterOperatorHealthMappings,
		}),
		panel.AddQuery(query.PromQL(
			singleClusterOperatorHealthStatus,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterOperatorConditions maps HCO system health codes to OK / Warning / Critical.
func SingleClusterOperatorConditions(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Operator Conditions",
		panel.Description("Status of HCO conditions - Check the HCO Conditions in the cluster"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode": "value",
			"mappings":  singleClusterOperatorHealthMappings,
		}),
		panel.AddQuery(query.PromQL(
			singleClusterHCOSystemHealthStatus,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterRunningVMsByOS is a pie chart of running guests grouped by OS name.
func SingleClusterRunningVMsByOS(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Running VMs by OS",
		panel.Plugin(apicommon.Plugin{
			Kind: "PieChart",
			Spec: map[string]any{
				"calculation": "last-number",
				"format":      map[string]any{"unit": "decimal"},
				"legend":      map[string]any{"mode": "table", "position": "right", "values": []string{"abs", "relative"}},
				"radius":      50,
			},
		}),
		panel.AddQuery(query.PromQL(
			singleClusterRunningVMsByOS1,
			query.SeriesNameFormat("{{os}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterRunningVMsByOS2,
			query.SeriesNameFormat("{{os}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterRecentVMsStarted lists VMs that started most recently with a drill-down link.
func SingleClusterRecentVMsStarted(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Recent VMs Started",
		tablePanel.Table(),
		mergePluginSpecFields(map[string]any{
			"columnSettings": singleClusterRecentVMsColumnSettings(project),
		}),
		panel.AddQuery(query.PromQL(
			singleClusterRecentVMsStarted,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsRunningByNode plots running VMI count per node (top series).
func SingleClusterVMsRunningByNode(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VMs Running by Node",
		panel.Description("Top 20 nodes"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit:          &dashboards.DecimalUnit,
					DecimalPlaces: 0,
				},
				Min: 0,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterVMsRunningByNode,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsByStatus plots aggregate VM counts by lifecycle phase.
func SingleClusterVMsByStatus(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VMs by Status",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation)),
			timeSeriesPanel.WithQuerySettings([]timeSeriesPanel.QuerySettingsItem{
				{QueryIndex: 0, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#5794f2"},
				{QueryIndex: 1, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#56a64b"},
				{QueryIndex: 2, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#e0b400"},
				{QueryIndex: 3, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#f2495c"},
				{QueryIndex: 4, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#1f60c4"},
			}),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.DecimalUnit,
				},
				Min: 0,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterVMsByStatusStarting,
			query.SeriesNameFormat("starting"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMsByStatusRunning,
			query.SeriesNameFormat("running"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMsByStatusMigrating,
			query.SeriesNameFormat("migrating"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMsByStatusError,
			query.SeriesNameFormat("error"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			singleClusterVMsByStatusStopped,
			query.SeriesNameFormat("stopped"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesCPUUtilization shows node CPU utilization rate (recording rule).
func SingleClusterNodesCPUUtilization(datasourceName string) panelgroup.Option {
	u := string(commonSdk.SecondsUnit)
	return panelgroup.AddPanel("Nodes by CPU Utilization",
		panel.Description("This panel displays the CPU Utilization Percentage for the top 20 nodes in the cluster, that are running virtual machines, over the past 1 minute. Node CPU Utilization reflects the rate of CPU usage relative to the node's total capacity, providing insight into the most resource-intensive nodes."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
				Min: 0,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterNodesCPUUtilizationRate1m,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsTotalCPUUsage shows CPU seconds rate per VM.
func SingleClusterVMsTotalCPUUsage(datasourceName string) panelgroup.Option {
	u := string(commonSdk.SecondsUnit)
	return panelgroup.AddPanel("VMs by Total CPU Usage",
		panel.Description("This panel displays the total CPU Usage for the top 20 VMs over the past 10 minutes. CPU Usage represents the rate of CPU time consumed by each VM, providing insight into the most resource-intensive workloads in the cluster during the recent period."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMsTotalCPUUsage,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesCPUUsagePercent shows non-idle CPU share per node.
func SingleClusterNodesCPUUsagePercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Nodes by CPU Usage (%)",
		panel.Description("This panel shows the CPU Usage Percentage for the top 20 nodes in the cluster, that are running virtual machines, over the past 10 minutes. CPU Usage Percentage is calculated as the proportion of active CPU time (user, system, I/O wait) relative to the total available CPU time. It helps identify nodes with the highest workload demand and potential resource contention."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterNodesCPUUsagePercent,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsCPUUsagePercent shows guest CPU usage vs requested capacity.
func SingleClusterVMsCPUUsagePercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VMs by CPU Usage (%)",
		panel.Description("This panel displays the CPU Usage Percentage for the top 20 VMs over the past 10 minutes. CPU Usage Percentage measures how much of the allocated CPU resources (cores, sockets, and threads) each VM is actively utilizing. It helps identify VMs with high or low CPU utilization relative to their allocated capacity."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMsCPUUsagePercent,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesCPUStealPercent shows steal time relative to total CPU.
func SingleClusterNodesCPUStealPercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Nodes by CPU Steal Time (%)",
		panel.Description("This panel shows the CPU steal time percentage over time for the top 20 nodes in the cluster, that are running virtual machines, over the past 10 minutes.\nIt can indicate resource contention on virtualized nodes. High values suggest VMs are competing for CPU resources, potentially impacting performance."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterNodesCPUStealPercent,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsCPUReadyPercent shows vCPU delay vs requested CPU topology.
func SingleClusterVMsCPUReadyPercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VMs by CPU Ready (%)",
		panel.Description("This panel shows the CPU Ready Percentage for the top 20 VMs over the past 10 minutes. CPU Ready indicates the proportion of time a VM's virtual CPUs were ready to execute but had to wait for physical CPU resources due to contention."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
				Min: 0,
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMsCPUReadyPercent,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesMemoryUsage shows estimated memory used in bytes per node.
func SingleClusterNodesMemoryUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Nodes by Memory Usage",
		panel.Description("This panel shows the Memory Usage for the top 20 nodes in the cluster that are running virtual machines. Memory usage is calculated as the proportion of memory utilized relative to the total available memory on each node. This helps identify resource-intensive nodes hosting virtual machines, enabling proactive resource management."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit:          &dashboards.BytesUnit,
					DecimalPlaces: 1,
				},
				Min: 0,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterNodesMemoryUsageBytes,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsMemoryUsage shows guest memory used (available - unused - cached).
func SingleClusterVMsMemoryUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VMs by Memory Usage",
		panel.Description("This panel displays the memory usage for the top 20 virtual machines (VMs) in the cluster. Memory usage is calculated as the allocated memory minus unused and cached memory. This provides insight into the most memory-intensive VMs, helping identify resource-heavy workloads and potential optimizations. The data is based on the most recent metrics collected."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMsMemoryUsageBytes,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesMemoryUsagePercent shows node memory utilization ratio.
func SingleClusterNodesMemoryUsagePercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Nodes by Memory Usage (%)",
		panel.Description("This panel shows the Memory Usage Percentage for the top 20 nodes in the cluster that are running virtual machines. Memory usage is calculated as the proportion of memory utilized relative to the total available memory on each node. This helps identify resource-intensive nodes hosting virtual machines, enabling proactive resource management."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
				Min: 0,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterNodesMemoryUsagePercent,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsMemoryUsagePercent shows guest memory used vs requested memory.
func SingleClusterVMsMemoryUsagePercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VMs by Memory Usage (%)",
		panel.Description("This panel displays the memory usage for the top 20 virtual machines (VMs) in the cluster. Memory usage is calculated as the allocated memory minus unused and cached memory. This provides insight into the most memory-intensive VMs, helping identify resource-heavy workloads and potential optimizations. The data is based on the most recent metrics collected."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMsMemoryUsagePercent,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesNetworkReceived shows receive throughput per node.
func SingleClusterNodesNetworkReceived(datasourceName string) panelgroup.Option {
	u := string(commonSdk.BytesDecPerSecondsUnit)
	return panelgroup.AddPanel("Nodes by Network Received Bytes",
		panel.Description("This panel displays the top 20 nodes running virtual machines (VMs) based on their total network received bytes over the past hour. It includes only nodes hosting VMs and excludes traffic from the loopback interface. This provides insight into the nodes with the highest incoming network traffic, helping identify workloads with significant data reception requirements."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(true)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterNodesNetworkReceivedRate,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsNetworkReceived shows receive throughput per VM.
func SingleClusterVMsNetworkReceived(datasourceName string) panelgroup.Option {
	u := string(commonSdk.BytesDecPerSecondsUnit)
	return panelgroup.AddPanel("VMs by Network Received Bytes",
		panel.Description("This panel displays the top 20 virtual machines (VMs) in the cluster by network received bytes per second over the past 10 minutes. The query measures the rate of incoming network traffic, helping identify VMs with the highest network activity. Use this information to monitor and optimize resource usage for workloads with significant data reception."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(true)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterVMsNetworkReceivedRate,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesNetworkTransmitted shows transmit throughput per node.
func SingleClusterNodesNetworkTransmitted(datasourceName string) panelgroup.Option {
	u := string(commonSdk.BytesDecPerSecondsUnit)
	return panelgroup.AddPanel("Nodes by Network Transmitted Bytes",
		panel.Description("This panel displays the top 20 nodes running virtual machines (VMs) based on their total network transmitted bytes over the past hour. It includes only nodes hosting VMs and excludes traffic from the loopback interface. This provides insight into the nodes with the highest outgoing network traffic, helping identify workloads with significant data transmission requirements."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(true)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterNodesNetworkTransmitRate,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsNetworkTransmitted shows transmit throughput per VM.
func SingleClusterVMsNetworkTransmitted(datasourceName string) panelgroup.Option {
	u := string(commonSdk.BytesDecPerSecondsUnit)
	return panelgroup.AddPanel("VMs by Network Transmitted Bytes",
		panel.Description("This panel displays the top 20 virtual machines (VMs) in the cluster by network transmitted bytes per second over the past 10 minutes. The query measures the rate of outgoing network traffic, helping identify VMs with the highest network activity. Use this information to monitor and optimize resource usage for workloads with significant data transmission."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(true)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterVMsNetworkTransmitRate,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesVMStorageIOPS aggregates read+write IOPS per node.
func SingleClusterNodesVMStorageIOPS(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Nodes by VM Storage IOPS",
		panel.Description("This panel displays the top 20 nodes running virtual machines (VMs) based on their combined storage IOPS (read + write) over the past 10 minutes. The metric aggregates the total input/output operations per second for all VMs on each node, helping identify nodes with the highest storage activity and potential bottlenecks."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &opsPerSecUnit, ShortValues: true},
			}),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterNodesStorageIOPS,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsStorageIOPS aggregates read+write IOPS per VM.
func SingleClusterVMsStorageIOPS(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VMs by Storage IOPS",
		panel.Description("This panel displays the top 20 virtual machines (VMs) based on their combined storage IOPS (read + write) over the past 10 minutes. The metric aggregates the total input/output operations per second for each VM, helping identify workloads with the highest storage activity and potential hotspots for performance optimization."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &opsPerSecUnit, ShortValues: true},
			}),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMsStorageIOPS,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterNodesVMStorageTraffic aggregates read+write bytes per second per node.
func SingleClusterNodesVMStorageTraffic(datasourceName string) panelgroup.Option {
	u := string(commonSdk.BytesDecPerSecondsUnit)
	return panelgroup.AddPanel("Nodes by VM Storage Traffic",
		panel.Description("This panel displays the top 20 nodes running virtual machines (VMs) based on their combined storage traffic (read + write) over the past 10 minutes. The metric aggregates the total input/output operations for all VMs on each node, helping identify nodes with the highest storage activity and potential bottlenecks."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterNodesStorageTraffic,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterVMsStorageTraffic aggregates read+write bytes per second per VM.
func SingleClusterVMsStorageTraffic(datasourceName string) panelgroup.Option {
	u := string(commonSdk.BytesDecPerSecondsUnit)
	return panelgroup.AddPanel("VMs by Storage Traffic",
		panel.Description("This panel displays the top 20 virtual machines (VMs) based on their combined storage traffic (read + write) over the past 10 minutes. The metric aggregates the total input/output operations for the last 10 minutes for each VM, helping identify workloads with the highest storage activity and potential hotspots for performance optimization."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleClusterTSLegend(commonSdk.LastNumberCalculation, commonSdk.MaxCalculation)),
			timeSeriesPanel.WithVisual(singleClusterTSVisual(false)),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &u,
				},
			}),
		),
		mergeTimeSeriesVisual(map[string]any{"lineStyle": "solid"}),
		panel.AddQuery(query.PromQL(
			singleClusterVMsStorageTraffic,
			query.SeriesNameFormat("{{namespace}} | {{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterCriticalSeverityAlerts counts firing critical kubevirt operator alerts.
func SingleClusterCriticalSeverityAlerts(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Critical Severity Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: critical."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode": "value",
			"thresholds": map[string]any{
				"steps": []map[string]any{
					{"value": 0, "color": "#f2495c"},
				},
			},
		}),
		panel.AddQuery(query.PromQL(
			singleClusterAlertsCritical,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterWarningSeverityAlerts counts firing warning alerts filtered by health impact.
func SingleClusterWarningSeverityAlerts(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Warning Severity Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: warning."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode": "value",
			"thresholds": map[string]any{
				"steps": []map[string]any{
					{"value": 0, "color": "#ff9830"},
				},
			},
		}),
		panel.AddQuery(query.PromQL(
			singleClusterAlertsWarning,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterInfoSeverityAlerts counts firing info-level kubevirt alerts.
func SingleClusterInfoSeverityAlerts(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Info Severity Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: info."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"colorMode": "value",
			"thresholds": map[string]any{
				"steps": []map[string]any{
					{"value": 0, "color": "#6ed0e0"},
				},
			},
		}),
		panel.AddQuery(query.PromQL(
			singleClusterAlertsInfo,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterOperatorHealthImpactAlertsTable lists firing alerts with operator health impact (excludes info).
func SingleClusterOperatorHealthImpactAlertsTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Operator Health Impact Alerts",
		panel.Plugin(apicommon.Plugin{
			Kind: "Table",
			Spec: map[string]any{
				"columnSettings": []any{
					map[string]any{"name": "alertname", "header": "Name", "enableSorting": true},
					map[string]any{"name": "operator_health_impact", "header": "Operator Health Impact", "enableSorting": true},
					map[string]any{
						"name": "severity", "header": "Severity", "enableSorting": true,
						"cellSettings": []any{
							map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "critical"}}, "textColor": "#f2495c"},
							map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "warning"}}, "textColor": "#ff9830"},
						},
					},
					map[string]any{"name": "alertstate", "hide": true},
					map[string]any{"name": "timestamp", "hide": true},
					map[string]any{"name": "value", "hide": true},
					map[string]any{"name": "cluster", "hide": true},
					map[string]any{"name": "name", "hide": true},
					map[string]any{"name": "namespace", "hide": true},
				},
			},
		}),
		panel.AddQuery(query.PromQL(
			singleClusterOperatorHealthImpactAlerts,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterOperatorCSVIssuesTable shows abnormal hyperconverged CSV rows by version/phase/reason.
func SingleClusterOperatorCSVIssuesTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Operator CSV Issues",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "version", Header: "OpenShift Virtualization Operator Version", EnableSorting: true},
				{Name: "phase", Header: "Phase", EnableSorting: true},
				{Name: "reason", Header: "Reason", EnableSorting: true},
				{Name: "value", Hide: true},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleClusterOperatorCSVAbnormal,
			query.SeriesNameFormat("{{version}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleClusterAllAlertsTable lists all firing kubevirt alerts with severity and impact labels.
func SingleClusterAllAlertsTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("All Alerts",
		panel.Plugin(apicommon.Plugin{
			Kind: "Table",
			Spec: map[string]any{
				"columnSettings": []any{
					map[string]any{"name": "alertname", "header": "Name", "enableSorting": true},
					map[string]any{"name": "namespace", "header": "Namespace", "enableSorting": true},
					map[string]any{"name": "operator_health_impact", "header": "Operator Health Impact", "enableSorting": true},
					map[string]any{
						"name": "severity", "header": "Severity", "enableSorting": true,
						"cellSettings": []any{
							map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "critical"}}, "textColor": "#f2495c"},
							map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "warning"}}, "textColor": "#ff9830"},
							map[string]any{"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "info"}}, "textColor": "#6ed0e0"},
						},
					},
					map[string]any{"name": "name", "header": "VM Name", "enableSorting": true},
					map[string]any{"name": "value", "header": "Total Alerts", "enableSorting": true},
					map[string]any{"name": "timestamp", "hide": true},
					map[string]any{"name": "alertstate", "hide": true},
					map[string]any{"name": "cluster", "hide": true},
				},
			},
		}),
		panel.AddQuery(query.PromQL(
			singleClusterAllAlerts,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
