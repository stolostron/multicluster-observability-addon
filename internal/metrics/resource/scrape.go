package resource

import (
	"fmt"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	defaultPlatformScrapeConfigName     = "platform-metrics-default"
	defaultUserWorkloadScrapeConfigName = "user-workload-metrics-default"
)

func PlatformScrapeConfig(ns string) *prometheusalpha1.ScrapeConfig {
	matchMetrics := make([]string, 0, len(metricsList)+len(matchList))
	for _, metric := range metricsList {
		matchMetrics = append(matchMetrics, fmt.Sprintf("{__name__=\"%s\"}", metric))
	}
	matchMetrics = append(matchMetrics, matchList...)
	for _, metric := range getRulesMetrics() {
		matchMetrics = append(matchMetrics, fmt.Sprintf("{__name__=\"%s\"}", metric))
	}

	return &prometheusalpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       prometheusalpha1.ScrapeConfigsKind,
			APIVersion: prometheusalpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultPlatformScrapeConfigName,
			Namespace: ns,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
		Spec: prometheusalpha1.ScrapeConfigSpec{
			MetricsPath: ptr.To("/federate"),
			Params: map[string][]string{
				"match[]": matchMetrics,
			},
			MetricRelabelConfigs: []prometheusv1.RelabelConfig{
				{
					SourceLabels: []prometheusv1.LabelName{"__name__"},
					Regex:        "mixin_pod_workload",
					TargetLabel:  "__name__",
					Replacement:  ptr.To("namespace_workload_pod:kube_pod_owner:relabel"),
					Action:       "replace",
				},
				{
					SourceLabels: []prometheusv1.LabelName{"__name__"},
					Regex:        "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate",
					TargetLabel:  "__name__",
					Replacement:  ptr.To("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate"),
					Action:       "replace",
				},
				{
					SourceLabels: []prometheusv1.LabelName{"__name__"},
					Regex:        "etcd_mvcc_db_total_size_in_bytes",
					TargetLabel:  "__name__",
					Replacement:  ptr.To("etcd_debugging_mvcc_db_total_size_in_bytes"),
					Action:       "replace",
				},
				// strip unneeded labels
				{
					Action: "labeldrop",
					Regex:  "prometheus_replica|managed_cluster",
				},
			},
			StaticConfigs: []prometheusalpha1.StaticConfig{
				{
					Targets: []prometheusalpha1.Target{
						prometheusalpha1.Target(fmt.Sprintf("localhost:%d", envoyProxyPortForPrometheus)),
					},
				},
			},
		},
	}
}

var metricsList = []string{
	":node_memory_MemAvailable_bytes:sum",
	"acm_managed_cluster_labels",
	"ALERTS",
	"cluster_infrastructure_provider",
	"cluster_operator_conditions",
	"cluster_version_payload",
	"cluster_version",
	"cluster:capacity_cpu_cores:sum",
	"cluster:capacity_memory_bytes:sum",
	"cluster:container_cpu_usage:ratio",
	"cluster:container_spec_cpu_shares:ratio",
	"cluster:cpu_usage_cores:sum",
	"cluster:memory_usage_bytes:sum",
	"cluster:memory_usage:ratio",
	"cluster:node_cpu:ratio",
	"cluster:usage:resources:sum",
	"cnv:vmi_status_running:count",
	"container_cpu_cfs_periods_total",
	"container_cpu_cfs_throttled_periods_total",
	"csv_abnormal",
	"csv_succeeded",
	"etcd_debugging_mvcc_db_total_size_in_bytes",
	"etcd_disk_backend_commit_duration_seconds_bucket",
	"etcd_disk_wal_fsync_duration_seconds_bucket",
	"etcd_mvcc_db_total_size_in_bytes",
	"etcd_network_client_grpc_received_bytes_total",
	"etcd_network_client_grpc_sent_bytes_total",
	"etcd_network_peer_received_bytes_total",
	"etcd_network_peer_sent_bytes_total",
	"etcd_server_has_leader",
	"etcd_server_leader_changes_seen_total",
	"etcd_server_proposals_applied_total",
	"etcd_server_proposals_committed_total",
	"etcd_server_proposals_failed_total",
	"etcd_server_proposals_pending",
	"grpc_server_started_total",
	"haproxy_backend_connections_total",
	"http_requests_total",
	"instance_device:node_disk_io_time_seconds:rate1m",
	"instance_device:node_disk_io_time_weighted_seconds:rate1m",
	"instance:node_cpu_utilisation:rate1m", //nolint:misspell
	"instance:node_load1_per_cpu:ratio",
	"instance:node_memory_utilisation:ratio", //nolint:misspell
	"instance:node_network_receive_bytes_excluding_lo:rate1m",
	"instance:node_network_receive_drop_excluding_lo:rate1m",
	"instance:node_network_transmit_bytes_excluding_lo:rate1m",
	"instance:node_network_transmit_drop_excluding_lo:rate1m",
	"instance:node_num_cpu:sum",
	"instance:node_vmstat_pgmajfault:rate1m",
	"kube_daemonset_status_desired_number_scheduled",
	"kube_node_spec_unschedulable",
	"kube_node_status_allocatable_cpu_cores",
	"kube_node_status_allocatable_memory_bytes",
	"kube_node_status_allocatable",
	"kube_node_status_capacity_cpu_cores",
	"kube_node_status_capacity",
	"kube_node_status_condition",
	"kube_persistentvolume_status_phase",
	"kube_pod_container_resource_limits_cpu_cores",
	"kube_pod_container_resource_limits_memory_bytes",
	"kube_pod_container_resource_limits",
	"kube_pod_container_resource_requests_cpu_cores",
	"kube_pod_container_resource_requests_memory_bytes",
	"kube_pod_container_resource_requests",
	"kube_pod_info",
	"kube_pod_owner",
	"kube_resourcequota",
	"kubelet_volume_stats_available_bytes",
	"kubelet_volume_stats_capacity_bytes",
	"kubevirt_hco_system_health_status",
	"kubevirt_hyperconverged_operator_health_status",
	"kubevirt_vm_error_status_last_transition_timestamp_seconds",
	"kubevirt_vm_migrating_status_last_transition_timestamp_seconds",
	"kubevirt_vm_non_running_status_last_transition_timestamp_seconds",
	"kubevirt_vm_running_status_last_transition_timestamp_seconds",
	"kubevirt_vm_starting_status_last_transition_timestamp_seconds",
	"kubevirt_vmi_cpu_usage_seconds_total",
	"kubevirt_vmi_info",
	"kubevirt_vmi_memory_available_bytes",
	"kubevirt_vmi_network_receive_bytes_total",
	"kubevirt_vmi_network_transmit_bytes_total",
	"kubevirt_vmi_phase_count",
	"kubevirt_vmi_storage_iops_read_total",
	"kubevirt_vmi_storage_iops_write_total",
	"machine_cpu_cores",
	"machine_memory_bytes",
	"mce_hs_addon_hosted_control_planes_status_gauge",
	"mce_hs_addon_qps_based_hcp_capacity_gauge",
	"mce_hs_addon_qps_gauge",
	"mce_hs_addon_request_based_hcp_capacity_gauge",
	"mce_hs_addon_worker_node_resource_capacities_gauge",
	"mixin_pod_workload",
	"namespace_cpu:kube_pod_container_resource_requests:sum",
	"namespace_memory:kube_pod_container_resource_requests:sum",
	"namespace_workload_pod:kube_pod_owner:relabel",
	"node_cpu_seconds_total",
	"node_filesystem_avail_bytes",
	"node_filesystem_size_bytes",
	"node_memory_MemAvailable_bytes",
	"node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate",
	"node_namespace_pod_container:container_cpu_usage_seconds_total:sum_rate",
	"node_netstat_Tcp_OutSegs",
	"node_netstat_Tcp_RetransSegs",
	"node_netstat_TcpExt_TCPSynRetrans",
	"policyreport_info",
	"up",
}

var matchList = []string{
	`{__name__="container_memory_cache",container!=""}`,
	`{__name__="container_memory_rss",container!=""}`,
	`{__name__="container_memory_swap",container!=""}`,
	`{__name__="container_memory_working_set_bytes",container!=""}`,
	`{__name__="go_goroutines",job="apiserver"}`,
	`{__name__="process_cpu_seconds_total",job="apiserver"}`,
	`{__name__="process_resident_memory_bytes",job=~"apiserver|etcd"}`,
	`{__name__="workqueue_adds_total",job="apiserver"}`,
	`{__name__="workqueue_depth",job="apiserver"}`,
	`{__name__="workqueue_queue_duration_seconds_bucket",job="apiserver"}`,
}
