package compute

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/compute"
)

func withPodCPUUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.PodCPUUsage(datasource),
	)
}

func withPodCPUThrottlingGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Throttling",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.PodCPUThrottling(datasource),
	)
}

func withPodCPUQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.PodCPUQuota(datasource),
	)
}

func withPodMemoryUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.PodMemoryUsage(datasource),
	)
}

func withPodMemoryQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.PodMemoryQuota(datasource),
	)
}

func BuildComputePod(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-compute-resources-pod",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Compute Resources / Pod"),

		acm.GetClusterVariable(datasource),
		acm.GetNamespaceVariable(datasource),
		acm.GetPodVariable(datasource),

		withPodCPUUsageGroup(datasource),
		withPodCPUThrottlingGroup(datasource),
		withPodCPUQuotaGroup(datasource),
		withPodMemoryUsageGroup(datasource),
		withPodMemoryQuotaGroup(datasource),
	)
}
