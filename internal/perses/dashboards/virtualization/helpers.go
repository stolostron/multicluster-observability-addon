package virtualization

import (
	"errors"
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

// AddTextVariable adds a text variable (free-form user input) to the dashboard.
func AddTextVariable(name string, value string, displayName string, description string) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		display := &variable.Display{
			Name:        displayName,
			Description: description,
		}
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

// StaticListValue represents a value in a static list variable.
type StaticListValue struct {
	Label string
	Value string
}

// AddStaticListVariable adds a list variable backed by static values.
func AddStaticListVariable(name string, displayName string, values []StaticListValue, defaultValue string, allowAll bool, allowMultiple bool, customAllValue string) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		staticValues := make([]map[string]any, 0, len(values))
		for _, v := range values {
			if v.Label == "" {
				staticValues = append(staticValues, map[string]any{"value": v.Value})
			} else {
				staticValues = append(staticValues, map[string]any{"label": v.Label, "value": v.Value})
			}
		}

		display := &variable.Display{
			Name: displayName,
		}

		spec := dashboardModel.ListVariableSpec{
			ListSpec: variable.ListSpec{
				Display:        display,
				AllowAllValue:  allowAll,
				AllowMultiple:  allowMultiple,
				CustomAllValue: customAllValue,
				Plugin: v1Common.Plugin{
					Kind: "StaticListVariable",
					Spec: map[string]any{
						"values": staticValues,
					},
				},
			},
			Name: name,
		}

		if allowMultiple && defaultValue != "" {
			spec.DefaultValue = &variable.DefaultValue{
				SliceValues: []string{defaultValue},
			}
		} else if defaultValue != "" {
			spec.DefaultValue = &variable.DefaultValue{
				SingleValue: defaultValue,
			}
		}

		builder.Dashboard.Spec.Variables = append(builder.Dashboard.Spec.Variables, dashboardModel.Variable{
			Kind: variable.KindList,
			Spec: &spec,
		})
		return nil
	}
}

var errPositionsMismatch = errors.New("grid positions count does not match panel count")

// addCustomPanelGroupImpl is the shared implementation for building a panel group
// with explicit grid positions and configurable collapse state.
func addCustomPanelGroupImpl(title string, collapseOpen bool, positions []GridItem, panelOpts ...panelgroup.Option) dashboard.Option {
	return func(builder *dashboard.Builder) error {
		pg, err := panelgroup.New(title, panelOpts...)
		if err != nil {
			return err
		}

		if len(positions) < len(pg.Panels) {
			return fmt.Errorf("%w: panel group %q has %d panels but only %d grid positions", errPositionsMismatch, title, len(pg.Panels), len(positions))
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
					Collapse: &dashboardModel.GridLayoutCollapse{Open: collapseOpen},
				},
				Items: gridItems,
			},
		})

		return nil
	}
}

// AddCustomPanelGroup adds a panel group with explicit grid positions for each panel.
func AddCustomPanelGroup(title string, positions []GridItem, panelOpts ...panelgroup.Option) dashboard.Option {
	return addCustomPanelGroupImpl(title, true, positions, panelOpts...)
}
