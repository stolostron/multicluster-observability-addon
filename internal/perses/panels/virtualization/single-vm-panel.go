package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	gaugePanel "github.com/perses/plugins/gaugechart/sdk/go"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

// Perses unit string not exported by community-mixins.
var singleVMDecBytesPerSecUnit = "decbytes/sec"

func singleVMTimeSeriesLegend() timeSeriesPanel.Legend {
	return timeSeriesPanel.Legend{
		Position: timeSeriesPanel.RightPosition,
		Mode:     timeSeriesPanel.TableMode,
		Values: []commonSdk.Calculation{
			commonSdk.LastNumberCalculation,
			commonSdk.MaxCalculation,
		},
	}
}

func singleVMGaugeThresholds802090() commonSdk.Thresholds {
	return commonSdk.Thresholds{
		Steps: []commonSdk.StepOption{
			{Value: 0, Color: "#73bf69"},
			{Value: 0.80, Color: "#EAB839"},
			{Value: 0.90, Color: "#f2495c"},
		},
	}
}

func singleVMGaugeThresholdsCPUdelay() commonSdk.Thresholds {
	return commonSdk.Thresholds{
		Steps: []commonSdk.StepOption{
			{Value: 0, Color: "#73bf69"},
			{Value: 0.05, Color: "#EAB839"},
			{Value: 0.10, Color: "#f2495c"},
		},
	}
}

// SingleVMStatus displays the current VM status as text (default black).
func SingleVMStatus(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Status",
		statPanel.Chart(
			statPanel.Calculation("last-number"),
		),
		mergePluginSpecFields(map[string]any{
			"metricLabel": "status",
			"colorMode":   "none",
		}),
		panel.AddQuery(query.PromQL(
			singleVMStatusQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMCriticalSeverityAlerts counts firing critical alerts for the VM.
func SingleVMCriticalSeverityAlerts(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Critical Severity Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: critical."),
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
			singleVMCriticalAlertsQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMWarningSeverityAlerts counts firing warning alerts for the VM.
func SingleVMWarningSeverityAlerts(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Warning Severity Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: warning."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0, Color: "#FF9F1C"},
				},
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "value"}),
		panel.AddQuery(query.PromQL(
			singleVMWarningAlertsQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMInfoSeverityAlerts counts firing info alerts for the VM.
func SingleVMInfoSeverityAlerts(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Info Severity Alerts",
		panel.Description("Total number of alerts that are firing with the severity level: info."),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0, Color: "#6E9FFF"},
				},
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "value"}),
		panel.AddQuery(query.PromQL(
			singleVMInfoAlertsQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMMemoryUsagePercentGauge shows memory usage vs requested memory.
func SingleVMMemoryUsagePercentGauge(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage (%)",
		panel.Description("This panel displays the VM memory usage Percentage. The memory usage is calculated as the allocated memory minus unused and cached memory. The data is based on the most recent metrics collected."),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			gaugePanel.Thresholds(singleVMGaugeThresholds802090()),
		),
		panel.AddQuery(query.PromQL(
			singleVMMemoryUsagePercentGaugeQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMCPUUsagePercentGauge shows CPU usage vs allocated CPU.
func SingleVMCPUUsagePercentGauge(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage (%)",
		panel.Description("This panel displays the VM CPU Usage Percentage. CPU Usage Percentage measures how much of the allocated CPU resources (cores, sockets, and threads) each VM is actively utilizing. It helps identify VMs with high or low CPU utilization relative to their allocated capacity."),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			gaugePanel.Thresholds(singleVMGaugeThresholds802090()),
		),
		panel.AddQuery(query.PromQL(
			singleVMCPUUsagePercentRatioQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMVMInformationTable shows label-based VM fields (name, status, OS, etc.).
func SingleVMVMInformationTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM Information",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "Field", EnableSorting: true},
				{Name: "Value", EnableSorting: true},
			}),
			tablePanel.Transform([]commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.MergeByColumnsKind, Spec: commonSdk.MergeColumnsSpec{
					Columns: []string{"name", "status", "guest_os_name", "guest_os_version_id", "instance_type", "workload", "flavor"},
					Name:    "Value",
				}},
			}),
		),
		mergePluginSpecFields(map[string]any{"defaultColumnHidden": true}),
		panel.AddQuery(query.PromQL(singleVMVMInformationNameQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationStatusQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationGuestOSQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationGuestOSVersionQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationInstanceTypeQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationWorkloadQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationFlavorQuery, dashboards.AddQueryDataSource(datasourceName))),
	)
}

// SingleVMAllocatedResourcesTable shows numeric allocated resources (CPU, memory, disk).
func SingleVMAllocatedResourcesTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Allocated Resources",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "Field", EnableSorting: true},
				{Name: "value", Header: "Value", EnableSorting: true},
			}),
			tablePanel.Transform([]commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.MergeIndexedColumnsKind, Spec: commonSdk.MergeIndexedColumnsSpec{
					Column: "value",
				}},
			}),
		),
		mergePluginSpecFields(map[string]any{"defaultColumnHidden": true}),
		panel.AddQuery(query.PromQL(singleVMVMInformationAllocatedCPUQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationAllocatedMemoryQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMVMInformationAllocatedDiskQuery, dashboards.AddQueryDataSource(datasourceName))),
	)
}

// SingleVMGeneralInformationTable merges namespace, node, pod, and related fields.
func SingleVMGeneralInformationTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("General Information",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "Field", EnableSorting: true},
				{Name: "Value", EnableSorting: true},
			}),
			tablePanel.Transform([]commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.MergeByColumnsKind, Spec: commonSdk.MergeColumnsSpec{
					Columns: []string{"namespace", "node", "pod", "evictable", "machine_type", "outdated"},
					Name:    "Value",
				}},
			}),
		),
		mergePluginSpecFields(map[string]any{"defaultColumnHidden": true}),
		panel.AddQuery(query.PromQL(singleVMGeneralInformationNamespaceQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMGeneralInformationNodeQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMGeneralInformationPodQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMGeneralInformationEvictableQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMGeneralInformationMachineTypeQuery, dashboards.AddQueryDataSource(datasourceName))),
		panel.AddQuery(query.PromQL(singleVMGeneralInformationOutdatedQuery, dashboards.AddQueryDataSource(datasourceName))),
	)
}

// SingleVMFilesystemUsagePercentGauge shows max filesystem usage ratio.
func SingleVMFilesystemUsagePercentGauge(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("File System Usage (%)",
		panel.Description("This panel displays the VM File System usage percentage for the disk that has the highest usage percentage."),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			gaugePanel.Thresholds(singleVMGaugeThresholds802090()),
		),
		panel.AddQuery(query.PromQL(
			singleVMFilesystemUsagePercentQuery,
			query.SeriesNameFormat("{{disk_name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMCPUDelayPercentGauge shows CPU delay as a percent of allocated CPU.
func SingleVMCPUDelayPercentGauge(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Delay (%)",
		panel.Description("This panel shows the VM CPU Delay Percentage. CPU Delay indicates the proportion of time a VM's virtual CPUs were ready to execute but had to wait for physical CPU resources due to contention."),
		gaugePanel.Chart(
			gaugePanel.Calculation("last-number"),
			gaugePanel.Format(commonSdk.Format{
				Unit:          &dashboards.PercentDecimalUnit,
				DecimalPlaces: 2,
			}),
			gaugePanel.Thresholds(singleVMGaugeThresholdsCPUdelay()),
		),
		panel.AddQuery(query.PromQL(
			singleVMCPUDelayPercentRatioQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMNetworkTable lists network addresses for the VM.
func SingleVMNetworkTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Network",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "type", Header: "Network Type", EnableSorting: true},
				{Name: "network_name", Header: "Network Name", EnableSorting: true},
				{Name: "address", Header: "IP Address", EnableSorting: true},
				{Name: "timestamp", Hide: true},
				{Name: "cluster", Hide: true},
				{Name: "name", Hide: true},
				{Name: "namespace", Hide: true},
				{Name: "value", Hide: true},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMNetworkAddressesQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMSnapshotsTable lists VM snapshots with create timestamps.
func SingleVMSnapshotsTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM Snapshots",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "cluster", Hide: true},
				{Name: "clusterID", Hide: true},
				{Name: "container", Hide: true},
				{Name: "endpoint", Hide: true},
				{Name: "instance", Hide: true},
				{Name: "job", Hide: true},
				{Name: "name", Hide: true},
				{Name: "namespace", Hide: true},
				{Name: "pod", Hide: true},
				{Name: "receive", Hide: true},
				{Name: "service", Hide: true},
				{
					Name: "value", Header: "Snapshot Create Date", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &dateTimeLocalFormatUnit},
				},
				{Name: "snapshot_name", Header: "Snapshot Name", EnableSorting: true, Width: 436},
				{Name: "tenant_id", Hide: true},
				{Name: "__name__", Hide: true},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMSnapshotsQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMTotalCPUUsage plots CPU usage rate in seconds.
func SingleVMTotalCPUUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total CPU Usage",
		panel.Description("This panel displays the total VM CPU Usage. CPU Usage represents the rate of CPU time consumed by each VM, providing insight into the most resource-intensive workloads in the cluster during the recent period."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.SecondsUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMTotalCPUUsageQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMCPUUsagePercentTimeSeries plots CPU usage percentage over time.
func SingleVMCPUUsagePercentTimeSeries(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Usage (%)",
		panel.Description("This panel displays the VM CPU Usage Percentage. CPU Usage Percentage measures how much of the allocated CPU resources (cores, sockets, and threads) each VM is actively utilizing. It helps identify VMs with high or low CPU utilization relative to their allocated capacity."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMCPUUsagePercentRatioQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMCPUReadyTime plots CPU ready / delay related time series (source dashboard uses seconds axis).
func SingleVMCPUReadyTime(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Ready Time (%)",
		panel.Description("This panel shows the VM CPU Ready Time percentage. CPU Ready indicates the proportion of time a VM's virtual CPUs were ready to execute but had to wait for physical CPU resources due to contention."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0, Color: "#73bf69"},
				},
			}),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMCPUReadyTimeQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMCPUDelayPercentTimeSeries plots vCPU delay as a percentage of allocation.
func SingleVMCPUDelayPercentTimeSeries(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("CPU Delay (%)",
		panel.Description("This panel shows the VM CPU Delay Percentage. CPU Delay indicates the proportion of time a VM's virtual CPUs were ready to execute but had to wait for physical CPU resources due to contention."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Min:  0,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMCPUDelayPercentRatioQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMMemoryUsage plots used memory (excl. unused/cache) in bytes.
func SingleVMMemoryUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage",
		panel.Description("This panel displays the VM memory usage. The memory usage is calculated as the allocated memory minus unused and cached memory. The data is based on the most recent metrics collected."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMMemoryUsageBytesQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMMemoryUsagePercentTimeSeries plots memory usage vs request.
func SingleVMMemoryUsagePercentTimeSeries(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Memory Usage (%)",
		panel.Description("This panel displays the VM memory usage Percentage. The memory usage is calculated as the allocated memory minus unused and cached memory. The data is based on the most recent metrics collected."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.Thresholds(commonSdk.Thresholds{
				Steps: []commonSdk.StepOption{
					{Value: 0, Color: "#73bf69"},
				},
			}),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMMemoryUsagePercentTimeSeriesQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMNetworkTransmit plots transmit throughput (decbytes/sec).
func SingleVMNetworkTransmit(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Network Usage - Transmit",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &singleVMDecBytesPerSecUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: true,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMNetworkTransmitQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMNetworkReceive plots receive throughput (decbytes/sec).
func SingleVMNetworkReceive(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Network Usage - Receive",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &singleVMDecBytesPerSecUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: true,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMNetworkReceiveQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMNetworkTransmitPacketsDropped plots dropped transmit packets rate.
func SingleVMNetworkTransmitPacketsDropped(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Network Usage - Transmit Packets Dropped",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: true,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMNetworkTransmitPacketsDroppedQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMNetworkReceivePacketsDropped plots dropped receive packets rate.
func SingleVMNetworkReceivePacketsDropped(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Network Usage - Receive Packets Dropped",
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.DecimalUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: true,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMNetworkReceivePacketsDroppedQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMStorageTraffic plots read+write bytes per second.
func SingleVMStorageTraffic(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Storage Traffic",
		panel.Description("This panel displays the VM storage traffic (read + write) over the past 10 minutes. The metric aggregates the total input/output operations, helping identify workloads with the highest storage activity and potential hotspots for performance optimization."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &singleVMDecBytesPerSecUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMStorageTrafficQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMStorageIOPs plots read+write IOPS.
func SingleVMStorageIOPs(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Storage IOPS",
		panel.Description("This panel displays the VM storage IOPS (read + write) over the past 10 minutes. The metric aggregates the total input/output operations per second, helping identify workloads with the highest storage activity and potential hotspots for performance optimization."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &commonSdk.Format{Unit: &opsPerSecUnit, ShortValues: true},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0.1,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMStorageIOPsQuery,
			query.SeriesNameFormat("{{name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMFilesystemUsage plots used bytes per disk.
func SingleVMFilesystemUsage(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("File System Usage",
		panel.Description("This panel displays the VM file system usage. The data is based on the most recent metrics collected."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.BytesUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMFilesystemUsedBytesQuery,
			query.SeriesNameFormat("{{disk_name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMFilesystemUsagePercentTimeSeries plots usage vs capacity per disk.
func SingleVMFilesystemUsagePercentTimeSeries(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("File System Usage (%)",
		panel.Description("This panel displays the VM file system usage Percentage. The data is based on the most recent metrics collected."),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(singleVMTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show: true,
				Format: &commonSdk.Format{
					Unit: &dashboards.PercentDecimalUnit,
				},
			}),
			timeSeriesPanel.WithVisual(timeSeriesPanel.Visual{
				Display:      timeSeriesPanel.LineDisplay,
				AreaOpacity:  0,
				ConnectNulls: false,
				LineWidth:    1,
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMFilesystemUsagePercentQuery,
			query.SeriesNameFormat("{{disk_name}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// SingleVMVMAlertsTable lists firing alerts for the VM.
func SingleVMVMAlertsTable(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("VM Alerts",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "severity", Header: "Severity", EnableSorting: true, Width: 150},
				{Name: "alertstate", Header: "State", EnableSorting: true, Hide: false, Width: 150},
				{Name: "pod", EnableSorting: true, Width: 330},
				{Name: "timestamp", Hide: true},
				{Name: "value", Hide: true},
				{Name: "cluster", Header: "Cluster", Hide: true},
				{Name: "alertname", Header: "Name", EnableSorting: true},
				{Name: "name", Header: "VM Name", EnableSorting: true},
				{Name: "namespace", Header: "Namespace", EnableSorting: true},
				{Name: "operator_health_impact", Header: "Operator Health Impact", EnableSorting: true},
			}),
		),
		panel.AddQuery(query.PromQL(
			singleVMVMAlertsTableQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
