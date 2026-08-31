package hosted_control_plane

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/hosted-control-plane"
)

func withCPUResourceGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("CPU",
		panelgroup.PanelsPerLine(3),
		panels.HCPCPUUsageGraph(datasource),
		panels.HCPCPURequestsPercent(datasource),
		panels.HCPCPURequests(datasource),
		panels.HCPCPUUsage(datasource),
	)
}

func withMemoryResourceGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Memory",
		panelgroup.PanelsPerLine(3),
		panels.HCPMemoryRequestsPercent(datasource),
		panels.HCPMemoryUsageGraph(datasource),
		panels.HCPMemoryRequests(datasource),
		panels.HCPMemoryUsage(datasource),
	)
}

func BuildACMHCPResources(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-hcp-resources",
		dashboard.ProjectName(project),
		dashboard.Name("ACM - Resources - Hosted Control Plane"),

		dashboard.AddVariable("hcp_namespace",
			listVar.List(
				labelValuesVar.PrometheusLabelValues("hcp_namespace",
					dashboards.AddVariableDatasource(datasource),
					labelValuesVar.Matchers(
						promql.SetLabelMatchers(
							"mce_hs_addon_hosted_control_planes_status_gauge",
							[]promql.LabelMatcher{},
						),
					),
				),
				listVar.DisplayName("HCP Namespace"),
				listVar.AllowAllValue(false),
				listVar.AllowMultiple(false),
			),
		),

		dashboard.AddPanelGroup("Overview",
			panelgroup.PanelsPerLine(1),
			panels.HCPPodCount(datasource),
		),

		withCPUResourceGroup(datasource),
		withMemoryResourceGroup(datasource),
	)
}