package virtualization

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

func withInventoryGroup(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Virtual Machines Inventory",
		[]GridItem{
			{X: 0, Y: 0, W: 24, H: 24},
		},
		panels.VMInventoryTable(datasource, project),
	)
}

func BuildVMInventory(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-virtual-machines-inventory",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Virtual Machines Inventory"),

		VMClusterVariable(datasource),
		VMNamespaceVariable(datasource),
		VMNameVariable(datasource),
		VMStatusVariableStatic(),
		VMFlavorVariable(datasource),
		VMWorkloadVariable(datasource),
		VMInstanceTypeVariable(datasource),
		VMPreferenceVariable(datasource),
		VMGuestOSNameVariable(datasource),
		VMGuestOSVersionVariable(datasource),

		withInventoryGroup(datasource, project),
	)
}
