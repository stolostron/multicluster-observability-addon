package compute

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/compute"
)

func withNamespaceWorkloadsCPUUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.NamespaceWorkloadsCPUUsage(datasource),
	)
}

func withNamespaceWorkloadsCPUQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(12),
		panels.NamespaceWorkloadsCPUQuota(datasource),
	)
}

func withNamespaceWorkloadsMemoryUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.NamespaceWorkloadsMemoryUsage(datasource),
	)
}

func withNamespaceWorkloadsMemoryQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(11),
		panels.NamespaceWorkloadsMemoryQuota(datasource),
	)
}

func BuildComputeNamespaceWorkloads(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-compute-resources-namespace-workloads",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Compute Resources / Namespace (Workloads)"),

		acm.GetClusterVariable(datasource),
		acm.GetNamespaceVariable(datasource),
		acm.GetTypeVariable(datasource),

		withNamespaceWorkloadsCPUUsageGroup(datasource),
		withNamespaceWorkloadsCPUQuotaGroup(datasource),
		withNamespaceWorkloadsMemoryUsageGroup(datasource),
		withNamespaceWorkloadsMemoryQuotaGroup(datasource),
	)
}
