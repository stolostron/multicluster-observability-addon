package hosted_control_plane

import (
	"github.com/perses/perses/go-sdk/dashboard"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/hosted-control-plane"
)

func withRequestBasedCapacityGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Estimated capacity based on HCP resource requests",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 12, H: 12},
			{X: 12, Y: 0, W: 12, H: 7},
			{X: 12, Y: 7, W: 12, H: 5},
		},
		panels.RequestBasedLimitEstimation(),
		panels.WorkerNodeCapacities(datasource),
		panels.NumberOfHCPsRequestBased(datasource),
	)
}

func withQPSBasedCapacityGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Estimated capacity based on API server query (QPS)",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 12, H: 14},
			{X: 12, Y: 0, W: 12, H: 6},
			{X: 12, Y: 6, W: 12, H: 8},
		},
		panels.LoadBasedLimitEstimation(),
		panels.QPSSettings(datasource),
		panels.NumberOfHCPsQPSBased(datasource),
	)
}

func withHCPListGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Hosted Control Planes List",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 24, H: 7},
		},
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
