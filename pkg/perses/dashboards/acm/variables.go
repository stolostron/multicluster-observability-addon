package acm

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
)

func GetClusterVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("name",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"acm_managed_cluster_labels{openshiftVersion_major!=\"3\"}",
						[]promql.LabelMatcher{},
					),
				),
			),
			listVar.DisplayName("cluster"),
			listVar.AllowAllValue(false),
			listVar.AllowMultiple(false),
		),
	)
}

func GetNodeVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("node",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("node",
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"kube_pod_info",
						[]promql.LabelMatcher{{Name: "cluster", Type: "=~", Value: "$cluster"}},
					),
				),
				dashboards.AddVariableDatasource(datasource),
			),
			listVar.DisplayName("node"),
		),
	)
}

func GetNamespaceVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("namespace",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("namespace",
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"kube_pod_info",
						[]promql.LabelMatcher{{Name: "cluster", Type: "=", Value: "$cluster"}},
					),
				),
				dashboards.AddVariableDatasource(datasource),
			),
			listVar.DisplayName("namespace"),
		),
	)
}

func GetPodVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("pod",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("pod",
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"kube_pod_info",
						[]promql.LabelMatcher{
							{Name: "cluster", Type: "=", Value: "$cluster"},
							{Name: "namespace", Type: "=", Value: "$namespace"},
						},
					),
				),
				dashboards.AddVariableDatasource(datasource),
			),
			listVar.DisplayName("pod"),
		),
	)
}

func GetWorkloadVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("workload",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("workload",
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"namespace_workload_pod:kube_pod_owner:relabel",
						[]promql.LabelMatcher{
							{Name: "cluster", Type: "=", Value: "$cluster"},
							{Name: "namespace", Type: "=", Value: "$namespace"},
						},
					),
				),
				dashboards.AddVariableDatasource(datasource),
			),
			listVar.DisplayName("workload"),
		),
	)
}

func GetTypeVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("type",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("workload_type",
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"namespace_workload_pod:kube_pod_owner:relabel",
						[]promql.LabelMatcher{
							{Name: "cluster", Type: "=", Value: "$cluster"},
							{Name: "namespace", Type: "=", Value: "$namespace"},
							{Name: "workload", Type: "=", Value: "$workload"},
						},
					),
				),
				dashboards.AddVariableDatasource(datasource),
			),
			listVar.DisplayName("type"),
		),
	)
}

func GetInstanceVariable(datasource string) dashboard.Option {
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
		),
	)
}
