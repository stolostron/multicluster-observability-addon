package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	apicommon "github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/plugins/prometheus/sdk/go/query"
)

func VMInventoryTable(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Virtual Machines Inventory",
		panel.Plugin(apicommon.Plugin{
			Kind: "Table",
			Spec: map[string]any{
				"columnSettings": []any{
					map[string]any{"name": "timestamp", "hide": true},
					map[string]any{"name": "cluster", "header": "Cluster", "enableSorting": true},
					map[string]any{"name": "namespace", "header": "Namespace", "enableSorting": true},
					map[string]any{"name": "name", "header": "VM Name", "enableSorting": true},
					map[string]any{"name": "status", "header": "Status", "enableSorting": true},
					map[string]any{"name": "instance_type", "header": "Instance Type", "enableSorting": true},
					map[string]any{"name": "preference", "header": "Preference", "enableSorting": true},
					map[string]any{"name": "workload", "header": "Workload", "enableSorting": true},
					map[string]any{"name": "flavor", "header": "Flavor", "enableSorting": true},
					map[string]any{
						"name": "live_migratable", "header": "Live Migratable", "enableSorting": true,
						"cellSettings": []any{
							map[string]any{
								"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "false"}},
								"textColor": "#f2495c",
							},
						},
					},
					map[string]any{"name": "evictable", "header": "Evictable", "enableSorting": true},
					map[string]any{"name": "outdated", "header": "Outdated", "enableSorting": true},
					map[string]any{"name": "guest_os_name", "header": "OS Name", "enableSorting": true},
					map[string]any{"name": "guest_os_version_id", "header": "OS Version", "enableSorting": true},
					map[string]any{"name": "guest_os_arch", "header": "OS Arch", "enableSorting": true},
					map[string]any{"name": "value #5", "header": "Create Date", "enableSorting": true, "format": map[string]any{"unit": "datetime-local"}},
					map[string]any{
						"name": "value #4", "header": "Guest Agent", "enableSorting": true,
						"cellSettings": []any{
							map[string]any{
								"condition": map[string]any{"kind": "Value", "spec": map[string]any{"value": "1"}},
								"text":      "Reporting",
							},
							map[string]any{
								"condition": map[string]any{"kind": "Misc", "spec": map[string]any{"value": "null"}},
								"text":      "Not Reporting",
								"textColor": "#F2495C",
							},
						},
					},
					map[string]any{"name": "value #2", "header": "CPU", "enableSorting": true, "format": map[string]any{"unit": "decimal", "decimalPlaces": 0}},
					map[string]any{"name": "value #3", "header": "Memory", "enableSorting": true, "format": map[string]any{"unit": "bytes", "shortValues": true}},
					map[string]any{"name": "value #6", "header": "Disk", "enableSorting": true, "format": map[string]any{"unit": "bytes", "shortValues": true}},
					map[string]any{"name": "value #1", "hide": true},
					map[string]any{"name": "value #8", "hide": true},
					map[string]any{"name": "value #7", "hide": true},
					map[string]any{"name": "value #9", "hide": true},
					map[string]any{"name": "value #10", "hide": true},
					map[string]any{"name": "value #11", "hide": true},
					map[string]any{"name": "value #12", "hide": true},
					map[string]any{"name": "__name__", "hide": true},
					map[string]any{"name": "clusterID", "hide": true},
					map[string]any{"name": "container", "hide": true},
					map[string]any{"name": "endpoint", "hide": true},
					map[string]any{"name": "instance", "hide": true},
					map[string]any{"name": "job", "hide": true},
					map[string]any{"name": "pod", "hide": true},
					map[string]any{"name": "receive", "hide": true},
					map[string]any{"name": "service", "hide": true},
					map[string]any{"name": "tenant_id", "hide": true},
					map[string]any{"name": "guest_os_kernel_release", "hide": true},
					map[string]any{"name": "guest_os_machine", "hide": true},
					map[string]any{"name": "os", "hide": true},
					map[string]any{"name": "disk_name", "hide": true},
				},
				"transforms": []any{
					map[string]any{"kind": string(commonSdk.MergeSeriesKind), "spec": map[string]any{}},
					map[string]any{"kind": string(commonSdk.JoinByColumValueKind), "spec": map[string]any{"columns": []string{"cluster", "name", "namespace"}}},
				},
			},
		}),
		addColumnDataLink("name", tableDataLink("Virtual Machine Details", vmDetailsDashboardLinkByValueURL(project))),
		// Column "value #N" → query N mapping (positional; keep in sync with columns above):
		//   value #1  → (hidden) Query 1: VM info series value
		//   value #2  → CPU            Query 2: inventoryCPUQuery
		//   value #3  → Memory         Query 3: inventoryMemoryQuery
		//   value #4  → Guest Agent    Query 4: inventoryGuestAgentQuery
		//   value #5  → Create Date    Query 5: inventoryCreateDateQuery
		//   value #6  → Disk           Query 6: inventoryDiskQuery
		//   value #7  → (hidden)       Query 7: inventoryFlavorQuery
		//   value #8  → (hidden)       Query 8: inventoryWorkloadQuery
		//   value #9  → (hidden)       Query 9: inventoryInstanceTypeQuery
		//   value #10 → (hidden)       Query 10: inventoryPreferenceQuery
		//   value #11 → (hidden)       Query 11: inventoryGuestOSQuery
		//   value #12 → (hidden)       Query 12: inventoryEvictableOutdatedQuery
		//   value #13 → Live Migratable Query 13: inventoryLiveMigratableQuery
		// Query 1: VM info with status
		panel.AddQuery(query.PromQL(
			inventoryInfoQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 2: CPU (cores * sockets * threads)
		panel.AddQuery(query.PromQL(
			inventoryCPUQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 3: Memory
		panel.AddQuery(query.PromQL(
			inventoryMemoryQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 4: Guest Agent indicator
		panel.AddQuery(query.PromQL(
			inventoryGuestAgentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 5: Create Date
		panel.AddQuery(query.PromQL(
			inventoryCreateDateQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 6: Disk
		panel.AddQuery(query.PromQL(
			inventoryDiskQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 7: Flavor
		panel.AddQuery(query.PromQL(
			inventoryFlavorQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 8: Workload
		panel.AddQuery(query.PromQL(
			inventoryWorkloadQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 9: Instance Type
		panel.AddQuery(query.PromQL(
			inventoryInstanceTypeQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 10: Preference
		panel.AddQuery(query.PromQL(
			inventoryPreferenceQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 11: Guest OS info
		panel.AddQuery(query.PromQL(
			inventoryGuestOSQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 12: Evictable/Outdated
		panel.AddQuery(query.PromQL(
			inventoryEvictableOutdatedQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		// Query 13: Live Migratable (derived from kubevirt_vmi_non_evictable)
		panel.AddQuery(query.PromQL(
			inventoryLiveMigratableQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
