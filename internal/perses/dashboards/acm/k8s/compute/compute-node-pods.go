package compute

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/compute"
)

func withNodePodsCPUUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.NodePodsCPUUsage(datasource),
	)
}

func withNodePodsCPUQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.NodePodsCPUQuota(datasource),
	)
}

func withNodePodsMemoryUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.NodePodsMemoryUsage(datasource),
	)
}

func withNodePodsMemoryQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.NodePodsMemoryQuota(datasource),
	)
}

func BuildComputeNodePods(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-compute-resources-node-pods",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Compute Resources / Node (Pods)"),

		acm.GetClusterVariable(datasource),
		acm.GetNodeVariable(datasource),

		withNodePodsCPUUsageGroup(datasource),
		withNodePodsCPUQuotaGroup(datasource),
		withNodePodsMemoryUsageGroup(datasource),
		withNodePodsMemoryQuotaGroup(datasource),
	)
}
