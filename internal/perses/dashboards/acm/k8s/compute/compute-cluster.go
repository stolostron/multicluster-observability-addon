package compute

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/compute"
)

func withClusterHeadlinesGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Headlines",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 4, H: 3},
			{X: 4, Y: 0, W: 4, H: 3},
			{X: 8, Y: 0, W: 4, H: 3},
			{X: 12, Y: 0, W: 4, H: 3},
			{X: 16, Y: 0, W: 4, H: 3},
			{X: 20, Y: 0, W: 4, H: 3},
		},
		panels.ClusterCPUUtilisation(datasource),
		panels.ClusterCPURequestsCommitment(datasource),
		panels.ClusterCPULimitsCommitment(datasource),
		panels.ClusterMemoryUtilisation(datasource),
		panels.ClusterMemoryRequestsCommitment(datasource),
		panels.ClusterMemoryLimitsCommitment(datasource),
	)
}

func withClusterCPUGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.ClusterCPUUsage(datasource),
	)
}

func withClusterCPUQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(14),
		panels.ClusterCPUQuota(datasource),
	)
}

func withClusterMemoryGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.ClusterMemoryUsage(datasource),
	)
}

func withClusterMemoryQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Requests",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.ClusterMemoryQuota(datasource),
	)
}

func BuildComputeCluster(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-compute-resources-cluster",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Compute Resources / Cluster"),

		acm.GetClusterVariable(datasource),

		withClusterHeadlinesGroup(datasource),
		withClusterCPUGroup(datasource),
		withClusterCPUQuotaGroup(datasource),
		withClusterMemoryGroup(datasource),
		withClusterMemoryQuotaGroup(datasource),
	)
}
