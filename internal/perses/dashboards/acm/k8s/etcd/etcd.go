package etcd

import (
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/etcd"
)

func withOverviewGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Overview",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 6, H: 7},
			{X: 6, Y: 0, W: 10, H: 7},
			{X: 16, Y: 0, W: 8, H: 7},
		},
		panels.Up(datasource),
		panels.RPCRate(datasource),
		panels.ActiveStreams(datasource),
	)
}

func withStorageGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Storage",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(7),
		panels.DBSize(datasource),
		panels.DiskSyncDuration(datasource),
		panels.Memory(datasource),
	)
}

func withNetworkGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Network",
		panelgroup.PanelsPerLine(4),
		panelgroup.PanelHeight(7),
		panels.ClientTrafficIn(datasource),
		panels.ClientTrafficOut(datasource),
		panels.PeerTrafficIn(datasource),
		panels.PeerTrafficOut(datasource),
	)
}

func withRaftGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Raft",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(7),
		panels.RaftProposals(datasource),
		panels.LeaderElections(datasource),
	)
}

func BuildETCDOverview(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-etcd",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / etcd Cluster"),

		acm.GetClusterVariable(datasource),

		withOverviewGroup(datasource),
		withStorageGroup(datasource),
		withNetworkGroup(datasource),
		withRaftGroup(datasource),
	)
}
