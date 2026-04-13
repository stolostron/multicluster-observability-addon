package virtualization

import (
	"time"

	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	variable "github.com/perses/perses/pkg/model/api/v1/variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

func singleClusterClusterVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("cluster",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(`kubevirt_hyperconverged_operator_health_status{name=~".*hyperconverged.*"}`),
			),
			listVar.DisplayName("Cluster"),
			listVar.AllowAllValue(false),
			listVar.AllowMultiple(false),
			listVar.SortingBy(variable.SortAlphabeticalAsc),
		),
	)
}

func singleClusterHealthImpactVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("health_impact",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("operator_health_impact",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(`ALERTS{kubernetes_operator_part_of="kubevirt"}`),
			),
			listVar.DisplayName("Alerts - Impact on Operator Health"),
			listVar.Description("Filter the alerts by their impact on the operator health"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.DefaultValues("$__all"),
			listVar.SortingBy(variable.SortNone),
		),
	)
}

func singleClusterSeverityVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("severity",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("severity",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(`ALERTS{kubernetes_operator_part_of="kubevirt"}`),
			),
			listVar.DisplayName("Alerts - Severity"),
			listVar.Description("Filter the alerts by their severity level"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.DefaultValues("$__all"),
			listVar.SortingBy(variable.SortNone),
		),
	)
}

func withSingleClusterGeneralInformation(datasource, project string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("General Information", true,
		[]GridItem{
			{X: 0, Y: 0, W: 4, H: 3},   // Cluster Name
			{X: 4, Y: 0, W: 4, H: 3},   // OpenShift Virt Version
			{X: 8, Y: 0, W: 4, H: 3},   // Provider
			{X: 12, Y: 0, W: 4, H: 3},  // OpenShift Version
			{X: 16, Y: 0, W: 4, H: 3},  // Operator Status
			{X: 20, Y: 0, W: 4, H: 3},  // Operator Conditions
			{X: 0, Y: 3, W: 4, H: 3},   // Total Nodes
			{X: 0, Y: 6, W: 4, H: 3},   // Total VMs
			{X: 4, Y: 3, W: 20, H: 6},  // Virtual Machines by Status
			{X: 0, Y: 9, W: 12, H: 7},  // Running VMs by OS
			{X: 12, Y: 9, W: 12, H: 7}, // Recent VMs Started
		},
		panels.SingleClusterClusterName(datasource),
		panels.SingleClusterOpenshiftVirtVersion(datasource),
		panels.SingleClusterProvider(datasource),
		panels.SingleClusterOpenshiftVersion(datasource),
		panels.SingleClusterOperatorStatus(datasource),
		panels.SingleClusterOperatorConditions(datasource),
		panels.SingleClusterTotalNodes(datasource),
		panels.SingleClusterTotalVMs(datasource),
		panels.SingleClusterVirtualMachinesByStatus(datasource),
		panels.SingleClusterRunningVMsByOS(datasource),
		panels.SingleClusterRecentVMsStarted(datasource, project),
	)
}

func withSingleClusterAdditionalVMDetails(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Additional Virtual Machines Details", false,
		[]GridItem{
			{X: 0, Y: 15, W: 12, H: 8},
			{X: 12, Y: 15, W: 12, H: 8},
		},
		panels.SingleClusterVMsRunningByNode(datasource),
		panels.SingleClusterVMsByStatus(datasource),
	)
}

func withSingleClusterCPUTop20(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("CPU Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 16, W: 12, H: 7},
			{X: 12, Y: 16, W: 12, H: 7},
			{X: 0, Y: 23, W: 12, H: 7},
			{X: 12, Y: 23, W: 12, H: 7},
			{X: 0, Y: 30, W: 12, H: 7},
			{X: 12, Y: 30, W: 12, H: 7},
		},
		panels.SingleClusterNodesCPUUtilization(datasource),
		panels.SingleClusterVMsTotalCPUUsage(datasource),
		panels.SingleClusterNodesCPUUsagePercent(datasource),
		panels.SingleClusterVMsCPUUsagePercent(datasource),
		panels.SingleClusterNodesCPUStealPercent(datasource),
		panels.SingleClusterVMsCPUReadyPercent(datasource),
	)
}

func withSingleClusterMemoryTop20(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Memory Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 17, W: 12, H: 7},
			{X: 12, Y: 17, W: 12, H: 7},
			{X: 0, Y: 24, W: 12, H: 7},
			{X: 12, Y: 24, W: 12, H: 7},
		},
		panels.SingleClusterNodesMemoryUsage(datasource),
		panels.SingleClusterVMsMemoryUsage(datasource),
		panels.SingleClusterNodesMemoryUsagePercent(datasource),
		panels.SingleClusterVMsMemoryUsagePercent(datasource),
	)
}

func withSingleClusterNetworkTop20(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Network Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 18, W: 12, H: 6},
			{X: 12, Y: 18, W: 12, H: 6},
			{X: 0, Y: 24, W: 12, H: 6},
			{X: 12, Y: 24, W: 12, H: 6},
		},
		panels.SingleClusterNodesNetworkReceived(datasource),
		panels.SingleClusterVMsNetworkReceived(datasource),
		panels.SingleClusterNodesNetworkTransmitted(datasource),
		panels.SingleClusterVMsNetworkTransmitted(datasource),
	)
}

func withSingleClusterStorageTop20(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Storage Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 19, W: 12, H: 7},
			{X: 12, Y: 19, W: 12, H: 7},
			{X: 0, Y: 26, W: 12, H: 7},
			{X: 12, Y: 26, W: 12, H: 7},
		},
		panels.SingleClusterNodesVMStorageIOPS(datasource),
		panels.SingleClusterVMsStorageIOPS(datasource),
		panels.SingleClusterNodesVMStorageTraffic(datasource),
		panels.SingleClusterVMsStorageTraffic(datasource),
	)
}

func withSingleClusterAlerts(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Alerts", false,
		[]GridItem{
			{X: 0, Y: 20, W: 8, H: 3},
			{X: 8, Y: 20, W: 8, H: 3},
			{X: 16, Y: 20, W: 8, H: 3},
			{X: 0, Y: 23, W: 16, H: 5},
			{X: 16, Y: 23, W: 8, H: 5},
			{X: 0, Y: 28, W: 24, H: 11},
		},
		panels.SingleClusterCriticalSeverityAlerts(datasource),
		panels.SingleClusterWarningSeverityAlerts(datasource),
		panels.SingleClusterInfoSeverityAlerts(datasource),
		panels.SingleClusterOperatorHealthImpactAlertsTable(datasource),
		panels.SingleClusterOperatorCSVIssuesTable(datasource),
		panels.SingleClusterAllAlertsTable(datasource),
	)
}

// BuildSingleClusterView builds the acm-openshift-virtualization-single-cluster-view Perses dashboard.
func BuildSingleClusterView(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-openshift-virtualization-single-cluster-view",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Cluster Details"),
		dashboard.Duration(time.Hour),

		singleClusterClusterVariable(datasource),
		singleClusterHealthImpactVariable(datasource),
		singleClusterSeverityVariable(datasource),

		withSingleClusterGeneralInformation(datasource, project),
		withSingleClusterAdditionalVMDetails(datasource),
		withSingleClusterCPUTop20(datasource),
		withSingleClusterMemoryTop20(datasource),
		withSingleClusterNetworkTop20(datasource),
		withSingleClusterStorageTop20(datasource),
		withSingleClusterAlerts(datasource),
	)
}
