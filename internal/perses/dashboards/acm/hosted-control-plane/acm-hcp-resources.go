package hosted_control_plane

import (
	"fmt"

	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	v1Common "github.com/perses/perses/pkg/model/api/v1/common"
	dashboardModel "github.com/perses/perses/pkg/model/api/v1/dashboard"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/hosted-control-plane"
)

type gridItem struct {
	x, y, w, h int
}

var hcpResourcesGridItems = []gridItem{
	{x: 0, y: 0, w: 3, h: 4},    // 0: Number of pods
	{x: 3, y: 0, w: 21, h: 16},  // 1: CPU usage graph
	{x: 0, y: 4, w: 3, h: 4},    // 2: CPU Requests %
	{x: 0, y: 8, w: 3, h: 4},    // 3: CPU Requests
	{x: 0, y: 12, w: 3, h: 4},   // 4: CPU Usage
	{x: 0, y: 16, w: 3, h: 4},   // 5: Memory Requests %
	{x: 3, y: 16, w: 21, h: 14}, // 6: Memory Usage (w/o cache) graph
	{x: 0, y: 20, w: 3, h: 5},   // 7: Memory Requests
	{x: 0, y: 25, w: 3, h: 5},   // 8: Memory Usage
}

func withHCPResourcesLayout(datasource string) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		pg, err := panelgroup.New("Panel Group 1",
			panels.HCPPodCount(datasource),
			panels.HCPCPUUsageGraph(datasource),
			panels.HCPCPURequestsPercent(datasource),
			panels.HCPCPURequests(datasource),
			panels.HCPCPUUsage(datasource),
			panels.HCPMemoryRequestsPercent(datasource),
			panels.HCPMemoryUsageGraph(datasource),
			panels.HCPMemoryRequests(datasource),
			panels.HCPMemoryUsage(datasource),
		)
		if err != nil {
			return err
		}

		if builder.Dashboard.Spec.Panels == nil {
			builder.Dashboard.Spec.Panels = make(map[string]*v1.Panel)
		}

		layoutIdx := len(builder.Dashboard.Spec.Layouts)
		gridItems := make([]dashboardModel.GridItem, 0, len(pg.Panels))

		for i, p := range pg.Panels {
			panelRef := fmt.Sprintf("%d_%d", layoutIdx, i)
			builder.Dashboard.Spec.Panels[panelRef] = &p

			gi := hcpResourcesGridItems[i]
			gridItems = append(gridItems, dashboardModel.GridItem{
				X:      gi.x,
				Y:      gi.y,
				Width:  gi.w,
				Height: gi.h,
				Content: &v1Common.JSONRef{
					Ref: fmt.Sprintf("#/spec/panels/%s", panelRef),
				},
			})
		}

		builder.Dashboard.Spec.Layouts = append(builder.Dashboard.Spec.Layouts, dashboardModel.Layout{
			Kind: "Grid",
			Spec: dashboardModel.GridLayoutSpec{
				Display: &dashboardModel.GridLayoutDisplay{
					Title:    "Panel Group 1",
					Collapse: &dashboardModel.GridLayoutCollapse{Open: true},
				},
				Items: gridItems,
			},
		})

		return nil
	}
}

func BuildACMHCPResources(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-hcp-resources",
		dashboard.ProjectName(project),
		dashboard.Name("ACM - Resources - Hosted Control Plane"),

		dashboard.AddVariable("hcp_ns",
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

		withHCPResourcesLayout(datasource),
	)
}
