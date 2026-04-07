package compute

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/compute"
)

func withNamespacePodsHeadlinesGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Headlines",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 6, H: 3},
			{X: 6, Y: 0, W: 6, H: 3},
			{X: 12, Y: 0, W: 6, H: 3},
			{X: 18, Y: 0, W: 6, H: 3},
		},
		panels.NamespacePodsCPUUtilisationFromRequests(datasource),
		panels.NamespacePodsCPUUtilisationFromLimits(datasource),
		panels.NamespacePodsMemoryUtilisationFromRequests(datasource),
		panels.NamespacePodsMemoryUtilisationFromLimits(datasource),
	)
}

func withNamespacePodsCPUUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.NamespacePodsCPUUsage(datasource),
	)
}

func withNamespacePodsCPUQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(8),
		panels.NamespacePodsCPUQuota(datasource),
	)
}

func withNamespacePodsMemoryUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.NamespacePodsMemoryUsage(datasource),
	)
}

func withNamespacePodsMemoryQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(8),
		panels.NamespacePodsMemoryQuota(datasource),
	)
}

func BuildComputeNamespacePods(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-compute-resources-namespace-pods",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Compute Resources / Namespace (Pods)"),

		acm.GetClusterVariable(datasource),
		acm.GetNamespaceVariable(datasource),

		withNamespacePodsHeadlinesGroup(datasource),
		withNamespacePodsCPUUsageGroup(datasource),
		withNamespacePodsCPUQuotaGroup(datasource),
		withNamespacePodsMemoryUsageGroup(datasource),
		withNamespacePodsMemoryQuotaGroup(datasource),
	)
}
