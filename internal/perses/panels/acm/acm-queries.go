package acm

import (
	"maps"

	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/promql/parser"
)

var ACMCommonPanelQueries = map[string]parser.Expr{
	// Top50MaxLatencyAPIServer queries from acm-clusters-overview-panel.go
	"Top50MaxLatencyAPIServer_MaxLatency": promqlbuilder.TopK(
		promqlbuilder.Max(
			vector.New(
				vector.WithMetricName("apiserver_request_duration_seconds:histogram_quantile_99"),
				vector.WithLabelMatchers(
					label.New("cluster").EqualRegexp("$cluster"),
					label.New("clusterType").NotEqual("ocp3"),
				),
			),
		).By("cluster"),
		50,
	),
	"Top50MaxLatencyAPIServer_ErrorRate": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("sum:apiserver_request_total:1h"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("code").EqualRegexp("5.."),
				label.New("clusterType").NotEqual("ocp3"),
			),
		),
	).By("cluster"),

	// EtcdHealth query from acm-clusters-overview-panel.go
	"EtcdHealth_LeaderChanges": promqlbuilder.Sum(
		promqlbuilder.Changes(
			matrix.New(
				vector.New(
					vector.WithMetricName("etcd_server_leader_changes_seen_total"),
					vector.WithLabelMatchers(
						label.New("cluster").EqualRegexp("$cluster"),
						label.New("job").Equal("etcd"),
					),
				),
				matrix.WithRangeAsVariable("$__range"),
			),
		),
	).By("cluster"),
	"EtcdHealth_DBSize": promqlbuilder.Max(
		vector.New(
			vector.WithMetricName("etcd_debugging_mvcc_db_total_size_in_bytes"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("job").Equal("etcd"),
			),
		),
	).By("cluster"),
	"EtcdHealth_HasLeader": promqlbuilder.Max(
		vector.New(
			vector.WithMetricName("etcd_server_has_leader"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("job").Equal("etcd"),
			),
		),
	).By("cluster"),

	// Top50CPUOverestimation query from acm-clusters-overview-panel.go
	"Top50CPUOverestimation": promqlbuilder.TopK(
		promqlbuilder.Sub(
			promqlbuilder.Sum(
				vector.New(
					vector.WithMetricName("cluster:cpu_requested:ratio"),
				),
			).By("cluster"),
			promqlbuilder.Sum(
				vector.New(
					vector.WithMetricName("cluster:node_cpu:ratio"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
					),
				),
			).By("cluster"),
		),
		50,
	),

	// Top50MemoryOverestimation query from acm-clusters-overview-panel.go
	"Top50MemoryOverestimation": promqlbuilder.TopK(
		promqlbuilder.Sub(
			vector.New(
				vector.WithMetricName("cluster:memory_requested:ratio"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
				),
			),
			vector.New(
				vector.WithMetricName("cluster:memory_utilized:ratio"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
				),
			),
		),
		50,
	),

	// Top50CPUUtilized query from acm-clusters-overview-panel.go
	"Top50CPUUtilized": promqlbuilder.TopK(
		vector.New(
			vector.WithMetricName("cluster:node_cpu:ratio"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
		50,
	),

	// Top5CPUUtilized query from acm-clusters-overview-panel.go
	"Top5CPUUtilized": promqlbuilder.TopK(
		vector.New(
			vector.WithMetricName("cluster:node_cpu:ratio"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
		5,
	),

	// Top50MemoryUtilized query from acm-clusters-overview-panel.go
	"Top50MemoryUtilized": promqlbuilder.TopK(
		vector.New(
			vector.WithMetricName("cluster:memory_utilized:ratio"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
		50,
	),

	// Top5MemoryUtilized query from acm-clusters-overview-panel.go
	"Top5MemoryUtilized": promqlbuilder.TopK(
		promqlbuilder.Sub(
			promqlbuilder.NewNumber(1),
			promqlbuilder.Div(
				promqlbuilder.Sum(
					vector.New(
						vector.WithMetricName(":node_memory_MemAvailable_bytes:sum"),
					),
				).By("cluster"),
				promqlbuilder.Sum(
					vector.New(
						vector.WithMetricName("kube_node_status_allocatable"),
						vector.WithLabelMatchers(
							label.New("cluster").Equal("$cluster"),
							label.New("resource").Equal("memory"),
						),
					),
				).By("cluster"),
			),
		),
		5,
	),

	// BandwidthUtilization queries from acm-clusters-overview-panel.go
	"BandwidthUtilization_ReceiveRate": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_receive_bytes_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("job").Equal("node-exporter"),
			),
		),
	).By("cluster"),
	"BandwidthUtilization_TransmitRate": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_transmit_bytes_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("job").Equal("node-exporter"),
			),
		),
	).By("cluster"),
	"BandwidthUtilization_ReceiveDropRate": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_receive_drop_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("job").Equal("node-exporter"),
			),
		),
	).By("cluster"),
	"BandwidthUtilization_TransmitDropRate": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_transmit_drop_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("job").Equal("node-exporter"),
			),
		),
	).By("cluster"),

	// Alert queries from acm-alert-analysis-panel.go
	"Top10AlertsFiringByName": promqlbuilder.TopK(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("ALERTS"),
				vector.WithLabelMatchers(
					label.New("alertstate").Equal("firing"),
					label.New("severity").EqualRegexp("$severity"),
				),
			),
		).By("alertname"),
		10,
	),
	"Top10AlertsFiringByCluster": promqlbuilder.TopK(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("ALERTS"),
				vector.WithLabelMatchers(
					label.New("alertstate").Equal("firing"),
					label.New("cluster").NotEqual(""),
					label.New("severity").EqualRegexp("$severity"),
				),
			),
		).By("cluster"),
		10,
	),

	// Alert queries from acm-alerts-by-cluster-panel.go
	"AlertsFiringSeverity": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("severity"),
	"AlertsFiringByName": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("cluster").Equal("$cluster"),
				label.New("severity").EqualRegexp("$severity"),
			),
		),
	).By("alertname"),
	"AlertsPendingByName": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("pending"),
				label.New("cluster").Equal("$cluster"),
				label.New("severity").EqualRegexp("$severity"),
			),
		),
	).By("alertname"),

	// Individual component queries for reference
	"ClusterCPURequested": vector.New(
		vector.WithMetricName("cluster:cpu_requested:ratio"),
	),
	"ClusterCPUUtilized": vector.New(
		vector.WithMetricName("cluster:node_cpu:ratio"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	"ClusterMemoryRequested": vector.New(
		vector.WithMetricName("cluster:memory_requested:ratio"),
	),
	"ClusterMemoryUtilized": vector.New(
		vector.WithMetricName("cluster:memory_utilized:ratio"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	"ClusterCPUCoresSum": vector.New(
		vector.WithMetricName("cluster:cpu_cores:sum"),
	),
	"ClusterCPUAllocatableSum": vector.New(
		vector.WithMetricName("cluster:cpu_allocatable:sum"),
	),
	"ClusterMachineMemorySum": vector.New(
		vector.WithMetricName("cluster:machine_memory:sum"),
	),

	// Queries from acm-clusters-by-alert-panel.go
	"ClustersWithAlertSeverity": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("alertname").EqualRegexp("$alert"),
				label.New("severity").EqualRegexp("$severity"),
			),
		),
	).By("cluster"),

	// Queries from acm-optimization-overview-panel.go
	"CPUOverestimation": promqlbuilder.Sub(
		promqlbuilder.Div(
			promqlbuilder.Sum(
				vector.New(
					vector.WithMetricName("cluster:kube_pod_container_resource_requests:cpu:sum"),
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
						label.New("resource").Equal("cpu"),
					),
				),
			),
		),
		promqlbuilder.Sub(
			promqlbuilder.NewNumber(1),
			vector.New(
				vector.WithMetricName("node_cpu_seconds_total:mode_idle:avg_rate5m"),
			),
		),
	),
	"CPUUsageNodeNamespacePod": vector.New(
		vector.WithMetricName("node_namespace_pod_container:container_cpu_usage_seconds_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	"CPURequestsCommitment": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("cluster:kube_pod_container_resource_requests:cpu:sum"),
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
					label.New("resource").Equal("cpu"),
				),
			),
		),
	),
	"CPUUtilization": promqlbuilder.Sub(
		promqlbuilder.NewNumber(1),
		vector.New(
			vector.WithMetricName("node_cpu_seconds_total:mode_idle:avg_rate5m"),
		),
	),
	"CPURequestsByNamespace": promqlbuilder.Div(
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
	"CPURequestsByNamespaceSum": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("resource").Equal("cpu"),
			),
		),
	).By("namespace"),
	"PodsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_info"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),

	// Memory queries from acm-optimization-overview-panel.go
	"MemoryOverestimation": promqlbuilder.Sub(
		promqlbuilder.Div(
			promqlbuilder.Sum(
				vector.New(
					vector.WithMetricName("cluster:kube_pod_container_resource_requests:memory:sum"),
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
		promqlbuilder.Sub(
			promqlbuilder.NewNumber(1),
			promqlbuilder.Div(
				vector.New(
					vector.WithMetricName("node_memory_MemAvailable_bytes"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
					),
				),
				vector.New(
					vector.WithMetricName("node_memory_MemTotal_bytes"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
					),
				),
			),
		),
	),
	"MemoryUsageNodeNamespacePod": vector.New(
		vector.WithMetricName("node_namespace_pod_container:container_memory_working_set_bytes:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	"MemoryRequestsCommitment": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("cluster:kube_pod_container_resource_requests:memory:sum"),
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
	"MemoryUtilization": promqlbuilder.Sub(
		promqlbuilder.NewNumber(1),
		promqlbuilder.Div(
			vector.New(
				vector.WithMetricName("node_memory_MemAvailable_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
				),
			),
			vector.New(
				vector.WithMetricName("node_memory_MemTotal_bytes"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
				),
			),
		),
	),
	"MemoryRequestsByNamespace": promqlbuilder.Div(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("node_namespace_pod_container:container_memory_working_set_bytes:sum"),
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
					label.New("resource").Equal("memory"),
				),
			),
		).By("namespace"),
	),
	"MemoryRequestsByNamespaceSum": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("kube_pod_container_resource_requests"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("resource").Equal("memory"),
			),
		),
	).By("namespace"),

	// Networking queries from acm-optimization-overview-panel.go
	"NetworkReceiveByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_receive_bytes_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),
	"NetworkTransmitByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_transmit_bytes_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),
	"NetworkReceiveDropByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_receive_drop_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),
	"NetworkTransmitDropByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_transmit_drop_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),

	// Alert analysis queries from acm-alert-analysis-panel.go (with or vector(0) fallback)
	"TotalAlerts": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
			),
		),
	),
	"TotalCriticalAlerts": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("severity").Equal("critical"),
			),
		),
	),
	"TotalWarningAlerts": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("severity").Equal("warning"),
			),
		),
	),
	"TotalModerateAlerts": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("severity").Equal("moderate"),
			),
		),
	),
	"TotalLowAlerts": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("severity").Equal("low"),
			),
		),
	),
	"TotalImportantAlerts": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("severity").Equal("important"),
			),
		),
	),
	"AlertTypeOverTime": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("severity").EqualRegexp("$severity"),
			),
		),
	).By("alertname"),
	"ClusterAffectedOverTime": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("cluster").NotEqual(""),
				label.New("severity").EqualRegexp("$severity"),
			),
		),
	).By("cluster"),
	"AlertsAndClusters": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("ALERTS"),
			vector.WithLabelMatchers(
				label.New("alertstate").Equal("firing"),
				label.New("severity").EqualRegexp("$severity"),
			),
		),
	).By("cluster", "alertname", "severity"),

	"LabelVariable": vector.New(
		vector.WithMetricName("acm_label_names")),
}

// OverrideACMPanelQueries overrides the ACMCommonPanelQueries global.
// Refer to panel queries in the map, that you'd like to override.
// The convention of naming followed, is to use Panel function name (with _suffix, in case panel has multiple queries)
func OverrideACMPanelQueries(queries map[string]parser.Expr) {
	maps.Copy(ACMCommonPanelQueries, queries)
}
