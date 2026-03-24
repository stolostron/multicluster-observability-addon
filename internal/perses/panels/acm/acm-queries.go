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
	"Top50MaxLatencyAPIServer_MaxLatency": promqlbuilder.Mul(
		promqlbuilder.TopK(
			promqlbuilder.Max(
				vector.New(
					vector.WithMetricName("apiserver_request_duration_seconds:histogram_quantile_99"),
					vector.WithLabelMatchers(
						label.New("cluster").EqualRegexp("$cluster"),
					),
				),
			).By("cluster"),
			50,
		),
		&parser.AggregateExpr{
			Op: parser.COUNT_VALUES,
			Expr: promqlbuilder.Round(promqlbuilder.Mul(promqlbuilder.Div(
				promqlbuilder.Sum(
					promqlbuilder.Eql(
						vector.New(
							vector.WithMetricName("up"),
							vector.WithLabelMatchers(
								label.New("cluster").EqualRegexp("$cluster"),
								label.New("service").Equal("kubernetes"),
							),
						),
						promqlbuilder.NewNumber(1),
					),
				).By("cluster"),
				promqlbuilder.Count(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(
							label.New("cluster").EqualRegexp("$cluster"),
							label.New("service").Equal("kubernetes"),
						),
					),
				).By("cluster"),
			), promqlbuilder.NewNumber(100)), 1),
			Param:   promqlbuilder.NewString("api_up"),
			Without: true,
		},
	).On("cluster").GroupLeft("api_up"),
	"APIServerRequestTotal_ErrorRate": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("sum:apiserver_request_total:1h"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
				label.New("code").EqualRegexp("5.."),
			),
		),
	).By("cluster"),

	// EtcdHealth query from acm-clusters-overview-panel.go
	"EtcdHealth_LeaderChanges": promqlbuilder.Mul(
		promqlbuilder.Mul(
			promqlbuilder.Sum(
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
			&parser.AggregateExpr{
				Op: parser.COUNT_VALUES,
				Expr: promqlbuilder.Round(promqlbuilder.Max(
					vector.New(
						vector.WithMetricName("etcd_mvcc_db_total_size_in_bytes"),
						vector.WithLabelMatchers(
							label.New("cluster").EqualRegexp("$cluster"),
							label.New("job").Equal("etcd"),
						),
					),
				).By("cluster"), 1),
				Param:   promqlbuilder.NewString("db_size"),
				Without: true,
			},
		).On("cluster").GroupLeft("db_size"),
		&parser.AggregateExpr{
			Op: parser.COUNT_VALUES,
			Expr: promqlbuilder.Round(promqlbuilder.Max(
				vector.New(
					vector.WithMetricName("etcd_server_has_leader"),
					vector.WithLabelMatchers(
						label.New("cluster").EqualRegexp("$cluster"),
						label.New("job").Equal("etcd"),
					),
				),
			).By("cluster"), 1),
			Param:   promqlbuilder.NewString("has_leader"),
			Without: true,
		},
	).On("cluster").GroupLeft("has_leader"),


	// Top50CPUOverestimation query from acm-clusters-overview-panel.go
	"Top50CPUOverestimation": promqlbuilder.Mul(
		promqlbuilder.Mul(
			promqlbuilder.TopK(
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
								label.New("cluster").EqualRegexp("$cluster"),
							),
						),
					).By("cluster"),
				),
				50,
			),
			&parser.AggregateExpr{
				Op: parser.COUNT_VALUES,
				Expr: promqlbuilder.Round(promqlbuilder.Mul(vector.New(
					vector.WithMetricName("cluster:cpu_requested:ratio"),
				), promqlbuilder.NewNumber(100)), 1),
				Param:   promqlbuilder.NewString("cpu_requested"),
				Without: true,
			},
		).On("cluster").GroupLeft("cpu_requested"),
		&parser.AggregateExpr{
			Op: parser.COUNT_VALUES,
			Expr: promqlbuilder.Round(promqlbuilder.Mul(vector.New(
				vector.WithMetricName("cluster:node_cpu:ratio"),
				vector.WithLabelMatchers(
					label.New("cluster").EqualRegexp("$cluster"),
				),
			), promqlbuilder.NewNumber(100)), 1),
			Param:   promqlbuilder.NewString("cpu_utilized"),
			Without: true,
		},
	).On("cluster").GroupLeft("cpu_utilized"),

	// Top50MemoryOverestimation query from acm-clusters-overview-panel.go
	"Top50MemoryOverestimation": promqlbuilder.Mul(
		promqlbuilder.Mul(
			promqlbuilder.TopK(
				promqlbuilder.Sub(
					vector.New(
						vector.WithMetricName("cluster:memory_requested:ratio"),
						vector.WithLabelMatchers(
							label.New("cluster").EqualRegexp("$cluster"),
						),
					),
					vector.New(
						vector.WithMetricName("cluster:memory_utilized:ratio"),
						vector.WithLabelMatchers(
							label.New("cluster").EqualRegexp("$cluster"),
						),
					),
				).Ignoring("usage"),
				50,
			),
			&parser.AggregateExpr{
				Op: parser.COUNT_VALUES,
				Expr: promqlbuilder.Round(promqlbuilder.Mul(vector.New(
					vector.WithMetricName("cluster:memory_requested:ratio"),
				), promqlbuilder.NewNumber(100)), 1),
				Param:   promqlbuilder.NewString("memory_requested"),
				Without: true,
			},
		).On("cluster").GroupLeft("memory_requested"),
		&parser.AggregateExpr{
			Op: parser.COUNT_VALUES,
			Expr: promqlbuilder.Round(promqlbuilder.Mul(vector.New(
				vector.WithMetricName("cluster:memory_utilized:ratio"),
			), promqlbuilder.NewNumber(100)), 1),
			Param:   promqlbuilder.NewString("memory_utilized"),
			Without: true,
		},
	).On("cluster").GroupLeft("memory_utilized"),

	// Top50CPUUtilized query from acm-clusters-overview-panel.go
	"Top50CPUUtilized": promqlbuilder.Mul(
		promqlbuilder.Mul(
			promqlbuilder.Mul(
				promqlbuilder.TopK(
					vector.New(
						vector.WithMetricName("cluster:node_cpu:ratio"),
						vector.WithLabelMatchers(
							label.New("cluster").EqualRegexp("$cluster"),
						),
					),
					50,
				),
				&parser.AggregateExpr{
					Op: parser.COUNT_VALUES,
					Expr: promqlbuilder.Round(vector.New(
						vector.WithMetricName("cluster:cpu_cores:sum"),
					), 1),
					Param:   promqlbuilder.NewString("machine_cpu_cores_sum"),
					Without: true,
				},
			).On("cluster").GroupLeft("machine_cpu_cores_sum"),
			&parser.AggregateExpr{
				Op: parser.COUNT_VALUES,
				Expr: promqlbuilder.Round(vector.New(
					vector.WithMetricName("cluster:cpu_allocatable:sum"),
				), 1),
				Param:   promqlbuilder.NewString("node_allocatable_cpu_cores_sum"),
				Without: true,
			},
		).On("cluster").GroupLeft("node_allocatable_cpu_cores_sum"),
		&parser.AggregateExpr{
			Op: parser.COUNT_VALUES,
			Expr: promqlbuilder.Round(promqlbuilder.Mul(vector.New(
				vector.WithMetricName("cluster:cpu_requested:ratio"),
			), promqlbuilder.NewNumber(100)), 1),
			Param:   promqlbuilder.NewString("cpu_requested"),
			Without: true,
		},
	).On("cluster").GroupLeft("cpu_requested"),

	// Top5CPUUtilized query from acm-clusters-overview-panel.go
	"Top5CPUUtilized": promqlbuilder.TopK(
		vector.New(
			vector.WithMetricName("cluster:node_cpu:ratio"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
			),
		),
		5,
	),

	// Top50MemoryUtilized query from acm-clusters-overview-panel.go
	"Top50MemoryUtilized": promqlbuilder.Mul(
		promqlbuilder.Mul(
			promqlbuilder.TopK(
				vector.New(
					vector.WithMetricName("cluster:memory_utilized:ratio"),
					vector.WithLabelMatchers(
						label.New("cluster").EqualRegexp("$cluster"),
					),
				),
				50,
			),
			&parser.AggregateExpr{
				Op: parser.COUNT_VALUES,
				Expr: promqlbuilder.Round(vector.New(
					vector.WithMetricName("cluster:machine_memory:sum"),
				), 1),
				Param:   promqlbuilder.NewString("machine_memory_sum"),
				Without: true,
			},
		).On("cluster").GroupLeft("machine_memory_sum"),
		&parser.AggregateExpr{
			Op: parser.COUNT_VALUES,
			Expr: promqlbuilder.Round(promqlbuilder.Mul(vector.New(
				vector.WithMetricName("cluster:memory_requested:ratio"),
			), promqlbuilder.NewNumber(100)), 1),
			Param:   promqlbuilder.NewString("machine_memory_requested"),
			Without: true,
		},
	).On("cluster").GroupLeft("machine_memory_requested"),

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
							label.New("cluster").EqualRegexp("$cluster"),
							label.New("resource").Equal("memory"),
						),
					),
				).By("cluster"),
			),
		),
		5,
	),

	// BandwidthUtilization combined query from acm-clusters-overview-panel.go
	"BandwidthUtilization": promqlbuilder.Mul(
		promqlbuilder.Mul(
			promqlbuilder.Mul(
				promqlbuilder.Sum(
					vector.New(
						vector.WithMetricName("instance:node_network_receive_bytes_excluding_lo:rate1m"),
						vector.WithLabelMatchers(
							label.New("cluster").EqualRegexp("$cluster"),
							label.New("job").Equal("node-exporter"),
						),
					),
				).By("cluster"),
				&parser.AggregateExpr{
					Op: parser.COUNT_VALUES,
					Expr: promqlbuilder.Round(promqlbuilder.Sum(
						vector.New(
							vector.WithMetricName("instance:node_network_transmit_bytes_excluding_lo:rate1m"),
							vector.WithLabelMatchers(
								label.New("cluster").EqualRegexp("$cluster"),
								label.New("job").Equal("node-exporter"),
							),
						),
					).By("cluster"), 1),
					Param:   promqlbuilder.NewString("node_transmit"),
					Without: true,
				},
			).On("cluster").GroupLeft("node_transmit"),
			&parser.AggregateExpr{
				Op: parser.COUNT_VALUES,
				Expr: promqlbuilder.Round(promqlbuilder.Sum(
					vector.New(
						vector.WithMetricName("instance:node_network_receive_drop_excluding_lo:rate1m"),
						vector.WithLabelMatchers(
							label.New("cluster").EqualRegexp("$cluster"),
							label.New("job").Equal("node-exporter"),
						),
					),
				).By("cluster"), 1),
				Param:   promqlbuilder.NewString("node_receive_drop"),
				Without: true,
			},
		).On("cluster").GroupLeft("node_receive_drop"),
		&parser.AggregateExpr{
			Op: parser.COUNT_VALUES,
			Expr: promqlbuilder.Round(promqlbuilder.Sum(
				vector.New(
					vector.WithMetricName("instance:node_network_transmit_drop_excluding_lo:rate1m"),
					vector.WithLabelMatchers(
						label.New("cluster").EqualRegexp("$cluster"),
						label.New("job").Equal("node-exporter"),
					),
				),
			).By("cluster"), 1),
			Param:   promqlbuilder.NewString("node_transmit_drop"),
			Without: true,
		},
	).On("cluster").GroupLeft("node_transmit_drop"),

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
	),
	"MemoryUsageNodeNamespacePod": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("container_memory_rss"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("container").NotEqual(""),
				label.New("container").NotEqual("POD"),
			),
		),
	).By("namespace"),
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
	"MemoryRequestsByNamespace": promqlbuilder.Div(
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