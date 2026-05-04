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

var vmStatusStaticValues = []StaticListValue{
	{Label: "Running", Value: "running"},
	{Label: "Stopped", Value: "non_running"},
	{Label: "Starting", Value: "starting"},
	{Label: "Migrating", Value: "migrating"},
	{Label: "Error", Value: "error"},
}

// VMStatusVariableStatic is a multi-select status filter used by inventory and
// utilization dashboards where $status is used as a regex label matcher.
func VMStatusVariableStatic() dashboard.Option {
	return AddStaticListVariable("status", "Status", "Filter virtual machines by status",
		vmStatusStaticValues, "$__all", true, true, ".*",
	)
}

// VMStatusVariableStaticSingleSelect is a single-select status filter used by
// the time-in-status dashboard, where $status is matched against status_group
// and multi-select would produce an invalid "a,b" regex.
func VMStatusVariableStaticSingleSelect() dashboard.Option {
	return AddStaticListVariable("status", "Status", "Filter virtual machines by status",
		vmStatusStaticValues, "$__all", true, false, ".*",
	)
}

// VMStatusVariableJoinExpr is used by service-level and time-in-status dashboards
// where status is a PromQL join expression rather than a label filter.
// The cluster filter is intentionally omitted from these join expressions:
// Perses substitutes ${status:raw} as a raw string without a second pass for
// nested variables, so $cluster inside a variable value would be sent literally
// to Prometheus and match nothing. The outer query already filters by cluster.
//
// See: https://perses.dev/perses/docs/api/variable/
// Upstream limitation: https://github.com/perses/perses/issues/2016
func VMStatusVariableJoinExpr() dashboard.Option {
	return AddStaticListVariable("status", "Status", "Filter virtual machines by status",
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

// vmiLabelListVariable builds a multi-select variable that lists distinct values
// of a VMI label from kubevirt_vmi_info. expr overrides the default
// per-label query when the variable needs a custom metric union (e.g. guest OS
// fields that also appear in kubevirt_vm_info).
func vmiLabelListVariable(name, displayName, datasource, expr string) dashboard.Option {
	if expr == "" {
		expr = `sum by (` + name + `)(kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",` + name + `!="<none>"})`
	}
	return dashboard.AddVariable(name,
		listVar.List(
			promqlVar.PrometheusPromQL(
				expr,
				promqlVar.LabelName(name),
				promqlVar.Datasource(datasource),
			),
			listVar.DisplayName(displayName),
			listVar.AllowAllValue(true),
			listVar.AllowMultiple(true),
			listVar.CustomAllValue(".*"),
			listVar.DefaultValues("$__all"),
		),
	)
}

func VMFlavorVariable(datasource string) dashboard.Option {
	return vmiLabelListVariable("flavor", "Flavor", datasource, "")
}

func VMWorkloadVariable(datasource string) dashboard.Option {
	return vmiLabelListVariable("workload", "Workload", datasource, "")
}

func VMInstanceTypeVariable(datasource string) dashboard.Option {
	return vmiLabelListVariable("instance_type", "Instance Type", datasource, "")
}

func VMPreferenceVariable(datasource string) dashboard.Option {
	return vmiLabelListVariable("preference", "Preference", datasource, "")
}

func VMGuestOSNameVariable(datasource string) dashboard.Option {
	return vmiLabelListVariable("guest_os_name", "OS Name", datasource,
		`sum by (guest_os_name)(kubevirt_vm_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_name!="<none>"} or kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_name!="<none>"})`)
}

func VMGuestOSVersionVariable(datasource string) dashboard.Option {
	return vmiLabelListVariable("guest_os_version_id", "OS Version", datasource,
		`sum by (guest_os_version_id)(kubevirt_vm_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_version_id!="<none>"} or kubevirt_vmi_info{cluster=~"$cluster", namespace=~"$namespace",guest_os_version_id!="<none>"})`)
}
