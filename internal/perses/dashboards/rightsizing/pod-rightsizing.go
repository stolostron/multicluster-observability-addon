package rightsizing

import (
	"time"

	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	staticListVar "github.com/perses/plugins/staticlistvariable/sdk/go"
	panels "github.com/stolostron/multicluster-observability-addon/internal/perses/panels/rightsizing"
	acmHelpers "github.com/stolostron/multicluster-observability-addon/pkg/perses/dashboards/acm"
)

func BuildPodDetail(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-pod-detail",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing Pod Detail"),
		dashboard.Duration(time.Hour*24*7),
		dashboard.AddVariable("days", listVar.List(staticListVar.StaticList(staticListVar.Values("1d", "2d", "5d", "10d", "30d", "60d", "90d")), listVar.DisplayName("Aggregation"), listVar.DefaultValue("10d"))),
		dashboard.AddVariable("cluster", listVar.List(labelValuesVar.PrometheusLabelValues("cluster", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers("acm_rs:pod:info", []promql.LabelMatcher{}))), listVar.DisplayName("Cluster"))),
		dashboard.AddVariable("namespace", listVar.List(labelValuesVar.PrometheusLabelValues("namespace", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:info{cluster="$cluster"}`, []promql.LabelMatcher{}))), listVar.DisplayName("Namespace"))),
		dashboard.AddVariable("pod", listVar.List(labelValuesVar.PrometheusLabelValues("pod", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:info{cluster="$cluster",namespace="$namespace"}`, []promql.LabelMatcher{}))), listVar.DisplayName("Pod"))),
		dashboard.AddVariable("workload", listVar.List(labelValuesVar.PrometheusLabelValues("workload", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:info{cluster="$cluster",namespace="$namespace",pod="$pod"}`, []promql.LabelMatcher{}))), listVar.DisplayName("Workload"), listVar.DefaultValue("$__all"), listVar.AllowAllValue(true), listVar.AllowMultiple(false))),
		dashboard.AddVariable("workload_type", listVar.List(labelValuesVar.PrometheusLabelValues("workload_type", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:info{cluster="$cluster",namespace="$namespace",pod="$pod"}`, []promql.LabelMatcher{}))), listVar.DisplayName("Workload Type"), listVar.DefaultValue("$__all"), listVar.AllowAllValue(true), listVar.AllowMultiple(false))),
		dashboard.AddVariable("cpu_profile", listVar.List(labelValuesVar.PrometheusLabelValues("profile", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:cpu_usage{cluster="$cluster"}`, []promql.LabelMatcher{}))), listVar.DisplayName("CPU Profile"), listVar.DefaultValue("Max OverAll"))),
		dashboard.AddVariable("memory_profile", listVar.List(labelValuesVar.PrometheusLabelValues("profile", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:memory_usage{cluster="$cluster"}`, []promql.LabelMatcher{}))), listVar.DisplayName("Memory Profile"), listVar.DefaultValue("Max OverAll"))),

		acmHelpers.AddCustomPanelGroup("",
			[]acmHelpers.GridItem{{X: 17, Y: 0, W: 7, H: 2}},
			panels.PodBackToMainDashboardPanel(datasource, project),
		),

		acmHelpers.AddCustomPanelGroup("CPU",
			[]acmHelpers.GridItem{
				{X: 0, Y: 0, W: 5, H: 4},
				{X: 5, Y: 0, W: 5, H: 4},
				{X: 10, Y: 0, W: 5, H: 4},
				{X: 15, Y: 0, W: 5, H: 4},
				{X: 20, Y: 0, W: 4, H: 4},
				{X: 0, Y: 4, W: 24, H: 12},
			},
			panels.PodDetailCPURecommendationStatPanel(datasource),
			panels.PodDetailCPUUsageStatPanel(datasource),
			panels.PodDetailCPURequestStatPanel(datasource),
			panels.PodDetailCPULimitStatPanel(datasource),
			panels.PodDetailCPUUtilizationStatPanel(datasource),
			panels.PodDetailCPUTimeSeriesPanel(datasource),
		),

		acmHelpers.AddCustomPanelGroup("Memory",
			[]acmHelpers.GridItem{
				{X: 0, Y: 0, W: 5, H: 4},
				{X: 5, Y: 0, W: 5, H: 4},
				{X: 10, Y: 0, W: 5, H: 4},
				{X: 15, Y: 0, W: 5, H: 4},
				{X: 20, Y: 0, W: 4, H: 4},
				{X: 0, Y: 4, W: 24, H: 12},
			},
			panels.PodDetailMemRecommendationStatPanel(datasource),
			panels.PodDetailMemUsageStatPanel(datasource),
			panels.PodDetailMemRequestStatPanel(datasource),
			panels.PodDetailMemLimitStatPanel(datasource),
			panels.PodDetailMemUtilizationStatPanel(datasource),
			panels.PodDetailMemTimeSeriesPanel(datasource),
		),
	)
}

// BuildPodRightSizing creates a pod-level dashboard. Pod series retain the
// namespace, pod, workload, and workload_type labels; ownerless pods simply
// render blank workload fields in the table.
func BuildPodRightSizing(project string, datasource string, _ string) (dashboard.Builder, error) {
	return dashboard.New("acm-rs-pod-overview",
		dashboard.ProjectName(project),
		dashboard.Name("ACM Right-Sizing Pods"),
		dashboard.Duration(time.Hour*24*7),
		dashboard.AddVariable("cluster", listVar.List(labelValuesVar.PrometheusLabelValues("cluster", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers("acm_rs:pod:info", []promql.LabelMatcher{}))), listVar.DisplayName("Cluster"), listVar.DefaultValue("local-cluster"))),
		dashboard.AddVariable("cpu_profile", listVar.List(labelValuesVar.PrometheusLabelValues("profile", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:cpu_usage{cluster="$cluster"}`, []promql.LabelMatcher{}))), listVar.DisplayName("CPU Profile"), listVar.DefaultValue("Max OverAll"))),
		dashboard.AddVariable("memory_profile", listVar.List(labelValuesVar.PrometheusLabelValues("profile", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:memory_usage{cluster="$cluster"}`, []promql.LabelMatcher{}))), listVar.DisplayName("Memory Profile"), listVar.DefaultValue("Max OverAll"))),
		dashboard.AddVariable("days", listVar.List(staticListVar.StaticList(staticListVar.Values("1d", "2d", "5d", "10d", "30d", "60d", "90d")), listVar.DisplayName("Days"), listVar.DefaultValue("10d"))),
		dashboard.AddVariable("namespace", listVar.List(labelValuesVar.PrometheusLabelValues("namespace", dashboards.AddVariableDatasource(datasource), labelValuesVar.Matchers(promql.SetLabelMatchers(`acm_rs:pod:info{cluster="$cluster"}`, []promql.LabelMatcher{}))), listVar.DisplayName("Namespace"), listVar.DefaultValue("$__all"), listVar.AllowAllValue(true), listVar.AllowMultiple(true))),
		dashboard.AddPanelGroup("CPU",
			panelgroup.PanelsPerLine(1),
			panelgroup.PanelHeight(10),
			panels.PodCPUTablePanel(datasource, project),
		),
		dashboard.AddPanelGroup("Memory",
			panelgroup.PanelsPerLine(1),
			panelgroup.PanelHeight(10),
			panels.PodMemTablePanel(datasource, project),
		),
	)
}
