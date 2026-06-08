package virtualization

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

func withUtilizationGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Virtual Machines Utilization",
		[]GridItem{
			{X: 0, Y: 0, W: 24, H: 24},
		},
		panels.VMUtilizationTable(datasource, project),
	)
}

func BuildVMUtilization(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-virtual-machines-utilization",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Virtual Machines Utilization"),

		VMClusterVariable(datasource),
		VMNamespaceVariable(datasource),
		VMNameVariable(datasource),
		VMStatusVariableStatic(),

		withUtilizationGroup(datasource, project),
	)
}
