package networking

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/networking"
)

func withNodeCurrentBandwidthGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Current Bandwidth",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 12, H: 9},
			{X: 12, Y: 0, W: 12, H: 9},
			{X: 0, Y: 9, W: 24, H: 9},
		},
		panels.NodeCurrentBytesReceived(datasource),
		panels.NodeCurrentBytesTransmitted(datasource),
		panels.NodeCurrentStatus(datasource),
	)
}

func withNodeErrorsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Errors",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.NodeReceivedPacketsDropped(datasource),
		panels.NodeTransmittedPacketsDropped(datasource),
		panels.NodeTCPRetransmits(datasource),
		panels.NodeTCPSynRetransmits(datasource),
	)
}

func BuildNetworkingNode(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-networking-node",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Networking / Node"),

		acm.GetClusterVariable(datasource),
		acm.AddTextVariable("interval", "4h", "Interval"),
		acm.AddTextVariable("resolution", "5m", "Resolution"),

		withNodeCurrentBandwidthGroup(datasource),
		withNodeErrorsGroup(datasource),
	)
}
