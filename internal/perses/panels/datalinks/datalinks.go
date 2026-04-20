package datalinks

import (
	"fmt"

	tablePanel "github.com/perses/plugins/table/sdk/go"
)

const dashboardViewBase = "/monitoring/v2/dashboards/view"

func DashboardURL(dashboard string, params ...string) string {
	url := fmt.Sprintf("%s?dashboard=%s&project=$__project", dashboardViewBase, dashboard)
	for _, p := range params {
		url += "&" + p
	}
	return url
}

func FieldParam(varName, fieldName string) string {
	return fmt.Sprintf(`var-%s=${__data.fields["%s"]}`, varName, fieldName)
}

func StaticParam(varName, value string) string {
	return fmt.Sprintf("var-%s=%s", varName, value)
}

func NewTableLink(dashboard, fieldName, title string) *tablePanel.DataLink {
	return &tablePanel.DataLink{
		URL:   DashboardURL(dashboard, FieldParam(fieldName, fieldName)),
		Title: title,
	}
}

func NewTableLinkNewTab(dashboard, fieldName, title string) *tablePanel.DataLink {
	return &tablePanel.DataLink{
		URL:        DashboardURL(dashboard, FieldParam(fieldName, fieldName)),
		Title:      title,
		OpenNewTab: true,
	}
}

func NewTableLinkCustomVar(dashboard, varName, fieldName, title string) *tablePanel.DataLink {
	return &tablePanel.DataLink{
		URL:        DashboardURL(dashboard, FieldParam(varName, fieldName)),
		Title:      title,
		OpenNewTab: true,
	}
}
