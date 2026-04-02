package networking

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/networking"
)

func withNamespacePodsCurrentBandwidthGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Current Bandwidth",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 12, H: 9},
			{X: 12, Y: 0, W: 12, H: 9},
			{X: 0, Y: 9, W: 24, H: 9},
		},
		panels.NamespacePodsCurrentBytesReceived(datasource),
		panels.NamespacePodsCurrentBytesTransmitted(datasource),
		panels.NamespacePodsCurrentStatus(datasource),
	)
}

func withNamespacePodsBandwidthGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Bandwidth",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(9),
		panels.NamespacePodsReceiveBandwidth(datasource),
		panels.NamespacePodsTransmitBandwidth(datasource),
	)
}

func withNamespacePodsPacketsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Packets",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(10),
		panels.NamespacePodsReceivedPackets(datasource),
		panels.NamespacePodsTransmittedPackets(datasource),
	)
}

func withNamespacePodsErrorsGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Errors",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(10),
		panels.NamespacePodsReceivedPacketsDropped(datasource),
		panels.NamespacePodsTransmittedPacketsDropped(datasource),
	)
}

func BuildNetworkingNamespacePods(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-networking-namespace-pods",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / Networking / Namespace (Pods)"),

		acm.GetClusterVariable(datasource),
		acm.GetNamespaceVariable(datasource),
		acm.AddTextVariable("resolution", "5m", "Resolution"),
		acm.AddTextVariable("interval", "4h", "Interval"),

		withNamespacePodsCurrentBandwidthGroup(datasource),
		withNamespacePodsBandwidthGroup(datasource),
		withNamespacePodsPacketsGroup(datasource),
		withNamespacePodsErrorsGroup(datasource),
	)
}
