package acm

import (
	"time"

	"github.com/perses/community-dashboards/pkg/dashboards"
	"github.com/perses/community-dashboards/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	"github.com/perses/perses/pkg/model/api/v1/common"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm"
)

func withClusterCPUResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("CPU",
		panelgroup.PanelsPerLine(2),
		panels.CPUUtilization(datasource, labelMatcher),
		panels.CPUSaturation(datasource, labelMatcher),
	)
}

func withClusterMemoryResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Memory",
		panelgroup.PanelsPerLine(2),
		panels.MemoryUtilization(datasource, labelMatcher),
		panels.MemorySaturation(datasource, labelMatcher),
	)
}

func withClusterNetworkResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Network",
		panelgroup.PanelsPerLine(2),
		panels.NetworkUtilization(datasource, labelMatcher),
		panels.NetworkSaturation(datasource, labelMatcher),
	)
}

func withClusterDiskIOResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Disk IO",
		panelgroup.PanelsPerLine(2),
		panels.DiskIOUtilization(datasource, labelMatcher),
		panels.DiskIOSaturation(datasource, labelMatcher),
	)
}

func withClusterDiskSpaceResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Disk Space",
		panelgroup.PanelsPerLine(1),
		panels.DiskSpaceUtilization(datasource, labelMatcher),
	)
}

func withClusterDescription(description string) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		if builder.Dashboard.Spec.Display == nil {
			builder.Dashboard.Spec.Display = &common.Display{}
		}
		builder.Dashboard.Spec.Display.Description = description
		return nil
	}
}

func BuildClusterResourceUse(project string, datasource string, clusterLabelName string) dashboards.DashboardResult {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
	return dashboards.NewDashboardResult(
		dashboard.New("acm-cluster-rsrc-use",
			dashboard.ProjectName(project),
			dashboard.Name("USE Method / Cluster"),
			withDescription("http://www.brendangregg.com/USEmethod/use-linux.html"),
			dashboard.Duration(time.Hour*3),
			dashboard.AddVariable("cluster",
				listVar.List(
					labelValuesVar.PrometheusLabelValues("name",
						dashboards.AddVariableDatasource(datasource),
						labelValuesVar.Matchers(
							promql.SetLabelMatchers(
								"acm_managed_cluster_labels{openshiftVersion_major!=\"3\"}",
								[]promql.LabelMatcher{},
							)),
					),
					listVar.DisplayName("cluster"),
					listVar.AllowAllValue(true),
					listVar.AllowMultiple(true),
				),
			),
			withClusterCPUResourceGroup(datasource, clusterLabelMatcher),
			withClusterMemoryResourceGroup(datasource, clusterLabelMatcher),
			withClusterNetworkResourceGroup(datasource, clusterLabelMatcher),
			withClusterDiskIOResourceGroup(datasource, clusterLabelMatcher),
			withClusterDiskSpaceResourceGroup(datasource, clusterLabelMatcher),
			dashboard.RefreshInterval(time.Minute*5),
		),
	).Component("acm")
}
