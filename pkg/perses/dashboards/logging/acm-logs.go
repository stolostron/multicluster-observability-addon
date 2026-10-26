package logging

import (
	"github.com/perses/perses/go-sdk/dashboard"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	logstable "github.com/perses/plugins/logstable/sdk/go"
	lokilog "github.com/perses/plugins/loki/sdk/go/query/log"
)

// BuildACMLogs shows logs from namespaced infrastructure workloads forwarded
// to the default LokiStack.
func BuildACMLogs(project, datasource, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-logs",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Logs"),
		dashboard.AddPanelGroup("Logs",
			panelgroup.PanelsPerLine(1),
			panelgroup.PanelHeight(8),
			panelgroup.AddPanel("Recent infrastructure logs",
				logstable.LogsTable(),
				panel.AddQuery(
					lokilog.LokiLogQuery(`{k8s_namespace_name=~".+"}`,
						lokilog.Datasource(datasource),
					),
				),
			),
		),
	)
}
