package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
)

// Perses table format strings for timestamp columns (Grafana dateTimeAsIso /
// dateTimeFromNow). community-mixins dashboards helpers do not expose these
// yet; align with dashboards.DateTimeLocalUnit / dashboards.RelativeTimeUnit
// when added upstream.
var (
	perTableUnitDateTimeLocal = "datetime-local"
	perTableUnitRelativeTime  = "relative-time"
)

func TotalAllocatedCPU(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Allocated CPU",
		panel.Description("The total CPUs of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.DecimalUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTotalAllocatedCPUStatQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TotalAllocatedMemory(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Allocated Memory",
		panel.Description("The total Memory of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.BytesUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTotalAllocatedMemoryStatQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TotalAllocatedDisk(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Allocated Disk",
		panel.Description("The total disk size of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.BytesUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTotalAllocatedDiskStatQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TimeInStatusTable(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Virtual Machines List by Time In Status",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "cluster", Header: "Cluster", EnableSorting: true},
				{Name: "clusterID", Hide: true},
				{Name: "container", Hide: true},
				{Name: "endpoint", Hide: true},
				{Name: "instance", Hide: true},
				{Name: "job", Hide: true},
				{Name: "namespace", Header: "Namespace", EnableSorting: true},
				{Name: "name", Header: "VM Name", EnableSorting: true},
				{Name: "pod", Hide: true},
				{Name: "receive", Hide: true},
				{Name: "service", Hide: true},
				{Name: "status", Header: "Status", EnableSorting: true},
				{Name: "tenant_id", Hide: true},
				{Name: "clusterType", Hide: true},
				{
					Name: "value #6", Header: "Allocated CPU", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &dashboards.DecimalUnit, DecimalPlaces: 0},
				},
				{
					Name: "value #5", Header: "Allocated Memory", EnableSorting: true,
					Format: &commonSdk.Format{ShortValues: true, Unit: &dashboards.BytesUnit},
				},
				{
					Name: "value #1", Header: "Allocated Disk", EnableSorting: true,
					Format: &commonSdk.Format{ShortValues: true, Unit: &dashboards.BytesUnit},
				},
				{
					Name: "value #3", Header: "Time Since Last Migration", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &perTableUnitRelativeTime},
				},
				{
					Name: "value #4", Header: "Last Migration", Hide: true,
					Format: &commonSdk.Format{Unit: &perTableUnitDateTimeLocal},
				},
				{
					Name: "value #2", Header: "Time in Status", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &dashboards.SecondsUnit},
				},
				{Name: "__name__", Hide: true},
				{Name: "guest_os_kernel_release", Hide: true},
				{Name: "guest_os_machine", Hide: true},
				{Name: "os", Hide: true},
				{Name: "disk_name", Hide: true},
			}),
			tablePanel.Transform([]commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"cluster", "name", "namespace"}}},
			}),
		),
		addColumnDataLink("name", vmDetailsDashboardLinkByValue(project)),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTableDiskQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTableTimeInStatusQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTableTimeSinceLastMigrationQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTableMigrationEndMsQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTableMemoryQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			vmByTimeInStatusTableCPUQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
