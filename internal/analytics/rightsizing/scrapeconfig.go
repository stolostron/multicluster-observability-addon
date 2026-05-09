package rightsizing

import (
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	// ScrapeConfigName is the name for the right-sizing ScrapeConfig
	ScrapeConfigName = "platform-metrics-right-sizing"

	// ScrapeConfigJobName is the job name for the right-sizing scrape job
	ScrapeConfigJobName = "right-sizing"

	// Labels for the ScrapeConfig
	componentLabel = "platform-metrics-collector"
	partOfLabel    = "multicluster-observability-addon"
	managedByLabel = "multicluster-observability-addon"
)

// NamespaceMetrics are the metrics to federate for namespace right-sizing
// Uses 1d aggregated metrics matching MCO Grafana dashboard patterns
var NamespaceMetrics = []string{
	"acm_rs:namespace:cpu_request_hard",
	"acm_rs:namespace:cpu_request",
	"acm_rs:namespace:cpu_usage",
	"acm_rs:namespace:cpu_recommendation",
	"acm_rs:namespace:memory_request_hard",
	"acm_rs:namespace:memory_request",
	"acm_rs:namespace:memory_usage",
	"acm_rs:namespace:memory_recommendation",
	"acm_rs:cluster:cpu_request_hard",
	"acm_rs:cluster:cpu_request",
	"acm_rs:cluster:cpu_usage",
	"acm_rs:cluster:cpu_recommendation",
	"acm_rs:cluster:memory_request_hard",
	"acm_rs:cluster:memory_request",
	"acm_rs:cluster:memory_usage",
	"acm_rs:cluster:memory_recommendation",
}

// WorkloadPodMetrics are the metrics to federate for workload-pod right-sizing
var WorkloadPodMetrics = []string{
	"acm_rs:pod:cpu_request",
	"acm_rs:pod:cpu_limit",
	"acm_rs:pod:cpu_usage",
	"acm_rs:pod:cpu_recommendation",
	"acm_rs:pod:memory_request",
	"acm_rs:pod:memory_limit",
	"acm_rs:pod:memory_usage",
	"acm_rs:pod:memory_recommendation",
	"acm_rs:workload:cpu_request",
	"acm_rs:workload:cpu_limit",
	"acm_rs:workload:cpu_usage",
	"acm_rs:workload:cpu_recommendation",
	"acm_rs:workload:memory_request",
	"acm_rs:workload:memory_limit",
	"acm_rs:workload:memory_usage",
	"acm_rs:workload:memory_recommendation",
}

// VirtualizationMetrics are the metrics to federate for virtualization right-sizing
// Uses 1d aggregated metrics matching MCO Grafana dashboard patterns
var VirtualizationMetrics = []string{
	"acm_rs_vm:namespace:cpu_request",
	"acm_rs_vm:namespace:cpu_usage",
	"acm_rs_vm:namespace:cpu_recommendation",
	"acm_rs_vm:namespace:memory_request",
	"acm_rs_vm:namespace:memory_usage",
	"acm_rs_vm:namespace:memory_recommendation",
	"acm_rs_vm:cluster:cpu_request",
	"acm_rs_vm:cluster:cpu_usage",
	"acm_rs_vm:cluster:cpu_recommendation",
	"acm_rs_vm:cluster:memory_request",
	"acm_rs_vm:cluster:memory_usage",
	"acm_rs_vm:cluster:memory_recommendation",
	"kubevirt_vm_running_status_last_transition_timestamp_seconds",
}

// GPUMetrics are the metrics to federate for GPU right-sizing
var GPUMetrics = []string{
	"acm_rs:namespace:gpu_request",
	"acm_rs:namespace:gpu_usage",
	"acm_rs:namespace:gpu_recommendation",
	"acm_rs:namespace:gpu_memory_used",
	"acm_rs:namespace:gpu_memory_recommendation",
	"acm_rs:namespace:gpu_memory_total",
	"acm_rs:namespace:gpu_power_usage_watts",
	"acm_rs:namespace:gpu_temperature_celsius",
	"acm_rs:namespace:gpu_sm_clock_hertz",
	"acm_rs:namespace:gpu_memory_clock_hertz",
	"acm_rs:namespace:gpu_type",
	"acm_rs:pod:gpu_request",
	"acm_rs:pod:gpu_usage",
	"acm_rs:pod:gpu_recommendation",
	"acm_rs:pod:gpu_memory_used",
	"acm_rs:pod:gpu_memory_recommendation",
	"acm_rs:pod:gpu_memory_total",
	"acm_rs:pod:gpu_power_usage_watts",
	"acm_rs:pod:gpu_temperature_celsius",
	"acm_rs:pod:gpu_sm_clock_hertz",
	"acm_rs:pod:gpu_memory_clock_hertz",
	"acm_rs:workload:gpu_request",
	"acm_rs:workload:gpu_usage",
	"acm_rs:workload:gpu_recommendation",
	"acm_rs:workload:gpu_memory_used",
	"acm_rs:workload:gpu_memory_recommendation",
	"acm_rs:workload:gpu_memory_total",
	"acm_rs:workload:gpu_power_usage_watts",
	"acm_rs:workload:gpu_temperature_celsius",
	"acm_rs:workload:gpu_sm_clock_hertz",
	"acm_rs:workload:gpu_memory_clock_hertz",
	"acm_rs:cluster:gpu_request",
	"acm_rs:cluster:gpu_usage",
	"acm_rs:cluster:gpu_recommendation",
	"acm_rs:cluster:gpu_memory_used",
	"acm_rs:cluster:gpu_memory_recommendation",
	"acm_rs:cluster:gpu_memory_total",
	"acm_rs:cluster:gpu_power_usage_watts",
	"acm_rs:cluster:gpu_temperature_celsius",
	"acm_rs:cluster:gpu_sm_clock_hertz",
	"acm_rs:cluster:gpu_memory_clock_hertz",
}

// GenerateScrapeConfig generates a ScrapeConfig for right-sizing metrics federation.
// Returns nil when no features are selected for the cluster. The caller gates
// invocation on metrics collection being enabled, which guarantees the ScrapeConfig
// CRD exists on the spoke. The work agent then tracks the resource via
// AppliedManifestWork and deletes it when it disappears from the ManifestWork spec.
func GenerateScrapeConfig(includeNamespace, includeVirtualization, includeWorkloadPod, includeGPU bool) *cooprometheusv1alpha1.ScrapeConfig {
	var matchParams []string

	if includeNamespace {
		for _, metric := range NamespaceMetrics {
			matchParams = append(matchParams, "{__name__=\""+metric+"\"}")
		}
	}

	if includeWorkloadPod {
		for _, metric := range WorkloadPodMetrics {
			matchParams = append(matchParams, "{__name__=\""+metric+"\"}")
		}
	}

	if includeGPU {
		for _, metric := range GPUMetrics {
			matchParams = append(matchParams, "{__name__=\""+metric+"\"}")
		}
	}

	if includeVirtualization {
		for _, metric := range VirtualizationMetrics {
			matchParams = append(matchParams, "{__name__=\""+metric+"\"}")
		}
	}

	if len(matchParams) == 0 {
		return nil
	}

	return &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ScrapeConfig",
			APIVersion: "monitoring.rhobs/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ScrapeConfigName,
			Labels: map[string]string{
				"app.kubernetes.io/component":  componentLabel,
				"app.kubernetes.io/part-of":    partOfLabel,
				"app.kubernetes.io/managed-by": managedByLabel,
			},
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			JobName:     ptr.To(ScrapeConfigJobName),
			MetricsPath: ptr.To("/federate"),
			Params: map[string][]string{
				"match[]": matchParams,
			},
			MetricRelabelConfigs: []cooprometheusv1.RelabelConfig{
				{
					Action: "labeldrop",
					Regex:  "managed_cluster|id",
				},
			},
		},
	}
}
