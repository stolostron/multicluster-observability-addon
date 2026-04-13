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

// community-mixins dashboards helpers do not export these format units yet.
var (
	dateTimeLocalFormatUnit = "datetime-local"
	hoursFormatUnit         = string(commonSdk.HoursUnit)
)

func TotalUptimePercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Uptime %",
		panel.Description("The total uptime % of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			serviceLevelUptimePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TotalPlannedDowntimePercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Planned Downtime %",
		panel.Description("The total planned downtime % of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			serviceLevelPlannedDowntimePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TotalUnplannedDowntimePercent(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Unplanned Downtime %",
		panel.Description("The total unplanned downtime % of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &dashboards.PercentDecimalUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			serviceLevelUnplannedDowntimePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TotalUptimeHours(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Uptime (Hours)",
		panel.Description("The total uptime hours of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &hoursFormatUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			serviceLevelUptimeHoursQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TotalPlannedDowntimeHours(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Planned Downtime (Hours)",
		panel.Description("The total planned downtime hours of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &hoursFormatUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			serviceLevelPlannedDowntimeHoursQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func TotalUnplannedDowntimeHours(datasourceName string) panelgroup.Option {
	return panelgroup.AddPanel("Total Unplanned Downtime (Hours)",
		panel.Description("The total unplanned downtime hours of the VMs that are listed in the dashboard"),
		statPanel.Chart(
			statPanel.Calculation("last-number"),
			statPanel.Format(commonSdk.Format{
				Unit: &hoursFormatUnit,
			}),
		),
		mergePluginSpecFields(map[string]any{"colorMode": "none"}),
		panel.AddQuery(query.PromQL(
			serviceLevelUnplannedDowntimeHoursQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}

func ServiceLevelTable(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Virtual Machines Service Level Summary",
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{Name: "timestamp", Hide: true},
				{Name: "cluster", Header: "Cluster", EnableSorting: true},
				{Name: "namespace", Header: "Namespace", EnableSorting: true},
				{Name: "name", Header: "VM Name", EnableSorting: true},
				{Name: "status_group", Header: "Status", EnableSorting: true},
				{Name: "status", Header: "Reason", EnableSorting: true},
				{Name: "clusterID", Hide: true},
				{Name: "container", Hide: true},
				{Name: "endpoint", Hide: true},
				{Name: "instance", Hide: true},
				{Name: "job", Hide: true},
				{Name: "pod", Hide: true},
				{Name: "receive", Hide: true},
				{Name: "service", Hide: true},
				{Name: "tenant_id", Hide: true},
				{
					Name: "value #6", Header: "VM Create Date", Hide: true,
					Format: &commonSdk.Format{Unit: &dateTimeLocalFormatUnit},
				},
				{Name: "value #1", Hide: true},
				{
					Name: "value #7", Header: "Uptime", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &hoursFormatUnit},
				},
				{
					Name: "value #8", Header: "Uptime %", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit},
				},
				{
					Name: "value #2", Header: "Planned Downtime", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &hoursFormatUnit},
				},
				{
					Name: "value #3", Header: "Planned Downtime %", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit},
				},
				{
					Name: "value #4", Header: "Unplanned Downtime", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &hoursFormatUnit},
				},
				{
					Name: "value #5", Header: "Unplanned Downtime %", EnableSorting: true,
					Format: &commonSdk.Format{Unit: &dashboards.PercentDecimalUnit},
				},
				{Name: "__name__", Hide: true},
				{Name: "guest_os_kernel_release", Hide: true},
				{Name: "guest_os_machine", Hide: true},
				{Name: "os", Hide: true},
				{Name: "disk_name", Hide: true},
				{Name: "value #9", Hide: true},
			}),
			tablePanel.Transform([]commonSdk.Transform{
				{Kind: commonSdk.MergeSeriesKind, Spec: commonSdk.MergeSeriesSpec{}},
				{Kind: commonSdk.JoinByColumValueKind, Spec: commonSdk.JoinByColumnValueSpec{Columns: []string{"cluster", "name", "namespace"}}},
			}),
		),
		addColumnDataLink("name", vmDetailsDashboardLinkByValue(project)),
		panel.AddQuery(query.PromQL(
			serviceLevelTableVMInfoQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTablePlannedDowntimeHoursQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTablePlannedDowntimePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTableUnplannedDowntimeHoursQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTableUnplannedDowntimePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTableCreateDateQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTableUptimeHoursQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTableUptimePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			serviceLevelTableErrorStatusQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
