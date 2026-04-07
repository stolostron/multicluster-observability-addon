package compute

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/compute"
)

func withWorkloadCPUUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.WorkloadCPUUsage(datasource),
	)
}

func withWorkloadCPUQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.WorkloadCPUQuota(datasource),
	)
}

func withWorkloadMemoryUsageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Usage",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.WorkloadMemoryUsage(datasource),
	)
}

func withWorkloadMemoryQuotaGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory Quota",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(7),
		panels.WorkloadMemoryQuota(datasource),
	)
}

func BuildComputeWorkload(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-compute-resources-workload",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Compute Resources / Workload"),

		acm.GetClusterVariable(datasource),
		acm.GetNamespaceVariable(datasource),
		acm.GetWorkloadVariable(datasource),
		acm.GetTypeVariable(datasource),

		withWorkloadCPUUsageGroup(datasource),
		withWorkloadCPUQuotaGroup(datasource),
		withWorkloadMemoryUsageGroup(datasource),
		withWorkloadMemoryQuotaGroup(datasource),
	)
}
