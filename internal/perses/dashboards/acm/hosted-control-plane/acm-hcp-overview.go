package hosted_control_plane

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/hosted-control-plane"
)

func withRequestBasedCapacityGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Estimated capacity based on HCP resource requests",
		panelgroup.PanelsPerLine(3),
		panels.RequestBasedLimitEstimation(),
		panels.WorkerNodeCapacities(datasource),
		panels.NumberOfHCPsRequestBased(datasource),
	)
}

func withQPSBasedCapacityGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Estimated capacity based on API server query (QPS)",
		panelgroup.PanelsPerLine(3),
		panels.LoadBasedLimitEstimation(),
		panels.QPSSettings(datasource),
		panels.NumberOfHCPsQPSBased(datasource),
	)
}

func withHCPListGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Hosted Control Planes List",
		panelgroup.PanelsPerLine(1),
		panels.HCPList(datasource),
	)
}

func BuildACMHCPOverview(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-hcp-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM - Hosted Control Planes Overview"),

		withRequestBasedCapacityGroup(datasource),
		withQPSBasedCapacityGroup(datasource),
		withHCPListGroup(datasource),
	)
}