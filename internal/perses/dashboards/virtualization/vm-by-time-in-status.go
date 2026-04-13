package virtualization

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

func withTimeInStatusStatsAndTable(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Time In Status",
		[]GridItem{
			{X: 0, Y: 0, W: 8, H: 3},
			{X: 8, Y: 0, W: 8, H: 3},
			{X: 16, Y: 0, W: 8, H: 3},
			{X: 0, Y: 3, W: 24, H: 16},
		},
		panels.TotalAllocatedCPU(datasource),
		panels.TotalAllocatedMemory(datasource),
		panels.TotalAllocatedDisk(datasource),
		panels.TimeInStatusTable(datasource, project),
	)
}

func BuildVMByTimeInStatus(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-virtual-machines-by-time-in-status",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Virtual Machines Time in Status"),

		VMClusterVariableMulti(datasource),
		VMNamespaceVariable(datasource),
		VMNameVariable(datasource),
		VMStatusVariableStaticSingleSelect(),
		AddTextVariable("days_in_status_gt", "0", "Days in Status >",
			"Filter the Virtual Machines that are in the specific status for more than the selected number of days"),
		AddTextVariable("days_in_status_lt", "1000", "Days in Status <",
			"Filter the Virtual Machines that are in the specific status for less than the selected number of days"),

		withTimeInStatusStatsAndTable(datasource, project),
	)
}
