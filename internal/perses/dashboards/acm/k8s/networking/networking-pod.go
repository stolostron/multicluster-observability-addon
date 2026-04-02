package networking

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/networking"
)

func withPodCurrentBandwidthGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Current Bandwidth",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(9),
		panels.PodCurrentBytesReceived(datasource),
		panels.PodCurrentBytesTransmitted(datasource),
	)
}

func withPodBandwidthGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Bandwidth",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(9),
		panels.PodReceiveBandwidth(datasource),
		panels.PodTransmitBandwidth(datasource),
	)
}

func withPodPacketsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Packets",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(10),
		panels.PodReceivedPackets(datasource),
		panels.PodTransmittedPackets(datasource),
	)
}

func withPodErrorsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Errors",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(10),
		panels.PodReceivedPacketsDropped(datasource),
		panels.PodTransmittedPacketsDropped(datasource),
	)
}

func BuildNetworkingPod(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-networking-pod",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Networking / Pod"),

		acm.GetClusterVariable(datasource),
		acm.GetNamespaceVariable(datasource),
		acm.GetPodVariable(datasource),
		acm.AddTextVariable("resolution", "5m", "Resolution"),
		acm.AddTextVariable("interval", "4h", "Interval"),

		withPodCurrentBandwidthGroup(datasource),
		withPodBandwidthGroup(datasource),
		withPodPacketsGroup(datasource),
		withPodErrorsGroup(datasource),
	)
}
