package virtualization

import (
	"github.com/perses/community-mixins/pkg/dashboards"
	"github.com/perses/community-mixins/pkg/promql"
	"github.com/perses/perses/go-sdk/dashboard"
	listVar "github.com/perses/perses/go-sdk/variable/list-variable"
	labelValuesVar "github.com/perses/plugins/prometheus/sdk/go/variable/label-values"
	promqlVar "github.com/perses/plugins/prometheus/sdk/go/variable/promql"
)

func VMClusterVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("cluster",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers("kubevirt_vm_info"),
			),
			listVar.DisplayName("Cluster"),
			listVar.AllowAllValue(false),
			listVar.AllowMultiple(false),
		),
	)
}

func VMClusterVariableMulti(datasource string) dashboard.Option {
	return dashboard.AddVariable("cluster",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("cluster",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers("kubevirt_vm_info"),
			),
			listVar.DisplayName("Cluster"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMNamespaceVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("namespace",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("namespace",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"kubevirt_vm_info",
						[]promql.LabelMatcher{{Name: "cluster", Type: "=~", Value: "$cluster"}},
					),
				),
			),
			listVar.DisplayName("Namespace"),
			listVar.Description("Filter the virtual machine by the namespace."),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMNameVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("name",
		listVar.List(
			labelValuesVar.PrometheusLabelValues("name",
				dashboards.AddVariableDatasource(datasource),
				labelValuesVar.Matchers(
					promql.SetLabelMatchers(
						"kubevirt_vm_info",
						[]promql.LabelMatcher{
							{Name: "cluster", Type: "=~", Value: "$cluster"},
							{Name: "namespace", Type: "=~", Value: "$namespace"},
						},
					),
				),
			),
			listVar.DisplayName("VM Name"),
			listVar.Description("Filter the virtual machine by the name."),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMStatusVariableStatic() dashboard.Option {
	return AddStaticListVariable("status", "Status",
		[]StaticListValue{
			{Label: "Running", Value: "running"},
			{Label: "Stopped", Value: "non_running"},
			{Label: "Starting", Value: "starting"},
			{Label: "Migrating", Value: "migrating"},
			{Label: "Error", Value: "error"},
		},
		"$__all", true, true, ".*",
	)
}

// VMStatusVariableJoinExpr is used by service-level and time-in-status dashboards
// where status is a PromQL join expression rather than a label filter.
// The cluster filter is intentionally omitted from these join expressions:
// Perses substitutes ${status:raw} as a raw string without a second pass for
// nested variables, so $cluster inside a variable value would be sent literally
// to Prometheus and match nothing. The outer query already filters by cluster.
func VMStatusVariableJoinExpr() dashboard.Option {
	return AddStaticListVariable("status", "Status",
		[]StaticListValue{
			{Label: "All", Value: `on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{})))`},
			{Label: "Stopped", Value: `on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{status_group="non_running"}>0)))`},
			{Label: "Starting", Value: `on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{status_group="starting"}>0)))`},
			{Label: "Migrating", Value: `on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{status_group="migrating"}>0)))`},
			{Label: "Error", Value: `on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{status_group="error"}>0)))`},
			{Label: "Running", Value: `on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{status_group="running"}>0)))`},
		},
		`on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{})))`,
		false, false, "",
	)
}

func VMFlavorVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("flavor",
		listVar.List(
			promqlVar.PrometheusPromQL(
				`sum by (flavor)(kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",flavor!="<none>"} )`,
				promqlVar.LabelName("flavor"),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName("Flavor"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMWorkloadVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("workload",
		listVar.List(
			promqlVar.PrometheusPromQL(
				`sum by (workload)(kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",workload!="<none>"})`,
				promqlVar.LabelName("workload"),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName("Workload"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMInstanceTypeVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("instance_type",
		listVar.List(
			promqlVar.PrometheusPromQL(
				`sum by (instance_type)(kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",instance_type!="<none>"})`,
				promqlVar.LabelName("instance_type"),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName("Instance Type"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMPreferenceVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("preference",
		listVar.List(
			promqlVar.PrometheusPromQL(
				`sum by (preference)(kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",preference!="<none>"})`,
				promqlVar.LabelName("preference"),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName("Preference"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMGuestOSNameVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("guest_os_name",
		listVar.List(
			promqlVar.PrometheusPromQL(
				`sum by (guest_os_name)(kubevirt_vm_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_name!="<none>"} or kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_name!="<none>"})`,
				promqlVar.LabelName("guest_os_name"),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName("OS Name"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMGuestOSVersionVariable(datasource string) dashboard.Option {
	return dashboard.AddVariable("guest_os_version_id",
		listVar.List(
			promqlVar.PrometheusPromQL(
				`sum by (guest_os_version_id)(kubevirt_vm_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_version_id!="<none>"} or kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_version_id!="<none>"})`,
				promqlVar.LabelName("guest_os_version_id"),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName("OS Version"),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}
