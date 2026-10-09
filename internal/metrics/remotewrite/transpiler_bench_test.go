package remotewrite

import (
	"testing"

	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"k8s.io/utils/ptr"
)

func BenchmarkTranspile(b *testing.B) {
	agent := &cooprometheusv1alpha1.PrometheusAgent{
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
					{
						Name: ptr.To("acm-observability"),
						URL:  "https://thanos-receive.example.com/api/v1/receive",
						WriteRelabelConfigs: []cooprometheusv1.RelabelConfig{
							{
								TargetLabel: "cluster",
								Replacement: ptr.To("spoke-cluster-1"),
							},
							{
								TargetLabel: "clusterID",
								Replacement: ptr.To("0000-0000-0000-0001"),
							},
						},
					},
				},
			},
		},
	}

	simpleSC := &cooprometheusv1alpha1.ScrapeConfig{
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					"up",
					"node_cpu_seconds_total{mode=\"idle\"}",
					"container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}",
				},
			},
		},
	}

	fleetScaleSC := &cooprometheusv1alpha1.ScrapeConfig{
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					"up",
					"node_cpu_seconds_total{mode=\"idle\"}",
					"node_cpu_seconds_total{mode=\"system\"}",
					"node_memory_MemAvailable_bytes",
					"container_memory_working_set_bytes{container!=\"\",container!=\"POD\"}",
					"container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}",
					"kube_pod_status_phase{phase=~\"Pending|Running|Failed\"}",
					"kube_node_status_condition{condition=\"Ready\",status=\"true\"}",
					"apiserver_request_total{code=~\"5..\"}",
					"apiserver_request_duration_seconds_bucket{le=~\"0.1|0.5|1|5\"}",
					"workqueue_adds_total{name=~\"cluster.*\"}",
					"etcd_server_has_leader",
					"cluster_infrastructure_provider{type=\"OpenShift\"}",
					"cluster_version{type=\"completed\"}",
					"instance:node_num_cpu:sum",
					"kube_deployment_status_replicas_available",
					"kube_statefulset_status_replicas_ready",
					"storage_operation_duration_seconds_count{status!=\"success\"}",
					"network_receive_bytes_total{device!~\"lo|veth.*\"}",
					"process_resident_memory_bytes{job=~\"apiserver|etcd\"}",
				},
			},
		},
	}

	b.Run("SimpleSelectors", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			specs, err := Transpile(simpleSC, agent)
			if err != nil {
				b.Fatalf("transpile error: %v", err)
			}
			if len(specs) == 0 {
				b.Fatal("unexpected empty remote write specs")
			}
		}
	})

	b.Run("FleetScaleSelectors", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			specs, err := Transpile(fleetScaleSC, agent)
			if err != nil {
				b.Fatalf("transpile error: %v", err)
			}
			if len(specs) == 0 {
				b.Fatal("unexpected empty remote write specs")
			}
		}
	})
}
