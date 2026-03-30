package hosted_control_plane

import (
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/promql/parser"
)

var HCPPanelQueries = map[string]parser.Expr{
	// HCP Overview queries
	"WorkerNodeCapacities": vector.New(
		vector.WithMetricName("mce_hs_addon_worker_node_resource_capacities_gauge"),
	),
	"HCPStatusUnavailable": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("mce_hs_addon_hosted_control_planes_status_gauge"),
			vector.WithLabelMatchers(
				label.New("ready").Equal("false"),
			),
		),
	),
	"HCPStatusAvailable": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("mce_hs_addon_hosted_control_planes_status_gauge"),
			vector.WithLabelMatchers(
				label.New("ready").Equal("true"),
			),
		),
	),
	"RequestBasedHCPCapacity": vector.New(
		vector.WithMetricName("mce_hs_addon_request_based_hcp_capacity_gauge"),
	),
	"QPSSettings": vector.New(
		vector.WithMetricName("mce_hs_addon_qps_gauge"),
	),
	"QPSBasedCapacityLow": vector.New(
		vector.WithMetricName("mce_hs_addon_qps_based_hcp_capacity_gauge"),
		vector.WithLabelMatchers(
			label.New("qps_rate").Equal("low"),
		),
	),
	"QPSBasedCapacityMedium": vector.New(
		vector.WithMetricName("mce_hs_addon_qps_based_hcp_capacity_gauge"),
		vector.WithLabelMatchers(
			label.New("qps_rate").Equal("medium"),
		),
	),
	"QPSBasedCapacityHigh": vector.New(
		vector.WithMetricName("mce_hs_addon_qps_based_hcp_capacity_gauge"),
		vector.WithLabelMatchers(
			label.New("qps_rate").Equal("high"),
		),
	),
	"QPSBasedCapacityAverage": vector.New(
		vector.WithMetricName("mce_hs_addon_qps_based_hcp_capacity_gauge"),
		vector.WithLabelMatchers(
			label.New("qps_rate").Equal("average"),
		),
	),
	"HCPList": vector.New(
		vector.WithMetricName("mce_hs_addon_hosted_control_planes_status_gauge"),
	),

	// HCP Resources queries
	"HCPPodCount": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_info"),
			vector.WithLabelMatchers(
				label.New("namespace").Equal("$hcp_namespace"),
			),
		),
	),
	"HCPCPUUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
			vector.WithLabelMatchers(
				label.New("namespace").Equal("$hcp_namespace"),
			),
		),
	).By("pod"),
	"HCPCPURequestsPercent": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum"),
				vector.WithLabelMatchers(
					label.New("namespace").Equal("$hcp_namespace"),
				),
			),
		).By("namespace"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("resource").Equal("cpu"),
					label.New("namespace").Equal("$hcp_namespace"),
				),
			),
		).By("namespace"),
	),
	"HCPCPURequests": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("namespace").Equal("$hcp_namespace"),
				label.New("resource").Equal("cpu"),
			),
		),
	),
	"HCPCPUUsage": vector.New(
		vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum"),
		vector.WithLabelMatchers(
			label.New("namespace").Equal("$hcp_namespace"),
		),
	),
	"HCPMemoryRequestsPercent": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_rss"),
				vector.WithLabelMatchers(
					label.New("namespace").Equal("$hcp_namespace"),
					label.New("container").NotEqual(""),
				),
			),
		).By("namespace"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("namespace").Equal("$hcp_namespace"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("namespace"),
	),
	"HCPMemoryUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("job").Equal("kubelet"),
				label.New("namespace").EqualRegexp("$hcp_namespace"),
			),
		),
	).By("pod"),
	"HCPMemoryRequests": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("namespace").Equal("$hcp_namespace"),
				label.New("resource").Equal("memory"),
			),
		),
	),
	"HCPMemoryUsage": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("namespace").Equal("$hcp_namespace"),
				label.New("container").NotEqual(""),
			),
		),
	),
}