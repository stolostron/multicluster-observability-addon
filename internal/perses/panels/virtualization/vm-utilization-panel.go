package virtualization

import (
	"encoding/json"

	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	apicommon "github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/plugins/prometheus/sdk/go/query"
)

func utilizationVMNameDataLink(project string) panel.Option {
	return addColumnDataLink("name", vmDetailsDashboardLinkByValue(project))
}

func addColumnDataLink(columnName string, dataLink map[string]any) panel.Option {
	return func(b *panel.Builder) error {
		raw, err := json.Marshal(b.Spec.Plugin.Spec)
		if err != nil {
			return err
		}
		spec := map[string]any{}
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &spec); err != nil {
				return err
			}
		}
		cols, _ := spec["columnSettings"].([]any)
		for i, c := range cols {
			col, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if col["name"] == columnName {
				col["dataLink"] = dataLink
				cols[i] = col
				break
			}
		}
		spec["columnSettings"] = cols
		b.Spec.Plugin.Spec = spec
		return nil
	}
}

func VMUtilizationTable(datasourceName, project string) panelgroup.Option {
	return panelgroup.AddPanel("Virtual Machines Utilization",
		panel.Plugin(apicommon.Plugin{
			Kind: "Table",
			Spec: map[string]any{
				"columnSettings": []any{
					map[string]any{"name": "timestamp", "hide": true},
					map[string]any{"name": "cluster", "header": "Cluster", "enableSorting": true},
					map[string]any{"name": "namespace", "header": "Namespace", "enableSorting": true},
					map[string]any{"name": "name", "header": "VM Name", "enableSorting": true},
					map[string]any{"name": "status", "header": "Status", "enableSorting": true},
					map[string]any{
						"name": "value #18", "header": "Guest Agent", "enableSorting": true,
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
					map[string]any{"name": "value #3", "header": "Allocated vCPU", "hide": true},
					map[string]any{"name": "value #2", "header": "CPU Usage", "hide": true, "format": map[string]any{"unit": "seconds"}},
					map[string]any{"name": "value #4", "header": "CPU Usage (%)", "enableSorting": true, "format": map[string]any{"unit": "percent-decimal"}},
					map[string]any{"name": "value #11", "header": "CPU Delay Time", "hide": true, "format": map[string]any{"unit": "seconds"}},
					map[string]any{"name": "value #12", "header": "CPU Delay (%)", "enableSorting": true, "format": map[string]any{"unit": "percent-decimal"}},
					map[string]any{"name": "value #6", "header": "Allocated Memory", "hide": true, "format": map[string]any{"unit": "bytes", "shortValues": true}},
					map[string]any{"name": "value #5", "header": "Memory Usage", "hide": true, "format": map[string]any{"unit": "bytes", "shortValues": true}},
					map[string]any{"name": "value #7", "header": "Memory Usage (%)", "enableSorting": true, "format": map[string]any{"unit": "percent-decimal"}},
					map[string]any{"name": "value #14", "header": "Memory Swap Traffic", "enableSorting": true, "format": map[string]any{"unit": "bytes/sec", "shortValues": true}},
					map[string]any{"name": "value #13", "header": "CPU I/O Wait", "enableSorting": true, "format": map[string]any{"unit": "seconds"}},
					map[string]any{"name": "value #9", "header": "Storage Traffic", "enableSorting": true, "format": map[string]any{"unit": "bytes/sec", "shortValues": true}},
					map[string]any{"name": "value #10", "header": "Storage IOPs", "enableSorting": true, "format": map[string]any{"unit": "decimal", "decimalPlaces": 5}},
					map[string]any{"name": "value #8", "header": "Network Traffic", "enableSorting": true, "format": map[string]any{"unit": "bytes/sec", "shortValues": true}},
					map[string]any{"name": "value #1", "hide": true},
					map[string]any{"name": "value #15", "header": "File System Capacity", "hide": true, "format": map[string]any{"unit": "bytes", "shortValues": true}},
					map[string]any{"name": "value #16", "header": "File System Usage", "hide": true, "format": map[string]any{"unit": "bytes", "shortValues": true}},
					map[string]any{"name": "value #17", "header": "File System Usage (Max %)", "enableSorting": true, "format": map[string]any{"unit": "percent-decimal"}},
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
					map[string]any{"kind": "MergeSeries", "spec": map[string]any{}},
					map[string]any{"kind": "JoinByColumnValue", "spec": map[string]any{"columns": []string{"cluster", "name", "namespace"}}},
				},
			},
		}),
		utilizationVMNameDataLink(project),
		panel.AddQuery(query.PromQL(
			utilizationInfoQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationCPUUsageQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationAllocatedVCPUQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationCPUUsagePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationMemoryUsageQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationAllocatedMemoryQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationMemoryUsagePercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationNetworkTrafficQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationStorageTrafficQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationStorageIOPsQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationCPUDelayQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationCPUDelayPercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationCPUIOWaitQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationMemorySwapQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationFSCapacityQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationFSUsageQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationFSUsageMaxPercentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
		panel.AddQuery(query.PromQL(
			utilizationGuestAgentQuery,
			dashboards.AddQueryDataSource(datasourceName),
		)),
	)
}
