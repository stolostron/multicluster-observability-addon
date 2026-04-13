package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	v1variable "github.com/perses/perses/pkg/model/api/v1/variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

func singleVMViewClusterVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("cluster",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers("kubevirt_vm_info"),
			),
			listVar.DisplayName("Cluster"),
			listVar.AllowAllValue(false),
			listVar.AllowMultiple(false),
			listVar.SortingBy(v1variable.SortAlphabeticalAsc),
		),
	)
}

func singleVMViewNamespaceVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("namespace",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("namespace",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(`kubevirt_vm_info{cluster="$cluster"}`),
			),
			listVar.DisplayName("Namespace"),
			listVar.Description("Filter the Virtual Machine by its Namespace"),
			listVar.AllowAllValue(false),
			listVar.AllowMultiple(false),
			listVar.SortingBy(v1variable.SortAlphabeticalAsc),
		),
	)
}

func singleVMViewNameVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("name",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("name",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(`kubevirt_vm_info{cluster="$cluster", namespace="$namespace"}`),
			),
			listVar.DisplayName("VM Name"),
			listVar.Description("Filter the Virtual Machine by its name"),
			listVar.AllowAllValue(false),
			listVar.AllowMultiple(false),
			listVar.SortingBy(v1variable.SortAlphabeticalAsc),
		),
	)
}

func withSingleVMGeneralInformation(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("General Information", true,
		[]GridItem{
			{X: 0, Y: 1, W: 4, H: 4},
			{X: 4, Y: 1, W: 4, H: 4},
			{X: 8, Y: 1, W: 4, H: 4},
			{X: 12, Y: 1, W: 4, H: 4},
			{X: 16, Y: 1, W: 4, H: 5},
			{X: 20, Y: 1, W: 4, H: 5},
			{X: 0, Y: 5, W: 8, H: 13},
			{X: 8, Y: 5, W: 8, H: 13},
			{X: 16, Y: 6, W: 4, H: 5},
			{X: 20, Y: 6, W: 4, H: 5},
			{X: 16, Y: 11, W: 8, H: 7},
		},
		panels.SingleVMStatus(datasource),
		panels.SingleVMCriticalSeverityAlerts(datasource),
		panels.SingleVMWarningSeverityAlerts(datasource),
		panels.SingleVMInfoSeverityAlerts(datasource),
		panels.SingleVMMemoryUsagePercentGauge(datasource),
		panels.SingleVMCPUUsagePercentGauge(datasource),
		panels.SingleVMVMInformationTable(datasource),
		panels.SingleVMGeneralInformationTable(datasource),
		panels.SingleVMFilesystemUsagePercentGauge(datasource),
		panels.SingleVMCPUDelayPercentGauge(datasource),
		panels.SingleVMAllocatedResourcesTable(datasource),
	)
}

func withSingleVMAdditionalDetails(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Additional Virtual Machines Details", false,
		[]GridItem{
			{X: 0, Y: 14, W: 12, H: 5},
			{X: 12, Y: 14, W: 12, H: 5},
		},
		panels.SingleVMNetworkTable(datasource),
		panels.SingleVMSnapshotsTable(datasource),
	)
}

func withSingleVMCPUUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("CPU Utilization", false,
		[]GridItem{
			{X: 0, Y: 17, W: 12, H: 6},
			{X: 12, Y: 17, W: 12, H: 6},
			{X: 0, Y: 23, W: 12, H: 6},
			{X: 12, Y: 23, W: 12, H: 6},
		},
		panels.SingleVMTotalCPUUsage(datasource),
		panels.SingleVMCPUUsagePercentTimeSeries(datasource),
		panels.SingleVMCPUReadyTime(datasource),
		panels.SingleVMCPUDelayPercentTimeSeries(datasource),
	)
}

func withSingleVMMemoryUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Memory Utilization", false,
		[]GridItem{
			{X: 0, Y: 18, W: 12, H: 6},
			{X: 12, Y: 18, W: 12, H: 6},
		},
		panels.SingleVMMemoryUsage(datasource),
		panels.SingleVMMemoryUsagePercentTimeSeries(datasource),
	)
}

func withSingleVMNetworkUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Network Utilization", false,
		[]GridItem{
			{X: 0, Y: 17, W: 12, H: 6},
			{X: 12, Y: 17, W: 12, H: 6},
			{X: 0, Y: 23, W: 12, H: 6},
			{X: 12, Y: 23, W: 12, H: 6},
		},
		panels.SingleVMNetworkTransmit(datasource),
		panels.SingleVMNetworkReceive(datasource),
		panels.SingleVMNetworkTransmitPacketsDropped(datasource),
		panels.SingleVMNetworkReceivePacketsDropped(datasource),
	)
}

func withSingleVMStorageUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Storage Utilization", false,
		[]GridItem{
			{X: 0, Y: 20, W: 12, H: 6},
			{X: 12, Y: 20, W: 12, H: 6},
		},
		panels.SingleVMStorageTraffic(datasource),
		panels.SingleVMStorageIOPs(datasource),
	)
}

func withSingleVMFilesystemUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("File System Utilization", false,
		[]GridItem{
			{X: 0, Y: 21, W: 12, H: 6},
			{X: 12, Y: 21, W: 12, H: 6},
		},
		panels.SingleVMFilesystemUsage(datasource),
		panels.SingleVMFilesystemUsagePercentTimeSeries(datasource),
	)
}

func withSingleVMAlerts(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Alerts", false,
		[]GridItem{
			{X: 0, Y: 19, W: 24, H: 6},
		},
		panels.SingleVMVMAlertsTable(datasource),
	)
}

// BuildSingleVMView builds the acm-openshift-virtualization-single-vm-view Perses dashboard.
func BuildSingleVMView(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-openshift-virtualization-single-vm-view",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Virtual Machine Details"),

		singleVMViewClusterVariable(datasource),
		singleVMViewNamespaceVariable(datasource),
		singleVMViewNameVariable(datasource),

		withSingleVMGeneralInformation(datasource),
		withSingleVMAdditionalDetails(datasource),
		withSingleVMCPUUtilization(datasource),
		withSingleVMMemoryUtilization(datasource),
		withSingleVMNetworkUtilization(datasource),
		withSingleVMStorageUtilization(datasource),
		withSingleVMFilesystemUtilization(datasource),
		withSingleVMAlerts(datasource),
	)
}
