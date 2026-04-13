package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	"github.com/perses/perses/pkg/model/api/v1/variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

func virtOverviewClusterVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("cluster",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers("kubevirt_hyperconverged_operator_health_status"),
			),
			listVar.DisplayName("Cluster"),
			listVar.Hidden(false),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
			listVar.SortingBy(variable.SortAlphabeticalAsc),
		),
	)
}

// NOTE: The "operator_health" variable values are literal PromQL comparison
// operator strings (e.g. "==0", "==1", "==2", "<3"). Consuming queries
// concatenate these directly into metric selectors, e.g.:
//
//	kubevirt_hyperconverged_operator_health_status{...}$operator_health
//
// Normalizing, trimming, or altering these values will silently break all
// panels that depend on this variable.
func virtOverviewOperatorHealthVariable() dashboard.Option {
	return AddStaticListVariable(
		"operator_health",
		"Operator Health",
		"Filter the clusters by the health of the OpenShift Virtualization operator",
		[]StaticListValue{
			{Label: "Healthy", Value: "==0"},
			{Label: "Warning", Value: "==1"},
			{Label: "Critical", Value: "==2"},
		},
		"$__all",
		true,  // allowAll — "<3" matches all health states
		false, // allowMultiple — single selection maps to one PromQL operator
		"<3",
	)
}

func withVirtOverviewGeneralInformation(datasource, project string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("General Information", true,
		[]GridItem{
			// Row 1: five stat panels, W:4 each
			{X: 0, Y: 1, W: 4, H: 3},
			{X: 4, Y: 1, W: 4, H: 3},
			{X: 8, Y: 1, W: 4, H: 3},
			{X: 12, Y: 1, W: 4, H: 3},
			{X: 16, Y: 1, W: 4, H: 3},
			// Row 2: one panel per VM status, W:4 each
			{X: 0, Y: 4, W: 4, H: 3},
			{X: 4, Y: 4, W: 4, H: 3},
			{X: 8, Y: 4, W: 4, H: 3},
			{X: 12, Y: 4, W: 4, H: 3},
			{X: 16, Y: 4, W: 4, H: 3},
			// Row 3: tables
			{X: 0, Y: 7, W: 6, H: 7},
			{X: 6, Y: 7, W: 9, H: 7},
			{X: 15, Y: 7, W: 9, H: 7},
		},
		panels.OverviewTotalClusters(datasource),
		panels.OverviewClustersCriticalHealth(datasource),
		panels.OverviewClustersWarningHealth(datasource),
		panels.OverviewTotalAllocatableNodes(datasource),
		panels.OverviewTotalVMsStat(datasource, project),
		panels.OverviewVMsRunning(datasource, project),
		panels.OverviewVMsInErrorStat(datasource, project),
		panels.OverviewVMsStopped(datasource, project),
		panels.OverviewVMsStarting(datasource, project),
		panels.OverviewVMsMigrating(datasource, project),
		panels.OverviewVMsStartedLast7Days(datasource, project),
		panels.OverviewClustersByOperatorVersion(datasource),
		panels.OverviewClustersByOpenShiftVersion(datasource),
	)
}

func withVirtOverviewOperatorHealth(datasource, project string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Operator Health", false,
		[]GridItem{
			{X: 0, Y: 15, W: 24, H: 8},
		},
		panels.OverviewOperatorHealthByCluster(datasource, project),
	)
}

func withVirtOverviewAdditionalVMDetails(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Additional Virtual Machines Details", false,
		[]GridItem{
			{X: 0, Y: 16, W: 12, H: 7},
			{X: 12, Y: 16, W: 12, H: 7},
			{X: 0, Y: 23, W: 12, H: 7},
			{X: 12, Y: 23, W: 12, H: 7},
		},
		panels.OverviewRunningVMsByOS(datasource),
		panels.OverviewRunningVMsByClusterTop20(datasource),
		panels.OverviewVMsByStatusTimeSeries(datasource),
		panels.OverviewRunningVMsByNodeTop20(datasource),
	)
}

func withVirtOverviewCPUUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("CPU Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 17, W: 12, H: 7},
			{X: 12, Y: 17, W: 12, H: 7},
		},
		panels.OverviewCPUUsageByCluster(datasource),
		panels.OverviewCPUUsagePercentByCluster(datasource),
	)
}

func withVirtOverviewMemoryUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Memory Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 18, W: 12, H: 7},
			{X: 12, Y: 18, W: 12, H: 7},
		},
		panels.OverviewMemoryUsageByCluster(datasource),
		panels.OverviewMemoryUsagePercentByCluster(datasource),
	)
}

func withVirtOverviewNetworkUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Network Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 19, W: 12, H: 7},
			{X: 12, Y: 19, W: 12, H: 7},
		},
		panels.OverviewNetworkReceivedByCluster(datasource),
		panels.OverviewNetworkTransmittedByCluster(datasource),
	)
}

func withVirtOverviewStorageUtilization(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Storage Utilization - Top 20", false,
		[]GridItem{
			{X: 0, Y: 20, W: 12, H: 7},
			{X: 12, Y: 20, W: 12, H: 7},
		},
		panels.OverviewStorageTrafficByCluster(datasource),
		panels.OverviewStorageIOPsByCluster(datasource),
	)
}

// BuildVirtOverview builds the acm-openshift-virtualization-overview Perses dashboard.
func BuildVirtOverview(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-openshift-virtualization-overview",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Clusters Overview"),

		virtOverviewClusterVariable(datasource),
		virtOverviewOperatorHealthVariable(),

		withVirtOverviewGeneralInformation(datasource, project),
		withVirtOverviewOperatorHealth(datasource, project),
		withVirtOverviewAdditionalVMDetails(datasource),
		withVirtOverviewCPUUtilization(datasource),
		withVirtOverviewMemoryUtilization(datasource),
		withVirtOverviewNetworkUtilization(datasource),
		withVirtOverviewStorageUtilization(datasource),
	)
}
