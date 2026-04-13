package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	v1Common "github.com/perses/perses/pkg/model/api/v1/common"
	dashboardModel "github.com/perses/perses/pkg/model/api/v1/dashboard"
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

func virtOverviewOperatorHealthVariable() dashboard.Option {
	return func(builder *dashboard.Builder) error {
		spec := dashboardModel.ListVariableSpec{
			ListSpec: variable.ListSpec{
				Display: &variable.Display{
					Name:        "Operator Health",
					Description: "Filter the clusters by the health of the OpenShift Virtualization operator",
					Hidden:      false,
				},
				DefaultValue: &variable.DefaultValue{
					SliceValues: []string{"$__all"},
				},
				AllowAllValue:  true,
				AllowMultiple:  false,
				CustomAllValue: "<3",
				Plugin: v1Common.Plugin{
					Kind: "StaticListVariable",
					Spec: map[string]any{
						"values": []map[string]any{
							{"label": "Healthy", "value": "==0"},
							{"label": "Warning", "value": "==1"},
							{"label": "Critical", "value": "==2"},
						},
					},
				},
			},
			Name: "operator_health",
		}
		builder.Dashboard.Spec.Variables = append(builder.Dashboard.Spec.Variables, dashboardModel.Variable{
			Kind: variable.KindList,
			Spec: &spec,
		})
		return nil
	}
}

func withVirtOverviewGeneralInformation(datasource, project string) dashboard.Option {
	return addCustomPanelGroupWithCollapse("General Information", true,
		[]GridItem{
			{X: 0, Y: 1, W: 3, H: 6},
			{X: 3, Y: 1, W: 3, H: 3},
			{X: 6, Y: 1, W: 3, H: 6},
			{X: 9, Y: 1, W: 3, H: 6},
			{X: 12, Y: 1, W: 12, H: 6},
			{X: 3, Y: 4, W: 3, H: 3},
			{X: 0, Y: 7, W: 6, H: 7},
			{X: 6, Y: 7, W: 9, H: 7},
			{X: 15, Y: 7, W: 9, H: 7},
		},
		panels.OverviewTotalClusters(datasource),
		panels.OverviewClustersCriticalHealth(datasource),
		panels.OverviewTotalAllocatableNodes(datasource),
		panels.OverviewTotalVMsStat(datasource),
		panels.OverviewVMsByStatusStat(datasource),
		panels.OverviewClustersWarningHealth(datasource),
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
