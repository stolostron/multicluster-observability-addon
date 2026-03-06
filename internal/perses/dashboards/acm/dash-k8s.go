package acm

import (
	dashboards "github.com/perses/community-mixins/pkg/dashboards"
	k8sEtcd "github.com/perses/community-mixins/pkg/dashboards/etcd"
	k8sApiServer "github.com/perses/community-mixins/pkg/dashboards/kubernetes/apiserver"
	k8sComputeResources "github.com/perses/community-mixins/pkg/dashboards/kubernetes/compute_resources"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/model/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	// These flags are required by the github.com/perses/community-mixins/pkg/dashboards library.
	// They are looked up in NewExec() which is called by NewDashboardWriter().
	if flag.Lookup("output") == nil {
		flag.String("output", "", "output format of the dashboard exec")
	}
	if flag.Lookup("output-dir") == nil {
		flag.String("output-dir", "", "output directory of the dashboard exec")
	}
}

func GetClusterVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("name",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(
					promql.SetLabelMatchersV2(
						vector.New(vector.WithMetricName("acm_managed_cluster_labels{openshiftVersion_major!=\"3\"}")),
						[]*labels.Matcher{},
					).Pretty(0),
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
					promql.SetLabelMatchersV2(
						vector.New(vector.WithMetricName("kube_pod_info")),
						[]*labels.Matcher{{Name: "cluster", Type: labels.MatchEqual, Value: "$cluster"}},
					).Pretty(0),
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
					promql.SetLabelMatchersV2(
						vector.New(vector.WithMetricName("kube_pod_info")),
						[]*labels.Matcher{{Name: "cluster", Type: labels.MatchEqual, Value: "$cluster"}},
					).Pretty(0),
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
					promql.SetLabelMatchersV2(
						vector.New(vector.WithMetricName("kube_pod_info")),
						[]*labels.Matcher{
							{Name: "cluster", Type: labels.MatchEqual, Value: "$cluster"},
							{Name: "namespace", Type: labels.MatchEqual, Value: "$namespace"},
						},
					).Pretty(0),
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
					promql.SetLabelMatchersV2(
						vector.New(vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel")),
						[]*labels.Matcher{
							{Name: "cluster", Type: labels.MatchEqual, Value: "$cluster"},
							{Name: "namespace", Type: labels.MatchEqual, Value: "$namespace"},
						},
					).Pretty(0),
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
					promql.SetLabelMatchersV2(
						vector.New(vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel")),
						[]*labels.Matcher{
							{Name: "cluster", Type: labels.MatchEqual, Value: "$cluster"},
							{Name: "namespace", Type: labels.MatchEqual, Value: "$namespace"},
							{Name: "workload", Type: labels.MatchEqual, Value: "$workload"},
						},
					).Pretty(0),
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
					promql.SetLabelMatchersV2(
						vector.New(vector.WithMetricName("process_resident_memory_bytes")),
						[]*labels.Matcher{{Name: "cluster", Type: labels.MatchEqual, Value: "$cluster"}},
					).Pretty(0),
				),
				dashboards.AddVariableDatasource(datasource),
			),
			listVar.DisplayName("instance"),
		),
	)
}

// Upstream dashboards imported from the community-dashboards repository. https://github.com/perses/community-dashboards/tree/main/pkg/dashboards/kubernetes
func BuildK8sDashboards(project string, datasource string, clusterLabelName string) (obj []runtime.Object, err error) {
	dashboardWriter := dashboards.NewDashboardWriter()

	dashboardVars := []dashboard.Option{
		GetClusterVariable(datasource),
		GetNodeVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesNodeResourcesOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardWriter.Add(k8sComputeResources.BuildKubernetesMultiClusterOverview(project, datasource, clusterLabelName, ))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetNamespaceVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesNamespaceOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetNamespaceVariable(datasource),
		GetPodVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesPodOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetNamespaceVariable(datasource),
		GetWorkloadVariable(datasource),
		GetTypeVariable(datasource),
	}
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesWorkloadOverview(project, datasource, clusterLabelName, dashboardVars...))
	dashboardWriter.Add(k8sComputeResources.BuildKubernetesWorkloadNamespaceOverview(project, datasource, clusterLabelName, dashboardVars...))

	dashboardVars = []dashboard.Option{
		GetClusterVariable(datasource),
		GetInstanceVariable(datasource),
	}
	dashboardWriter.Add(k8sApiServer.BuildAPIServerOverview(project, datasource, clusterLabelName, dashboardVars...))

	objs := dashboardWriter.OperatorResources()
	return objs, nil
}

func BuildETCDDashboards(project string, datasource string, clusterLabelName string) (obj []runtime.Object, err error) {
	dashboardWriter := dashboards.NewDashboardWriter()
	dashboardWriter.Add(k8sEtcd.BuildETCDOverview(project, datasource, clusterLabelName))
	objs := dashboardWriter.OperatorResources()
	return objs, nil
}
