package acm

import (
	"fmt"

	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	v1Common "github.com/perses/perses/pkg/model/api/v1/common"
	dashboardModel "github.com/perses/perses/pkg/model/api/v1/dashboard"
	"github.com/perses/perses/pkg/model/api/v1/variable"
)

type GridItem struct {
	X, Y, W, H int
}

func AddTextVariable(name string, value string, displayName string) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		display := &variable.Display{Name: displayName}
		builder.Dashboard.Spec.Variables = append(builder.Dashboard.Spec.Variables, dashboardModel.Variable{
			Kind: variable.KindText,
			Spec: &dashboardModel.TextVariableSpec{
				TextSpec: variable.TextSpec{
					Value:   value,
					Display: display,
				},
				Name: name,
			},
		})
		return nil
	}
}

func AddCustomPanelGroup(title string, positions []GridItem, panelOpts ...panelgroup.Option) dashboard.Option {
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

		for i := range pg.Panels {
			panelRef := fmt.Sprintf("%d_%d", layoutIdx, i)
			builder.Dashboard.Spec.Panels[panelRef] = &pg.Panels[i]

			gi := positions[i]
			gridItems = append(gridItems, dashboardModel.GridItem{
				X:      gi.X,
				Y:      gi.Y,
				Width:  gi.W,
				Height: gi.H,
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
