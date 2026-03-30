package hosted_control_plane

import (
	"fmt"

	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	v1Common "github.com/perses/perses/pkg/model/api/v1/common"
	dashboardModel "github.com/perses/perses/pkg/model/api/v1/dashboard"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/hosted-control-plane"
)

func addCustomPanelGroup(title string, positions []gridItem, panelOpts ...panelgroup.Option) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		pg, err := panelgroup.New(title, panelOpts...)
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

			gi := positions[i]
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
					Title:    title,
					Collapse: &dashboardModel.GridLayoutCollapse{Open: true},
				},
				Items: gridItems,
			},
		})

		return nil
	}
}

func withRequestBasedCapacityGroup(datasource string) dashboard.Option {
	return addCustomPanelGroup(
		"Estimated capacity based on HCP resource requests",
		[]gridItem{
			{x: 0, y: 0, w: 12, h: 12},
			{x: 12, y: 0, w: 12, h: 7},
			{x: 12, y: 7, w: 12, h: 5},
		},
		panels.RequestBasedLimitEstimation(),
		panels.WorkerNodeCapacities(datasource),
		panels.NumberOfHCPsRequestBased(datasource),
	)
}

func withQPSBasedCapacityGroup(datasource string) dashboard.Option {
	return addCustomPanelGroup(
		"Estimated capacity based on API server query (QPS)",
		[]gridItem{
			{x: 0, y: 0, w: 12, h: 14},
			{x: 12, y: 0, w: 12, h: 6},
			{x: 12, y: 6, w: 12, h: 8},
		},
		panels.LoadBasedLimitEstimation(),
		panels.QPSSettings(datasource),
		panels.NumberOfHCPsQPSBased(datasource),
	)
}

func withHCPListGroup(datasource string) dashboard.Option {
	return addCustomPanelGroup(
		"Hosted Control Planes List",
		[]gridItem{
			{x: 0, y: 0, w: 24, h: 7},
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
