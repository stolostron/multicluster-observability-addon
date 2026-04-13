package virtualization

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/virtualization"
)

func withServiceLevelStatsAndTable(datasource, project string) dashboard.Option {
	return AddCustomPanelGroup("Service Level",
		[]GridItem{
			{X: 0, Y: 0, W: 8, H: 3},
			{X: 8, Y: 0, W: 8, H: 3},
			{X: 16, Y: 0, W: 8, H: 3},
			{X: 0, Y: 3, W: 8, H: 3},
			{X: 8, Y: 3, W: 8, H: 3},
			{X: 16, Y: 3, W: 8, H: 3},
			{X: 0, Y: 6, W: 24, H: 16},
		},
		panels.TotalUptimePercent(datasource),
		panels.TotalPlannedDowntimePercent(datasource),
		panels.TotalUnplannedDowntimePercent(datasource),
		panels.TotalUptimeHours(datasource),
		panels.TotalPlannedDowntimeHours(datasource),
		panels.TotalUnplannedDowntimeHours(datasource),
		panels.ServiceLevelTable(datasource, project),
	)
}

func BuildVMServiceLevel(project string, datasource string) (dashboard.Builder, error) {
	return dashboard.New("acm-virtual-machines-service-level",
		dashboard.ProjectName(project),
		dashboard.Name("Virtualization / Virtual Machines Service Level"),

		VMClusterVariableMulti(datasource),
		VMNamespaceVariable(datasource),
		VMNameVariable(datasource),
		VMStatusVariableJoinExpr(),

		withServiceLevelStatsAndTable(datasource, project),
	)
}
