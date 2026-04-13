package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/link"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	gaugePanel "github.com/perses/plugins/gaugechart/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func memoryTSBottomLegend() timeSeriesPanel.Legend {
	return timeSeriesPanel.Legend{
		Position: timeSeriesPanel.BottomPosition,
		Mode:     timeSeriesPanel.ListMode,
	}
}

// NodeMemoryClusterUtilizationNow is a gauge for aggregated physical memory utilization.
func NodeMemoryClusterUtilizationNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster Utilization",
		panel.Description("This panel is providing the aggregated memory utilization of all nodes of the cluster. This value is more helpful, the more balanced the memory utilization of all nodes is.\nKeep it green."),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			gaugePanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.7},
					{Value: 0.8, Color: "#FFB249"},
					{Value: 0.9, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationNowPhysicalRatio,
			query.SeriesNameFormat("Physical Memory utilization"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryClusterVirtualCommittedNow is a gauge for virtual memory commitment vs allocatable.
func NodeMemoryClusterVirtualCommittedNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster Virtual Committed",
		panel.Description("This panel shows the amount of committed virtual memory as a percentage of the allocatable physical memory. Overcommit occurs whenever the values goes beyond 100%."),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			gaugePanel.Max(2),
			gaugePanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 1.2},
					{Value: 1.5, Color: "#EA4747"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterVirtualCommittedNowRatio,
			query.SeriesNameFormat("Virtual Memory commitment"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryVmVirtualCommittedNow is a gauge for average VM overcommit ratio.
func NodeMemoryVmVirtualCommittedNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM Virtual Committed",
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			gaugePanel.Max(2),
			gaugePanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 1.5},
					{Value: 1.75, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryVMVirtualCommittedNowAvgOvercommit,
			query.SeriesNameFormat("Average VM overcommit ratio"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeUtilizationMinNow is a gauge for the lowest node memory utilization.
func NodeMemoryNodeUtilizationMinNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Node Utilization - min",
		panel.Description("This panel is showing the node memory utilization of the node with the lowest memory utilization in the cluster."),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			gaugePanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.7},
					{Value: 0.8},
					{Value: 0.9, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeUtilizationMinNow,
			query.SeriesNameFormat("Min"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeUtilizationMaxNow is a gauge for the highest node memory utilization.
func NodeMemoryNodeUtilizationMaxNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Node Utilization - max",
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			gaugePanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.7},
					{Value: 0.8},
					{Value: 0.9, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeUtilizationMaxNow,
			query.SeriesNameFormat("Max"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodePressureMaxNow is a gauge for peak memory PSI (pressure stall information).
func NodeMemoryNodePressureMaxNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Node PSI - max",
		panel.Description("Memory PSI (Pressure Stall Information)"),
		panel.AddLink("https://access.redhat.com/solutions/6987181",
			link.Name("Red Hat Customer Portal Article"),
			link.TargetBlank(true),
		),
		panel.AddLink("https://www.kernel.org/doc/html/latest/accounting/psi.html",
			link.Name("Upstream Linux Kernel PSI Documentation"),
		),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{
				Unit:          &dashboards.DecimalUnit,
				DecimalPlaces: 3,
			}),
			gaugePanel.Max(1),
			gaugePanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.1},
					{Value: 0.25},
					{Value: 0.5, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodePressureMaxNow,
			query.SeriesNameFormat("{{instance}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeRequestsMinmaxNow shows min/max pod memory requests as a fraction of allocatable.
func NodeMemoryNodeRequestsMinmaxNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Node Requests - min/max",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.8, Color: "#FF9F1C"},
					{Value: 0.9, Color: "#EA4747"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeRequestsMinNow,
			query.SeriesNameFormat("Min"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeRequestsMaxNow,
			query.SeriesNameFormat("Max"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryPlanMinmax shows min/max virtual commit level across nodes.
func NodeMemoryPlanMinmax(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Node Virtual - min/max",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			statPanel.WithSparkline(statPanel.Sparkline{}),
			statPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 1.2},
					{Value: 1.5, Color: "#FF9F1C"},
					{Value: 2, Color: "#EA4747"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryPlanMinVirtualCommitLevel,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryPlanMaxVirtualCommitLevel,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeSystemExceedsReservationAlertNow shows the share of nodes firing SystemMemoryExceedsReservation.
func NodeMemoryNodeSystemExceedsReservationAlertNow(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("System Exceeding Reservation",
		panel.Description("This panel provides the percentage of nodes where core system (hypervisor) components are utilizing more than reserved memory. This should not happen, and is specifically critical for clusters under load. Keep it green."),
		panel.AddLink("https://access.redhat.com/solutions/5788171",
			link.Name("Red Hat Customer Portal Article"),
			link.TargetBlank(true),
		),
		panel.AddLink("https://github.com/openshift/runbooks/blob/master/alerts/machine-config-operator/SystemMemoryExceedsReservation.md",
			link.Name("Upstream OpenShift Alert Runbook"),
			link.Tooltip("Runbook"),
		),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{Unit: &dashboards.PercentDecimalUnit}),
			statPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 1, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeSystemExceedsReservationAlertNow,
			query.SeriesNameFormat("{{alertname}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryClusterPressure plots cluster memory pressure (waiting vs stalled).
func NodeMemoryClusterPressure(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster - Memory Pressure",
		panel.Description("The pressure is indicating if workloads are waiting for memory."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit:        &dashboards.DecimalUnit,
					ShortValues: true,
				},
				Max: 1,
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.1},
					{Value: 0.25, Color: "#FF9F1C"},
					{Value: 0.5, Color: "#EA4747"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterPressureWaiting,
			query.SeriesNameFormat("Waiting"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterPressureStalled,
			query.SeriesNameFormat("Stalled"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryClusterUtilizationHistory shows capacity, utilization, and planned requests over time.
func NodeMemoryClusterUtilizationHistory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Physical Memory Utilization & Requests",
		panel.Description("This is showing the total system utilization (split between virt and non virt workloads). In addition the current plan (requests) are shown in order to to show how much available memory the scheduler is seeing."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
				Min: 0,
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.WithQuerySettings([]timeSeriesPanel.QuerySettingsItem{
				{QueryIndex: 0, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#8f8fff"},
				{QueryIndex: 1, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#EE6C6C"},
				{QueryIndex: 2, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#643535"},
				{QueryIndex: 3, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#FFCC00"},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistoryCapacity,
			query.SeriesNameFormat("Node memory capacity"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistoryUtilWithVirt,
			query.SeriesNameFormat("Utilization - Node memory utilization (with virt)"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistoryUtilWithoutVirt,
			query.SeriesNameFormat("Utilization - Node memory utilization (without virt)"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistoryPlanRequests,
			query.SeriesNameFormat("Plan - Memory requests"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryClusterUtilizationHistorySummary summarizes allocatable, VM plan, and utilization (worst-case view).
func NodeMemoryClusterUtilizationHistorySummary(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster Utilization",
		panel.Description("The virtual memory assignment is showing the worst-case scenario if all virtual memory was used right now. The assigned virtual memory is shown on top of the non virtualization related memory utilization. The current utilization plus worst-case virtual memory utilization is indicating the worst case memory overcommitment."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
				Min: 0,
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.WithQuerySettings([]timeSeriesPanel.QuerySettingsItem{
				{QueryIndex: 1, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#59CC8D"},
				{QueryIndex: 2, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#FFCC00"},
				{QueryIndex: 3, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#EE6C6C"},
				{QueryIndex: 0, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#196b3e"},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0,
				LineWidth:    1.25,
				PointRadius:  2.75,
				ConnectNulls: false,
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistorySummaryAllocatablePlusSwap,
			query.SeriesNameFormat("Cluster allocatable memory + swap capacity"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistorySummaryAllocatable,
			dashboards.AddQueryDataSource(datasourceName),
			query.SeriesNameFormat("Cluster allocatable memory capacity"),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistorySummaryPlanVMAssigned,
			query.SeriesNameFormat("Plan - VM assigned virtual memory"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistorySummaryUtilizationCluster,
			query.SeriesNameFormat("Utilization - Cluster memory utilization"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterUtilizationHistorySummaryPlanMaxVMAssigned,
			query.SeriesNameFormat("Plan - Maximum VM assigned virtual memory"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryClusterVirtualCommittedHistory shows virtual memory assignment vs capacity over time.
func NodeMemoryClusterVirtualCommittedHistory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Virtual Memory Assignment",
		panel.Description("The virtual memory assignment is showing the worst-case scenario if all virtual memory was used right now. The assigned virtual memory is shown on top of the non virtualization related memory utilization. The current utilization plus worst-case virtual memory utilization is indicating the worst case memory overcommitment."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
				Min: 0,
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.WithQuerySettings([]timeSeriesPanel.QuerySettingsItem{
				{QueryIndex: 0, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#8f8fff"},
				{QueryIndex: 1, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#FFCC00"},
				{QueryIndex: 2, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#7c6400"},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0,
				LineWidth:    1.25,
				PointRadius:  2.75,
				ConnectNulls: false,
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterVirtualCommittedHistoryNodeCapacity,
			query.SeriesNameFormat("Node capacity"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterVirtualCommittedHistoryPlanVMAssigned,
			query.SeriesNameFormat("Plan - VM assigned virtual memory"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryClusterVirtualCommittedHistoryUtilWithoutVirt,
			query.SeriesNameFormat("Utilization - Node memory utilization (without virt)"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodePressureHistory plots memory PSI waiting rate per instance.
func NodeMemoryNodePressureHistory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Node - Pressure",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit:        &dashboards.DecimalUnit,
					ShortValues: true,
				},
				Min: 0,
				Max: 2,
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodePressureHistory,
			query.SeriesNameFormat("{{instance}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeRequestsHistory shows pod memory requests as a fraction of node allocatable.
func NodeMemoryNodeRequestsHistory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Plan - Pod Requests per Node",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.8, Color: "#FFB249"},
					{Value: 0.9, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeRequestsHistory,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeUtilizationHistory shows actual memory overcommit level per node.
func NodeMemoryNodeUtilizationHistory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Utilization - Actual Overcommit Level",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.8, Color: "#FFB249"},
					{Value: 0.9, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeUtilizationHistory,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryUtilizationDistribution plots per-node virtual memory commit level.
func NodeMemoryUtilizationDistribution(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Plan - Virtual Memory Commit Level",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 1.2},
					{Value: 1.5, Color: "#FF9F1C"},
					{Value: 2, Color: "#EA4747"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryUtilizationDistribution,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemorySwap shows aggregated cluster swap available vs used.
func NodeMemorySwap(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Cluster - Aggregated Swap",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.WithQuerySettings([]timeSeriesPanel.QuerySettingsItem{
				{QueryIndex: 0, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#59CC8D"},
				{QueryIndex: 1, ColorMode: timeSeriesPanel.FixedMode, ColorValue: "#FFB249"},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemorySwapAvailableBytes,
			query.SeriesNameFormat("Available"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemorySwapUsedBytes,
			query.SeriesNameFormat("Used"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeSystemReservedUtilizationHistory shows top nodes by system reserved memory utilization.
func NodeMemoryNodeSystemReservedUtilizationHistory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Utilization - Reserved System Memory",
		panel.Description("This graph is showing the utilization of the hypervisor system reserved memory. The utilization must stay below 100%, otherwise the hypervisor is using more memory for system processes than what was reserved. This is putting system processes at risk."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.9},
					{Value: 1, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeSystemReservedUtilizationHistory,
			query.SeriesNameFormat("{{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNodeSystemReservedMinmaxHistory shows min and max system reserved utilization across nodes.
func NodeMemoryNodeSystemReservedMinmaxHistory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Utilization - min/max",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0.9, Color: "#EE6C6C"},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeSystemReservedMinHistory,
			query.SeriesNameFormat("min {{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			nodeMemoryNodeSystemReservedMaxHistory,
			query.SeriesNameFormat("max {{node}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryVMs plots per-VM memory overcommit ratio (domain + overhead vs request).
func NodeMemoryVMs(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM Overcommit Ratio",
		panel.Description("Any value larger than 1 shows that the VM is using more memory than it requested"),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 1},
				},
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryVMsOvercommitRatio,
			query.SeriesNameFormat("{{namespace}}/{{label_vm_kubevirt_io_name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryVMVirtualMemoryUtilization compares VM guest memory used to launcher usage (top 10).
func NodeMemoryVMVirtualMemoryUtilization(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM Virtual Memory Utilization vs Host VM Utilization",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.DecimalUnit,
				},
				Min: 0,
			}),
			timeSeriesPanel.WithLegend(memoryTSBottomLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 1},
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0,
				LineWidth:    1.25,
				PointRadius:  2.75,
				ConnectNulls: false,
			}),
		),
		panel.AddQuery(query.PromQL(
			nodeMemoryVMVirtualMemoryUtilizationHostVMRatio,
			query.SeriesNameFormat("{{namespace}}/{{label_vm_kubevirt_io_name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// NodeMemoryNumberOfRunningVMs counts running VMs via kubevirt_vmi_memory_domain_bytes.
func NodeMemoryNumberOfRunningVMs(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Number of Running VMs",
		timeSeriesPanel.Chart(),
		panel.AddQuery(query.PromQL(
			nodeMemoryNumberOfRunningVMs,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
