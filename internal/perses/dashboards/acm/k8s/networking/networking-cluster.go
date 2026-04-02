package networking

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/networking"
)

func withClusterCurrentBandwidthGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Current Bandwidth",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 12, H: 9},
			{X: 12, Y: 0, W: 12, H: 9},
			{X: 0, Y: 9, W: 24, H: 9},
		},
		panels.ClusterCurrentBytesReceived(datasource),
		panels.ClusterCurrentBytesTransmitted(datasource),
		panels.ClusterCurrentStatus(datasource),
	)
}

func withClusterBandwidthHistoryGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Bandwidth History",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.ClusterReceiveBandwidth(datasource),
		panels.ClusterTransmitBandwidth(datasource),
	)
}

func withClusterPacketsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Packets",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.ClusterReceivedPackets(datasource),
		panels.ClusterTransmittedPackets(datasource),
	)
}

func withClusterErrorsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Errors",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.ClusterReceivedPacketsDropped(datasource),
		panels.ClusterTransmittedPacketsDropped(datasource),
		panels.ClusterTCPRetransmits(datasource),
		panels.ClusterTCPSynRetransmits(datasource),
	)
}

func BuildNetworkingCluster(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-networking-cluster",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Networking / Cluster"),

		acm.GetClusterVariable(datasource),
		acm.AddTextVariable("interval", "4h", "Interval"),
		acm.AddTextVariable("resolution", "5m", "Resolution"),

		withClusterCurrentBandwidthGroup(datasource),
		withClusterBandwidthHistoryGroup(datasource),
		withClusterPacketsGroup(datasource),
		withClusterErrorsGroup(datasource),
	)
}