package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	tablePanel "github.com/perses/plugins/table/sdk/go"
	timeSeriesPanel "github.com/perses/plugins/timeserieschart/sdk/go"
)

func topConsumersTimeSeriesLegend() timeSeriesPanel.Legend {
	return timeSeriesPanel.Legend{
		Position: timeSeriesPanel.RightPosition,
		Mode:     timeSeriesPanel.TableMode,
		Values: []commonSdk.Calculation{
			commonSdk.LastNumberCalculation,
			commonSdk.MaxCalculation,
		},
	}
}

func topConsumersTimeSeriesVisual() timeSeriesPanel.Visual {
	return timeSeriesPanel.Visual{
		Display:      timeSeriesPanel.LineDisplay,
		AreaOpacity:  0.1,
		ConnectNulls: false,
		LineWidth:    1,
	}
}

// buildTopConsumersTable centralizes the boilerplate shared by every
// Top Consumers table panel. The caller only supplies the per-panel
// metadata (title, description, value-column header and format) plus
// the PromQL query string.
func buildTopConsumersTable(
	title, description, valueHeader string,
	valueFmt commonSdk.Format,
	promQL, datasourceName, project string,
) panelgroup.Option {
	return panelgroup.AddPanel(title,
		panel.Description(description),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "cluster", Header: "Cluster", EnableSorting: true},
				{Name: "namespace", Header: "Namespace", EnableSorting: true},
				{Name: "name", Header: "Virtual Machine", EnableSorting: true},
				{Name: "value", Header: valueHeader, EnableSorting: true, Format: &valueFmt},
			}),
		),
		addColumnDataLink("name", tableDataLink("Virtual Machine Details", vmDetailsDashboardLinkByValueURL(project))),
		panel.AddQuery(query.PromQL(
			promQL,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// buildTopConsumersTimeSeries centralizes the boilerplate shared by
// every Top Consumers time-series panel.
func buildTopConsumersTimeSeries(
	title, description string,
	yAxisFmt commonSdk.Format,
	promQL, datasourceName string,
) panelgroup.Option {
	return panelgroup.AddPanel(title,
		panel.Description(description),
		timeSeriesPanel.Chart(
			timeSeriesPanel.WithLegend(topConsumersTimeSeriesLegend()),
			timeSeriesPanel.WithYAxis(timeSeriesPanel.YAxis{
				Show:   true,
				Format: &yAxisFmt,
			}),
			timeSeriesPanel.WithVisual(topConsumersTimeSeriesVisual()),
		),
		panel.AddQuery(query.PromQL(
			promQL,
			query.SeriesNameFormat("{{name}} / {{namespace}}"),
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

// --- Table panels (Top Consumers) ---

func TopConsumersMemoryTable(datasourceName, project string) panelgroup.Option {
	return buildTopConsumersTable(
		"Top Consumers of Memory",
		"Virtual machines with the highest memory consumption.",
		"Avg Memory Usage",
		commonSdk.Format{Unit: &dashboards.BytesUnit},
		topConsumersMemoryTableQuery, datasourceName, project,
	)
}

func TopConsumersCPUTable(datasourceName, project string) panelgroup.Option {
	return buildTopConsumersTable(
		"Top Consumers of CPU",
		"Virtual machines with the highest CPU usage in cores. A value of 2.0 means two full CPU cores are in use.",
		"CPU Usage (cores)",
		commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 3},
		topConsumersCPUTableQuery, datasourceName, project,
	)
}

func TopConsumersStorageTrafficTable(datasourceName, project string) panelgroup.Option {
	return buildTopConsumersTable(
		"Top Consumers of Storage Traffic",
		"Virtual machines with the highest storage traffic (read + write).",
		"Storage Traffic",
		commonSdk.Format{Unit: &decBytesPerSecUnit},
		topConsumersStorageTrafficTableQuery, datasourceName, project,
	)
}

func TopConsumersStorageIOPSTable(datasourceName, project string) panelgroup.Option {
	return buildTopConsumersTable(
		"Top Consumers of Storage IOPS",
		"Virtual machines with the highest storage IOPS (read + write).",
		"Storage IOPS",
		commonSdk.Format{Unit: &opsPerSecUnit},
		topConsumersStorageIOPSTableQuery, datasourceName, project,
	)
}

func TopConsumersNetworkTrafficTable(datasourceName, project string) panelgroup.Option {
	return buildTopConsumersTable(
		"Top Consumers of Network Traffic",
		"Virtual machines with the highest network traffic (receive + transmit).",
		"Network Traffic",
		commonSdk.Format{Unit: &decBytesPerSecUnit},
		topConsumersNetworkTrafficTableQuery, datasourceName, project,
	)
}

func TopConsumersVCPUWaitTable(datasourceName, project string) panelgroup.Option {
	return buildTopConsumersTable(
		"Top Consumers of vCPU Wait",
		"Virtual machines with the highest vCPU wait time, indicating CPU contention.",
		"vCPU Wait Time",
		commonSdk.Format{Unit: &dashboards.SecondsUnit},
		topConsumersVCPUWaitTableQuery, datasourceName, project,
	)
}

func TopConsumersMemorySwapTable(datasourceName, project string) panelgroup.Option {
	return buildTopConsumersTable(
		"Top Consumers of Memory Swap Traffic",
		"Virtual machines with the highest memory swap traffic (in + out). High swap activity may indicate memory pressure.",
		"Avg Memory Swap Traffic",
		commonSdk.Format{Unit: &decBytesPerSecUnit},
		topConsumersMemorySwapTableQuery, datasourceName, project,
	)
}

// --- Time-series panels (Top Consumers Over Time) ---

func TopConsumersMemoryTimeSeries(datasourceName string) panelgroup.Option {
	return buildTopConsumersTimeSeries(
		"Memory Usage Over Time",
		"Top virtual machines by memory consumption over time.",
		commonSdk.Format{Unit: &dashboards.BytesUnit},
		topConsumersMemoryTimeSeriesQuery, datasourceName,
	)
}

func TopConsumersCPUTimeSeries(datasourceName string) panelgroup.Option {
	return buildTopConsumersTimeSeries(
		"CPU Usage Over Time",
		"Top virtual machines by CPU usage in cores over time. A value of 2.0 means two full CPU cores are in use.",
		commonSdk.Format{Unit: &dashboards.DecimalUnit},
		topConsumersCPUTimeSeriesQuery, datasourceName,
	)
}

func TopConsumersStorageTrafficTimeSeries(datasourceName string) panelgroup.Option {
	return buildTopConsumersTimeSeries(
		"Storage Traffic Over Time",
		"Top virtual machines by storage traffic (read + write) over time.",
		commonSdk.Format{Unit: &decBytesPerSecUnit},
		topConsumersStorageTrafficTimeSeriesQuery, datasourceName,
	)
}

func TopConsumersStorageIOPSTimeSeries(datasourceName string) panelgroup.Option {
	return buildTopConsumersTimeSeries(
		"Storage IOPS Over Time",
		"Top virtual machines by storage IOPS (read + write) over time.",
		commonSdk.Format{Unit: &opsPerSecUnit},
		topConsumersStorageIOPSTimeSeriesQuery, datasourceName,
	)
}

func TopConsumersNetworkTrafficTimeSeries(datasourceName string) panelgroup.Option {
	return buildTopConsumersTimeSeries(
		"Network Traffic Over Time",
		"Top virtual machines by network traffic (receive + transmit) over time.",
		commonSdk.Format{Unit: &decBytesPerSecUnit},
		topConsumersNetworkTrafficTimeSeriesQuery, datasourceName,
	)
}

func TopConsumersVCPUWaitTimeSeries(datasourceName string) panelgroup.Option {
	return buildTopConsumersTimeSeries(
		"vCPU Wait Over Time",
		"Top virtual machines by vCPU wait time over time, indicating CPU contention.",
		commonSdk.Format{Unit: &dashboards.SecondsUnit},
		topConsumersVCPUWaitTimeSeriesQuery, datasourceName,
	)
}

func TopConsumersMemorySwapTimeSeries(datasourceName string) panelgroup.Option {
	return buildTopConsumersTimeSeries(
		"Memory Swap Traffic Over Time",
		"Top virtual machines by memory swap traffic (in + out) over time. High swap activity may indicate memory pressure.",
		commonSdk.Format{Unit: &decBytesPerSecUnit},
		topConsumersMemorySwapTimeSeriesQuery, datasourceName,
	)
}
