// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	commonSdk "github.com/perses/perses/go-sdk/common"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/plugins/prometheus/sdk/go/query"
	statPanel "github.com/perses/plugins/statchart/sdk/go"
	tablePanel "github.com/perses/plugins/table/sdk/go"
)

// StatPanelConfig defines configuration for building a stat panel
type StatPanelConfig struct {
	Title       string
	Description string
	Query       string
	Unit        *string
	Decimals    int
	FontSize    int
	Thresholds  *commonSdk.Thresholds
}

// BuildStatPanel creates a stat panel with the given configuration.
// This provides a reusable way to create consistent stat panels across dashboards.
func BuildStatPanel(datasourceName string, cfg StatPanelConfig) panelgroup.Option {
	opts := []statPanel.Option{
		statPanel.Format(commonSdk.Format{
			Unit:          cfg.Unit,
			DecimalPlaces: cfg.Decimals,
		}),
		statPanel.ValueFontSize(cfg.FontSize),
	}
	if cfg.Thresholds != nil {
		opts = append(opts, statPanel.Thresholds(*cfg.Thresholds))
	}
	return panelgroup.AddPanel(cfg.Title,
		panel.Description(cfg.Description),
		statPanel.Chart(opts...),
		panel.AddQuery(
			query.PromQL(cfg.Query, dashboards.AddQueryDataSource(datasourceName)),
		),
	)
}

// DataLink adds a clickable hyperlink to a table column for drill-down navigation.
// The Go SDK's tablePanel.ColumnSettings does not include this field, so we extend
// it with a wrapper type that serializes the extra `dataLink` JSON property.
type DataLink struct {
	OpenNewTab bool   `json:"openNewTab"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

// ColumnSettingsWithLink extends the SDK ColumnSettings with an optional DataLink.
type ColumnSettingsWithLink struct {
	tablePanel.ColumnSettings
	DataLink *DataLink `json:"dataLink,omitempty"`
}

// TablePluginSpec mirrors the SDK's PluginSpec but uses ColumnSettingsWithLink.
type TablePluginSpec struct {
	Density         tablePanel.Density        `json:"density,omitempty"`
	ColumnSettings  []ColumnSettingsWithLink  `json:"columnSettings,omitempty"`
	CellSettings    []tablePanel.CellSettings `json:"cellSettings,omitempty"`
	Transforms      []commonSdk.Transform     `json:"transforms,omitempty"`
	EnableFiltering bool                      `json:"enableFiltering,omitempty"`
}

// TableWithLinks creates a panel.Option that builds a Table plugin spec
// supporting dataLink on columns (which the upstream SDK does not expose).
func TableWithLinks(spec TablePluginSpec) panel.Option {
	return func(builder *panel.Builder) error {
		builder.Spec.Plugin.Kind = "Table"
		builder.Spec.Plugin.Spec = spec
		return nil
	}
}
