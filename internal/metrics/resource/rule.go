package resource

import (
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	defaultPlatformRuleName     = "platform-rules-default"
	defaultUserWorkloadRuleName = "user-workload-rules-default"
)

func PlatformRecordingRules(ns string) *prometheusv1.PrometheusRule {
	return &prometheusv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       prometheusv1.PrometheusRuleKind,
			APIVersion: prometheusv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultPlatformRuleName,
			Namespace: ns,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
		Spec: prometheusv1.PrometheusRuleSpec{
			Groups: []prometheusv1.RuleGroup{
				{
					Name: "acm-platform-default-rules",
					Rules: []prometheusv1.Rule{
						{
							Record: "apiserver_request_duration_seconds:histogram_quantile_99",
							Expr:   intstr.FromString(`(histogram_quantile(0.99,sum(rate(apiserver_request_latencies_bucket{job="apiserver", verb!="WATCH"}[5m])) by (le)))/1000000`),
						},
						{
							Record: "apiserver_request_duration_seconds:histogram_quantile_99:instance",
							Expr:   intstr.FromString(`(histogram_quantile(0.99,sum(rate(apiserver_request_latencies_bucket{job="apiserver", verb!="WATCH"}[5m])) by (le, verb, instance)))/1000000`),
						},
						{
							Record: "sum:apiserver_request_total:1h",
							Expr:   intstr.FromString(`sum(rate(apiserver_request_count{job="apiserver"}[1h])) by (code, instance)`),
						},
						{
							Record: "sum:apiserver_request_total:5m",
							Expr:   intstr.FromString(`sum(rate(apiserver_request_count{job="apiserver"}[5m])) by (code, instance)`),
						},
						{
							Record: "rpc_rate:grpc_server_handled_total:sum_rate",
							Expr:   intstr.FromString(`sum(rate(grpc_server_handled_total{job="etcd",grpc_type="unary",grpc_code!="OK"}[5m]))`),
						},
						{
							Record: "active_streams_watch:grpc_server_handled_total:sum",
							Expr:   intstr.FromString(`sum(grpc_server_started_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"}) - sum(grpc_server_handled_total{job="etcd",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"})`),
						},
						{
							Record: "active_streams_lease:grpc_server_handled_total:sum",
							Expr:   intstr.FromString(`sum(grpc_server_started_total{job="etcd",grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"}) - sum(grpc_server_handled_total{job="etcd",grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"})`),
						},
						{
							Record: "cluster:kube_pod_container_resource_requests:cpu:sum",
							Expr:   intstr.FromString(`sum(sum(sum(kube_pod_container_resource_requests_cpu_cores) by (pod,namespace,container) * on(pod,namespace) group_left(phase) max(kube_pod_status_phase{phase=~"Running|Pending|Unknown"} >0) by (pod,namespace,phase)) by (pod,namespace,phase))`),
						},
						{
							Record: "cluster:kube_pod_container_resource_requests:memory:sum",
							Expr:   intstr.FromString(`sum(sum(sum(kube_pod_container_resource_requests_memory_bytes) by (pod,namespace,container) * on(pod,namespace) group_left(phase) max(kube_pod_status_phase{phase=~"Running|Pending|Unknown"} >0) by (pod,namespace,phase)) by (pod,namespace,phase))`),
						},
						{
							Record: "sli:apiserver_request_duration_seconds:trend:1m",
							Expr:   intstr.FromString(`sum(increase(apiserver_request_latencies_bucket{job="apiserver",service="kubernetes",le="1",verb=~"POST|PUT|DELETE|PATCH"}[1m])) / sum(increase(apiserver_request_latencies_count{job="apiserver",service="kubernetes",verb=~"POST|PUT|DELETE|PATCH"}[1m]))`),
						},
						{
							Record: ":node_memory_MemAvailable_bytes:sum",
							Expr:   intstr.FromString(`sum(node_memory_MemAvailable_bytes{job="node-exporter"} or (node_memory_Buffers_bytes{job="node-exporter"} + node_memory_Cached_bytes{job="node-exporter"} + node_memory_MemFree_bytes{job="node-exporter"} + node_memory_Slab_bytes{job="node-exporter"}))`),
						},
						{
							Record: "instance:node_network_receive_bytes_excluding_lo:rate1m",
							Expr:   intstr.FromString(`sum(rate(node_network_receive_bytes_total{job="node-exporter", device!="lo"}[1m])) without(device)`),
						},
						{
							Record: "instance:node_network_transmit_bytes_excluding_lo:rate1m",
							Expr:   intstr.FromString(`sum(rate(node_network_transmit_bytes_total{job="node-exporter", device!="lo"}[1m])) without(device)`),
						},
						{
							Record: "instance:node_network_receive_drop_excluding_lo:rate1m",
							Expr:   intstr.FromString(`sum(rate(node_network_receive_drop_total{job="node-exporter", device!="lo"}[1m])) without(device)`),
						},
						{
							Record: "instance:node_network_transmit_drop_excluding_lo:rate1m",
							Expr:   intstr.FromString(`sum(rate(node_network_transmit_drop_total{job="node-exporter", device!="lo"}[1m])) without(device)`),
						},
					},
				},
			},
		},
	}
}

var ruleMetrics = []string{
	"apiserver_request_duration_seconds:histogram_quantile_99",
	"apiserver_request_duration_seconds:histogram_quantile_99:instance",
	"sum:apiserver_request_total:1h",
	"sum:apiserver_request_total:5m",
	"rpc_rate:grpc_server_handled_total:sum_rate",
	"active_streams_watch:grpc_server_handled_total:sum",
	"active_streams_lease:grpc_server_handled_total:sum",
	"cluster:kube_pod_container_resource_requests:cpu:sum",
	"cluster:kube_pod_container_resource_requests:memory:sum",
	"sli:apiserver_request_duration_seconds:trend:1m",
	":node_memory_MemAvailable_bytes:sum",
	"instance:node_network_receive_bytes_excluding_lo:rate1m",
	"instance:node_network_transmit_bytes_excluding_lo:rate1m",
	"instance:node_network_receive_drop_excluding_lo:rate1m",
	"instance:node_network_transmit_drop_excluding_lo:rate1m",
}
