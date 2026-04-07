package compute

import (
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/promql/parser"
)

// Cluster-level queries

var ClusterQueries = map[string]parser.Expr{
	// Stat panel queries
	"CPUUtilisation": promqlbuilder.Sub(
		promqlbuilder.NewNumber(1),
		vector.New(
			vector.WithMetricName("node_cpu_seconds_total:mode_idle:avg_rate5m"),
		),
	),
	"CPURequestsCommitment": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("cpu"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_node_status_allocatable"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("cpu"),
				),
			),
		),
	),
	"CPULimitsCommitment": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits:sum"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("cpu"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_node_status_allocatable"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("cpu"),
				),
			),
		),
	),
	"MemoryUtilisation": promqlbuilder.Sub(
		promqlbuilder.NewNumber(1),
		promqlbuilder.Div(
			promqlbuilder.Sum(
				vector.New(
					vector.WithMetricName(":node_memory_MemAvailable_bytes:sum"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
					),
				),
			),
			promqlbuilder.Sum(
				vector.New(
					vector.WithMetricName("kube_node_status_allocatable"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
						label.New("resource").Equal("memory"),
					),
				),
			),
		),
	),
	"MemoryRequestsCommitment": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("memory"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_node_status_allocatable"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("memory"),
				),
			),
		),
	),
	"MemoryLimitsCommitment": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits:sum"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("memory"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_node_status_allocatable"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("memory"),
				),
			),
		),
	),

	// Graph queries
	"CPUUsageByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"MemoryUsageByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("namespace"),

	// CPU Quota table queries
	"TablePodsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_info"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"TableWorkloadsByNamespace": vector.New(
		vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel:avg"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	"TableCPUUsageByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"TableCPURequestsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("namespace"),
	"TableCPURequestsPercentByNamespace": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("namespace"),
	),
	"TableCPULimitsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("namespace"),
	"TableCPULimitsPercentByNamespace": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits:sum"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("namespace"),
	),

	// Memory Quota table queries
	"TableMemoryUsageByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("namespace"),
	"TableMemoryRequestsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("namespace"),
	"TableMemoryRequestsPercentByNamespace": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_rss"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
		).By("namespace"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("namespace"),
	),
	"TableMemoryLimitsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("namespace"),
	"TableMemoryLimitsPercentByNamespace": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_rss"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
		).By("namespace"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits:sum"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("namespace"),
	),
}

// Namespace (Pods) queries

var NamespacePodsQueries = map[string]parser.Expr{
	// Stat panel queries
	"CPUUtilisationFromRequests": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
		),
	),
	"CPUUtilisationFromLimits": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
		),
	),
	"MemoryUtilisationFromRequests": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
		),
	),
	"MemoryUtilisationFromLimits": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
		),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
		),
	),

	// Graph queries
	"CPUUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
			),
		),
	).By("pod"),
	"CPUQuotaRequests": promqlbuilder.Scalar(
		vector.New(
			vector.WithMetricName("kube_resourcequota"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("type").Equal("hard"),
				label.New("resource").Equal("requests.cpu"),
			),
		),
	),
	"CPUQuotaLimits": promqlbuilder.Scalar(
		vector.New(
			vector.WithMetricName("kube_resourcequota"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("type").Equal("hard"),
				label.New("resource").Equal("limits.cpu"),
			),
		),
	),
	"MemoryUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_working_set_bytes"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("pod"),
	"MemoryQuotaRequests": promqlbuilder.Scalar(
		vector.New(
			vector.WithMetricName("kube_resourcequota"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("type").Equal("hard"),
				label.New("resource").Equal("requests.memory"),
			),
		),
	),
	"MemoryQuotaLimits": promqlbuilder.Scalar(
		vector.New(
			vector.WithMetricName("kube_resourcequota"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("type").Equal("hard"),
				label.New("resource").Equal("limits.memory"),
			),
		),
	),

	// CPU Quota table queries
	"TableCPUUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
			),
		),
	).By("pod"),
	"TableCPURequestsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("pod"),
	"TableCPURequestsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("pod"),
	),
	"TableCPULimitsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("pod"),
	"TableCPULimitsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("pod"),
	),

	// Memory Quota table queries
	"TableMemoryUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_working_set_bytes"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("pod"),
	"TableMemoryRequestsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("pod"),
	"TableMemoryRequestsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("pod"),
	),
	"TableMemoryLimitsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("pod"),
	"TableMemoryLimitsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("pod"),
	),
	"TableMemoryRSSByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("pod"),
	"TableMemoryCacheByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_cache"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("pod"),
	"TableMemorySwapByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_swap"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("container").NotEqual(""),
			),
		),
	).By("pod"),
}

// Namespace (Workloads) queries - use workload join pattern

func workloadCPUUsage() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("workload", "workload_type")
}

func workloadCPURequests() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("workload", "workload_type")
}

func workloadCPULimits() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("workload", "workload_type")
}

func workloadMemoryUsage() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("workload", "workload_type")
}

func workloadMemoryRequests() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("workload", "workload_type")
}

func workloadMemoryLimits() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("workload", "workload_type")
}

var NamespaceWorkloadsQueries = map[string]parser.Expr{
	"CPUUsageByWorkload":                   workloadCPUUsage(),
	"TableRunningPods":                     promqlbuilder.Count(vector.New(vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"), vector.WithLabelMatchers(label.New("cluster").Equal("$cluster"), label.New("namespace").Equal("$namespace"), label.New("workload_type").Equal("$type")))).By("workload", "workload_type"),
	"TableCPUUsageByWorkload":              workloadCPUUsage(),
	"TableCPURequestsByWorkload":           workloadCPURequests(),
	"TableCPURequestsPercentByWorkload":    promqlbuilder.Div(workloadCPUUsage(), workloadCPURequests()),
	"TableCPULimitsByWorkload":             workloadCPULimits(),
	"TableCPULimitsPercentByWorkload":      promqlbuilder.Div(workloadCPUUsage(), workloadCPULimits()),
	"MemoryUsageByWorkload":                workloadMemoryUsage(),
	"TableMemoryUsageByWorkload":           workloadMemoryUsage(),
	"TableMemoryRequestsByWorkload":        workloadMemoryRequests(),
	"TableMemoryRequestsPercentByWorkload": promqlbuilder.Div(workloadMemoryUsage(), workloadMemoryRequests()),
	"TableMemoryLimitsByWorkload":          workloadMemoryLimits(),
	"TableMemoryLimitsPercentByWorkload":   promqlbuilder.Div(workloadMemoryUsage(), workloadMemoryLimits()),
}

// Node (Pods) queries

var NodePodsQueries = map[string]parser.Expr{
	// Graph queries
	"CPUUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
			),
		),
	).By("pod"),
	"MemoryUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_working_set_bytes"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
				label.New("job").Equal("kubelet"),
				label.New("metrics_path").Equal("/metrics/cadvisor"),
				label.New("image").NotEqual(""),
			),
		),
	).By("pod"),

	// CPU Quota table queries
	"TableCPUUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
			),
		),
	).By("pod"),
	"TableCPURequestsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("pod"),
	"TableCPURequestsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("pod"),
	),
	"TableCPULimitsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("pod"),
	"TableCPULimitsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("pod"),
	),

	// Memory Quota table queries
	"TableMemoryUsageByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_working_set_bytes"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
				label.New("job").Equal("kubelet"),
				label.New("metrics_path").Equal("/metrics/cadvisor"),
				label.New("image").NotEqual(""),
			),
		),
	).By("pod"),
	"TableMemoryRequestsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("pod"),
	"TableMemoryRequestsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
					label.New("job").Equal("kubelet"),
					label.New("metrics_path").Equal("/metrics/cadvisor"),
					label.New("image").NotEqual(""),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("pod"),
	),
	"TableMemoryLimitsByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("pod"),
	"TableMemoryLimitsPercentByPod": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
					label.New("job").Equal("kubelet"),
					label.New("metrics_path").Equal("/metrics/cadvisor"),
					label.New("image").NotEqual(""),
				),
			),
		).By("pod"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("node").Equal("$node"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("pod"),
	),
	"TableMemoryRSSByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("pod"),
	"TableMemoryCacheByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_cache"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("pod"),
	"TableMemorySwapByPod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_swap"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("node").Equal("$node"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
				label.New("job").Equal("kubelet"),
				label.New("metrics_path").Equal("/metrics/cadvisor"),
				label.New("image").NotEqual(""),
			),
		),
	).By("pod"),
}

// Pod queries (by container)

var PodQueries = map[string]parser.Expr{
	// Graph queries
	"CPUUsageByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
			vector.WithLabelMatchers(
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual("POD"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("container"),
	"CPURequestsOverlay": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("resource").Equal("cpu"),
			),
		),
	),
	"CPULimitsOverlay": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("resource").Equal("cpu"),
			),
		),
	),
	"CPUThrottling": promqlbuilder.Div(
		promqlbuilder.Sum(
			promqlbuilder.Increase(
				matrix.New(
					vector.New(
						vector.WithMetricName("container_cpu_cfs_throttled_periods_total"),
						vector.WithLabelMatchers(
							label.New("namespace").Equal("$namespace"),
							label.New("pod").Equal("$pod"),
							label.New("container").NotEqual("POD"),
							label.New("cluster").Equal("$cluster"),
						),
					),
					matrix.WithRangeAsString("5m"),
				),
			),
		).By("container"),
		promqlbuilder.Sum(
			promqlbuilder.Increase(
				matrix.New(
					vector.New(
						vector.WithMetricName("container_cpu_cfs_periods_total"),
						vector.WithLabelMatchers(
							label.New("namespace").Equal("$namespace"),
							label.New("pod").Equal("$pod"),
							label.New("container").NotEqual("POD"),
							label.New("cluster").Equal("$cluster"),
						),
					),
					matrix.WithRangeAsString("5m"),
				),
			),
		).By("container"),
	),
	"MemoryUsageByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_working_set_bytes"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual("POD"),
				label.New("container").NotEqual(""),
			),
		),
	).By("container"),
	"MemoryRequestsOverlay": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("resource").Equal("memory"),
			),
		),
	),
	"MemoryLimitsOverlay": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("resource").Equal("memory"),
			),
		),
	),

	// CPU Quota table queries
	"TableCPUUsageByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("container"),
	"TableCPURequestsByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("container"),
	"TableCPURequestsPercentByContainer": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
				),
			),
		).By("container"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("container"),
	),
	"TableCPULimitsByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("container"),
	"TableCPULimitsPercentByContainer": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
				),
			),
		).By("container"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
					label.New("resource").Equal("cpu"),
				),
			),
		).By("container"),
	),

	// Memory Quota table queries
	"TableMemoryUsageByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_working_set_bytes"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual("POD"),
				label.New("container").NotEqual(""),
			),
		),
	).By("container"),
	"TableMemoryRequestsByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("container"),
	"TableMemoryRequestsPercentByContainer": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
				),
			),
		).By("container"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("container"),
	),
	"TableMemoryLimitsByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_limits"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual(""),
				label.New("resource").Equal("memory"),
			),
		),
	).By("container"),
	"TableMemoryLimitsPercentByContainer": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
					label.New("container").NotEqual(""),
				),
			),
		).By("container"),
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("pod").Equal("$pod"),
					label.New("resource").Equal("memory"),
				),
			),
		).By("container"),
	),
	"TableMemoryRSSByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("container"),
	"TableMemoryCacheByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_cache"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("container"),
	"TableMemorySwapByContainer": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_swap"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("container"),
}

// Workload queries (specific workload, by pod)

func specificWorkloadCPUUsage() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate5m"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload").Equal("$workload"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("pod")
}

func specificWorkloadCPURequests() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload").Equal("$workload"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("pod")
}

func specificWorkloadCPULimits() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("cpu"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload").Equal("$workload"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("pod")
}

func specificWorkloadMemoryUsage() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("container_memory_working_set_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("container").NotEqual(""),
					label.New("container").NotEqual("POD"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload").Equal("$workload"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("pod")
}

func specificWorkloadMemoryRequests() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_requests"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload").Equal("$workload"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("pod")
}

func specificWorkloadMemoryLimits() parser.Expr {
	return promqlbuilder.Sum(
		promqlbuilder.Mul(
			vector.New(
				vector.WithMetricName("kube_pod_container_resource_limits"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("resource").Equal("memory"),
				),
			),
			vector.New(
				vector.WithMetricName("namespace_workload_pod:kube_pod_owner:relabel"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").Equal("$namespace"),
					label.New("workload").Equal("$workload"),
					label.New("workload_type").Equal("$type"),
				),
			),
		).On("namespace", "pod").GroupLeft("workload", "workload_type"),
	).By("pod")
}

var WorkloadQueries = map[string]parser.Expr{
	"CPUUsageByPod":                   specificWorkloadCPUUsage(),
	"TableCPUUsageByPod":              specificWorkloadCPUUsage(),
	"TableCPURequestsByPod":           specificWorkloadCPURequests(),
	"TableCPURequestsPercentByPod":    promqlbuilder.Div(specificWorkloadCPUUsage(), specificWorkloadCPURequests()),
	"TableCPULimitsByPod":             specificWorkloadCPULimits(),
	"TableCPULimitsPercentByPod":      promqlbuilder.Div(specificWorkloadCPUUsage(), specificWorkloadCPULimits()),
	"MemoryUsageByPod":                specificWorkloadMemoryUsage(),
	"TableMemoryUsageByPod":           specificWorkloadMemoryUsage(),
	"TableMemoryRequestsByPod":        specificWorkloadMemoryRequests(),
	"TableMemoryRequestsPercentByPod": promqlbuilder.Div(specificWorkloadMemoryUsage(), specificWorkloadMemoryRequests()),
	"TableMemoryLimitsByPod":          specificWorkloadMemoryLimits(),
	"TableMemoryLimitsPercentByPod":   promqlbuilder.Div(specificWorkloadMemoryUsage(), specificWorkloadMemoryLimits()),
}
