package apiserver

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	acm "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/acm"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/acm/k8s/apiserver"
)

func getInstanceVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("instance",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("instance",
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"process_resident_memory_bytes",
						[]promql.LabelMatcher{{Name: "cluster", Type: "=", Value: "$cluster"}},
					),
				),
				dashboards.AddVariableDatasource(datasource),
			),
			listVar.DisplayName("instance"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(false),
		),
	)
}

func withOverviewGroup(datasource string) dashboard.Option {
	return acm.AddCustomPanelGroup(
		"Overview",
		[]acm.GridItem{
			{X: 0, Y: 0, W: 4, H: 7},
			{X: 4, Y: 0, W: 10, H: 7},
			{X: 14, Y: 0, W: 10, H: 7},
		},
		panels.APIServersUp(datasource),
		panels.RequestLatency(datasource),
		panels.RequestRateByHTTPCode(datasource),
	)
}

func withWorkQueueGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Work Queue",
		panelgroup.PanelsPerLine(1),
		panelgroup.PanelHeight(9),
		panels.WorkQueueLatency(datasource),
	)
}

func withSaturationGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Saturation",
		panelgroup.PanelsPerLine(2),
		panelgroup.PanelHeight(7),
		panels.QueueDepth(datasource),
		panels.QueueAddRate(datasource),
	)
}

func withResourcesGroup(datasource string) dashboard.Option {
	return dashboard.AddPanelGroup("Resources",
		panelgroup.PanelsPerLine(3),
		panelgroup.PanelHeight(7),
		panels.Memory(datasource),
		panels.CPUUsage(datasource),
		panels.Goroutines(datasource),
	)
}

func BuildAPIServerOverview(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("k8s-apiserver",
		dashboard.ProjectName(project),
		dashboard.Name("Kubernetes / API server"),

		acm.GetClusterVariable(datasource),
		getInstanceVariable(datasource),

		withOverviewGroup(datasource),
		withWorkQueueGroup(datasource),
		withSaturationGroup(datasource),
		withResourcesGroup(datasource),
	)
}
