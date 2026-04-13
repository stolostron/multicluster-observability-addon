package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	promqlVar "github.com/perses/plugins/prometheus/sdk/go/variable/promql"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

// addCustomPanelGroupWithCollapse is like AddCustomPanelGroup but allows controlling
// whether the grid section starts expanded (Open) or collapsed.
func addCustomPanelGroupWithCollapse(title string, collapseOpen bool, positions []GridItem, panelOpts ...panelgroup.Option) dashboard.Option {
	return addCustomPanelGroupImpl(title, collapseOpen, positions, panelOpts...)
}

func nodeMemoryClusterVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("cluster",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers("node_memory_MemTotal_bytes"),
			),
			listVar.DisplayName("Cluster"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func nodeMemoryNodeVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("node",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("node",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"kube_node_role",
						[]promql.LabelMatcher{{Name: "cluster", Type: "=~", Value: "$cluster"}},
					),
				),
			),
			listVar.DisplayName("Node"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(false),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValue("$__all"),
		),
	)
}

func nodeMemoryRoleVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("role",
		listVar.List(
			promqlVar.PrometheusPromQL(
				`kube_node_role{cluster=~"$cluster"}`,
				promqlVar.LabelName("role"),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName("role"),
			listVar.AllowAllValue(false),
			listVar.AllowMultiple(false),
			listVar.DefaultValue("worker"),
		),
	)
}

func withNodeMemorySummary(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Summary", true,
		[]GridItem{
			{X: 0, Y: 0, W: 10, H: 7},
			{X: 10, Y: 0, W: 6, H: 7},
			{X: 16, Y: 0, W: 6, H: 7},
			{X: 0, Y: 7, W: 22, H: 7},
			{X: 0, Y: 14, W: 5, H: 7},
			{X: 5, Y: 14, W: 5, H: 7},
			{X: 10, Y: 14, W: 5, H: 7},
			{X: 15, Y: 14, W: 7, H: 7},
		},
		panels.NodeMemoryClusterUtilizationNow(datasource),
		panels.NodeMemoryClusterVirtualCommittedNow(datasource),
		panels.NodeMemoryVmVirtualCommittedNow(datasource),
		panels.NodeMemoryClusterUtilizationHistorySummary(datasource),
		panels.NodeMemoryNodeUtilizationMinNow(datasource),
		panels.NodeMemoryNodeUtilizationMaxNow(datasource),
		panels.NodeMemoryNodePressureMaxNow(datasource),
		panels.NodeMemoryNodeSystemExceedsReservationAlertNow(datasource),
	)
}

func withNodeMemoryCluster(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Cluster", false,
		[]GridItem{
			{X: 0, Y: 0, W: 15, H: 7},
			{X: 15, Y: 0, W: 7, H: 7},
			{X: 0, Y: 7, W: 15, H: 7},
			{X: 15, Y: 7, W: 7, H: 7},
			{X: 0, Y: 14, W: 15, H: 5},
			{X: 0, Y: 19, W: 15, H: 6},
		},
		panels.NodeMemoryClusterUtilizationHistory(datasource),
		panels.NodeMemoryClusterUtilizationNow(datasource),
		panels.NodeMemoryClusterVirtualCommittedHistory(datasource),
		panels.NodeMemoryClusterVirtualCommittedNow(datasource),
		panels.NodeMemoryClusterPressure(datasource),
		panels.NodeMemorySwap(datasource),
	)
}

func withNodeMemoryNodes(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Nodes", false,
		[]GridItem{
			{X: 0, Y: 0, W: 15, H: 8},
			{X: 15, Y: 0, W: 7, H: 8},
			{X: 0, Y: 8, W: 15, H: 8},
			{X: 15, Y: 8, W: 7, H: 8},
			{X: 0, Y: 16, W: 15, H: 8},
			{X: 15, Y: 16, W: 7, H: 8},
			{X: 0, Y: 24, W: 22, H: 6},
		},
		panels.NodeMemoryNodeUtilizationHistory(datasource),
		panels.NodeMemoryNodeUtilizationMinNow(datasource),
		panels.NodeMemoryNodeRequestsHistory(datasource),
		panels.NodeMemoryNodeRequestsMinmaxNow(datasource),
		panels.NodeMemoryUtilizationDistribution(datasource),
		panels.NodeMemoryPlanMinmax(datasource),
		panels.NodeMemoryNodePressureHistory(datasource),
	)
}

func withNodeMemorySystemReserved(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("System Reserved", false,
		[]GridItem{
			{X: 0, Y: 0, W: 9, H: 8},
			{X: 9, Y: 0, W: 9, H: 8},
			{X: 18, Y: 0, W: 6, H: 8},
		},
		panels.NodeMemoryNodeSystemReservedUtilizationHistory(datasource),
		panels.NodeMemoryNodeSystemReservedMinmaxHistory(datasource),
		panels.NodeMemoryNodeSystemExceedsReservationAlertNow(datasource),
	)
}

func withNodeMemoryWorkloads(datasource string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("Workloads", false,
		[]GridItem{
			{X: 0, Y: 0, W: 14, H: 6},
			{X: 14, Y: 0, W: 7, H: 6},
			{X: 0, Y: 6, W: 14, H: 6},
			{X: 14, Y: 6, W: 7, H: 6},
		},
		panels.NodeMemoryVMs(datasource),
		panels.NodeMemoryVmVirtualCommittedNow(datasource),
		panels.NodeMemoryVMVirtualMemoryUtilization(datasource),
		panels.NodeMemoryNumberOfRunningVMs(datasource),
	)
}

// BuildNodeMemoryOverview builds the acm-node-memory-overview Perses dashboard.
func BuildNodeMemoryOverview(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-node-memory-overview",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Nodes Memory"),

		nodeMemoryClusterVariable(datasource),
		nodeMemoryNodeVariable(datasource),
		nodeMemoryRoleVariable(datasource),

		withNodeMemorySummary(datasource),
		withNodeMemoryCluster(datasource),
		withNodeMemoryNodes(datasource),
		withNodeMemorySystemReserved(datasource),
		withNodeMemoryWorkloads(datasource),
	)
}
