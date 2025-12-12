package acm

import (
	"time"

	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	"github.com/perses/perses/pkg/model/api/v1/common"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm"
)

func withCPUResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("CPU",
		panelgroup.PanelsPerLine(2),
		panels.NodeCPUUtilization(datasource, labelMatcher),
		panels.NodeCPUSaturation(datasource, labelMatcher),
	)
}

func withMemoryResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Memory",
		panelgroup.PanelsPerLine(2),
		panels.NodeMemoryUtilization(datasource, labelMatcher),
		panels.NodeMemorySaturation(datasource, labelMatcher),
	)
}

func withNetworkResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Net",
		panelgroup.PanelsPerLine(2),
		panels.NodeNetworkUtilization(datasource, labelMatcher),
		panels.NodeNetworkSaturation(datasource, labelMatcher),
	)
}

func withDiskIOResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Disk IO",
		panelgroup.PanelsPerLine(2),
		panels.NodeDiskIOUtilization(datasource, labelMatcher),
		panels.NodeDiskIOSaturation(datasource, labelMatcher),
	)
}

func withDiskSpaceResourceGroup(datasource string, labelMatcher promql.LabelMatcher) dashboard.Option {
	return dashboard.AddPanelGroup("Disk Space",
		panelgroup.PanelsPerLine(1),
		panels.NodeDiskSpaceUtilization(datasource, labelMatcher),
	)
}

func withDescription(description string) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		if builder.Dashboard.Spec.Display == nil {
			builder.Dashboard.Spec.Display = &common.Display{}
		}
		builder.Dashboard.Spec.Display.Description = description
		return nil
	}
}

func BuildNodeResourceUse(project string, datasource string, clusterLabelName string) (dashboard.Builder, error) {
	clusterLabelMatcher := dashboards.GetClusterLabelMatcher(clusterLabelName)
	return dashboard.New("acm-node-rsrc-use",
		dashboard.ProjectName(project),
		dashboard.Name("USE Method / Node"),
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
				listVar.DisplayName("Cluster"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),
		dashboard.AddVariable("instance",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("instance",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"up{cluster=\"$cluster\", job=\"node-exporter\"}",
							[]promql.LabelMatcher{},
						)),
				),
				listVar.DisplayName("Instance"),
				listVar.AllowAllValue(true),
				listVar.AllowMultiple(true),
			),
		),
		withCPUResourceGroup(datasource, clusterLabelMatcher),
		withMemoryResourceGroup(datasource, clusterLabelMatcher),
		withNetworkResourceGroup(datasource, clusterLabelMatcher),
		withDiskIOResourceGroup(datasource, clusterLabelMatcher),
		withDiskSpaceResourceGroup(datasource, clusterLabelMatcher),
		dashboard.RefreshInterval(time.Minute*5),
	)
}
