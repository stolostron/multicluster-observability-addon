package gpu

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GeneratePrometheusRule builds a PrometheusRule containing GPU recording rules
// at namespace, pod, workload, and cluster levels.
func GeneratePrometheusRule(configData rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error) {
	return GeneratePrometheusRuleWithMapping(configData, true)
}

// GeneratePrometheusRuleWithMapping builds a PrometheusRule for GPU metrics.
// When includePodWorkloadMapping is false, the pod-to-workload relabel expression
// is omitted (to avoid duplicate rules when workload-pod RS is also enabled).
func GeneratePrometheusRuleWithMapping(
	configData rightsizing.RSConfigMapData,
	includePodWorkloadMapping bool,
) (monitoringv1.PrometheusRule, error) {
	nsFilter, err := rightsizing.BuildNamespaceFilter(configData.PrometheusRuleConfig)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	labelJoin, err := rightsizing.BuildLabelJoin(configData.PrometheusRuleConfig.LabelFilterCriteria)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	rb := rightsizing.NewRuleBuilder(labelJoin)

	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.GPUPrometheusRuleName,
			Namespace: rightsizing.MonitoringNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:     "acm-right-sizing-gpu-namespace-5m.rules",
					Interval: &rightsizing.Duration5m,
					Rules:    buildGPUNamespaceRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-right-sizing-gpu-workload-5m.rules",
					Interval: &rightsizing.Duration5m,
					Rules:    buildGPUWorkloadPodRules5m(nsFilter, rb, includePodWorkloadMapping),
				},
				{
					Name:     "acm-right-sizing-gpu-namespace-1d.rules",
					Interval: &rightsizing.Duration1d,
					Rules:    buildGPUNamespaceRules1d(configData, rb),
				},
				{
					Name:     "acm-right-sizing-gpu-workload-1d.rules",
					Interval: &rightsizing.Duration1d,
					Rules:    buildGPUWorkloadPodRules1d(configData, rb),
				},
				{
					Name:     "acm-right-sizing-gpu-cluster-5m.rules",
					Interval: &rightsizing.Duration5m,
					Rules:    buildGPUClusterRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-right-sizing-gpu-cluster-1d.rules",
					Interval: &rightsizing.Duration1d,
					Rules:    buildGPUClusterRules1d(configData, rb),
				},
			},
		},
	}, nil
}

func buildGPUNamespaceRules5m(
	nsFilter string,
	rb *rightsizing.RuleBuilder,
) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.Rule(
			"acm_rs:namespace:gpu_request:5m",
			fmt.Sprintf(`max_over_time(sum by (namespace) (kube_pod_container_resource_requests{%s, resource=~"nvidia.com/gpu|amd.com/gpu", container!=""})[5m:])`, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_usage:5m",
			fmt.Sprintf(`max_over_time(sum by (namespace) (accelerator_gpu_utilization{%s})[5m:])`, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_memory_used:5m",
			fmt.Sprintf(`max_over_time(sum by (namespace) (accelerator_memory_used_bytes{%s})[5m:])`, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_memory_total:5m",
			fmt.Sprintf(`max_over_time(sum by (namespace) ((DCGM_FI_DEV_FB_USED{%s} + DCGM_FI_DEV_FB_FREE{%s}))[5m:])`, nsFilter, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_power_usage_watts:5m",
			fmt.Sprintf(`max_over_time(sum by (namespace) (accelerator_power_usage_watts{%s})[5m:])`, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_temperature_celsius:5m",
			fmt.Sprintf(`max_over_time(max by (namespace) (accelerator_temperature_celsius{%s})[5m:])`, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_sm_clock_hertz:5m",
			fmt.Sprintf(`max_over_time(max by (namespace) (accelerator_sm_clock_hertz{%s})[5m:])`, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_memory_clock_hertz:5m",
			fmt.Sprintf(`max_over_time(max by (namespace) (accelerator_memory_clock_hertz{%s})[5m:])`, nsFilter),
		),
		rb.Rule(
			"acm_rs:namespace:gpu_type:5m",
			fmt.Sprintf(`max by (namespace, resource) (kube_pod_container_resource_requests{%s, resource=~"nvidia.com/gpu|amd.com/gpu", container!=""})`, nsFilter),
		),
	}
}

func buildGPUWorkloadPodRules5m(
	nsFilter string,
	rb *rightsizing.RuleBuilder,
	includeMapping bool,
) []monitoringv1.Rule {
	var rules []monitoringv1.Rule

	if includeMapping {
		rules = append(rules, rb.Rule("acm_rs:pod_workload:relabel:5m", podWorkloadRelabelExpr(nsFilter)))
	}

	// Pod GPU series
	rules = append(rules,
		rb.Rule("acm_rs:pod:gpu_request:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  kube_pod_container_resource_requests{%s, resource=~"nvidia.com/gpu|amd.com/gpu", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:gpu_usage:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  accelerator_gpu_utilization{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:gpu_memory_used:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  accelerator_memory_used_bytes{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:gpu_memory_total:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  (DCGM_FI_DEV_FB_USED{%s} + DCGM_FI_DEV_FB_FREE{%s})
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter, nsFilter)),
		rb.Rule("acm_rs:pod:gpu_power_usage_watts:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  accelerator_power_usage_watts{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:gpu_temperature_celsius:5m", fmt.Sprintf(
			`max_over_time(max by (namespace, pod, workload, workload_type) (
			  accelerator_temperature_celsius{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:gpu_sm_clock_hertz:5m", fmt.Sprintf(
			`max_over_time(max by (namespace, pod, workload, workload_type) (
			  accelerator_sm_clock_hertz{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:gpu_memory_clock_hertz:5m", fmt.Sprintf(
			`max_over_time(max by (namespace, pod, workload, workload_type) (
			  accelerator_memory_clock_hertz{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
	)

	// Workload GPU series
	rules = append(rules,
		rb.Rule("acm_rs:workload:gpu_request:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  kube_pod_container_resource_requests{%s, resource=~"nvidia.com/gpu|amd.com/gpu", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:gpu_usage:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  accelerator_gpu_utilization{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:gpu_memory_used:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  accelerator_memory_used_bytes{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:gpu_memory_total:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  (DCGM_FI_DEV_FB_USED{%s} + DCGM_FI_DEV_FB_FREE{%s})
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter, nsFilter)),
		rb.Rule("acm_rs:workload:gpu_power_usage_watts:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  accelerator_power_usage_watts{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:gpu_temperature_celsius:5m", fmt.Sprintf(
			`max_over_time(max by (namespace, workload, workload_type) (
			  accelerator_temperature_celsius{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:gpu_sm_clock_hertz:5m", fmt.Sprintf(
			`max_over_time(max by (namespace, workload, workload_type) (
			  accelerator_sm_clock_hertz{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:gpu_memory_clock_hertz:5m", fmt.Sprintf(
			`max_over_time(max by (namespace, workload, workload_type) (
			  accelerator_memory_clock_hertz{%s}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
	)

	return rules
}

func buildGPUNamespaceRules1d(
	configData rightsizing.RSConfigMapData,
	rb *rightsizing.RuleBuilder,
) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	return []monitoringv1.Rule{
		rb.RuleWithLabels("acm_rs:namespace:gpu_request", `max_over_time(acm_rs:namespace:gpu_request:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_usage", `max_over_time(acm_rs:namespace:gpu_usage:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:namespace:gpu_usage:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:namespace:gpu_memory_used", `max_over_time(acm_rs:namespace:gpu_memory_used:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_memory_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:namespace:gpu_memory_used:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:namespace:gpu_memory_total", `max_over_time(acm_rs:namespace:gpu_memory_total:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_power_usage_watts", `max_over_time(acm_rs:namespace:gpu_power_usage_watts:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_temperature_celsius", `max_over_time(acm_rs:namespace:gpu_temperature_celsius:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_sm_clock_hertz", `max_over_time(acm_rs:namespace:gpu_sm_clock_hertz:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_memory_clock_hertz", `max_over_time(acm_rs:namespace:gpu_memory_clock_hertz:5m[1d])`),
		rb.RuleWithLabels("acm_rs:namespace:gpu_type", `max_over_time(acm_rs:namespace:gpu_type:5m[1d])`),
	}
}

func buildGPUWorkloadPodRules1d(
	configData rightsizing.RSConfigMapData,
	rb *rightsizing.RuleBuilder,
) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	return []monitoringv1.Rule{
		// pod
		rb.RuleWithLabels("acm_rs:pod:gpu_request", `max_over_time(acm_rs:pod:gpu_request:5m[1d])`),
		rb.RuleWithLabels("acm_rs:pod:gpu_usage", `max_over_time(acm_rs:pod:gpu_usage:5m[1d])`),
		rb.RuleWithLabels("acm_rs:pod:gpu_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:pod:gpu_usage:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:pod:gpu_memory_used", `max_over_time(acm_rs:pod:gpu_memory_used:5m[1d])`),
		rb.RuleWithLabels("acm_rs:pod:gpu_memory_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:pod:gpu_memory_used:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:pod:gpu_memory_total", `max_over_time(acm_rs:pod:gpu_memory_total:5m[1d])`),
		rb.RuleWithLabels("acm_rs:pod:gpu_power_usage_watts", `max_over_time(acm_rs:pod:gpu_power_usage_watts:5m[1d])`),
		rb.RuleWithLabels("acm_rs:pod:gpu_temperature_celsius", `max_over_time(acm_rs:pod:gpu_temperature_celsius:5m[1d])`),
		rb.RuleWithLabels("acm_rs:pod:gpu_sm_clock_hertz", `max_over_time(acm_rs:pod:gpu_sm_clock_hertz:5m[1d])`),
		rb.RuleWithLabels("acm_rs:pod:gpu_memory_clock_hertz", `max_over_time(acm_rs:pod:gpu_memory_clock_hertz:5m[1d])`),
		// workload
		rb.RuleWithLabels("acm_rs:workload:gpu_request", `max_over_time(acm_rs:workload:gpu_request:5m[1d])`),
		rb.RuleWithLabels("acm_rs:workload:gpu_usage", `max_over_time(acm_rs:workload:gpu_usage:5m[1d])`),
		rb.RuleWithLabels("acm_rs:workload:gpu_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:workload:gpu_usage:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:workload:gpu_memory_used", `max_over_time(acm_rs:workload:gpu_memory_used:5m[1d])`),
		rb.RuleWithLabels("acm_rs:workload:gpu_memory_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:workload:gpu_memory_used:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:workload:gpu_memory_total", `max_over_time(acm_rs:workload:gpu_memory_total:5m[1d])`),
		rb.RuleWithLabels("acm_rs:workload:gpu_power_usage_watts", `max_over_time(acm_rs:workload:gpu_power_usage_watts:5m[1d])`),
		rb.RuleWithLabels("acm_rs:workload:gpu_temperature_celsius", `max_over_time(acm_rs:workload:gpu_temperature_celsius:5m[1d])`),
		rb.RuleWithLabels("acm_rs:workload:gpu_sm_clock_hertz", `max_over_time(acm_rs:workload:gpu_sm_clock_hertz:5m[1d])`),
		rb.RuleWithLabels("acm_rs:workload:gpu_memory_clock_hertz", `max_over_time(acm_rs:workload:gpu_memory_clock_hertz:5m[1d])`),
	}
}

func buildGPUClusterRules5m(
	nsFilter string,
	rb *rightsizing.RuleBuilder,
) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.Rule("acm_rs:cluster:gpu_request:5m",
			fmt.Sprintf(`max_over_time(sum by (cluster) (kube_pod_container_resource_requests{%s, resource=~"nvidia.com/gpu|amd.com/gpu", container!=""})[5m:])`, nsFilter)),
		rb.Rule("acm_rs:cluster:gpu_usage:5m",
			fmt.Sprintf(`max_over_time(sum by (cluster) (accelerator_gpu_utilization{%s})[5m:])`, nsFilter)),
		rb.Rule("acm_rs:cluster:gpu_memory_used:5m",
			fmt.Sprintf(`max_over_time(sum by (cluster) (accelerator_memory_used_bytes{%s})[5m:])`, nsFilter)),
		rb.Rule("acm_rs:cluster:gpu_memory_total:5m",
			fmt.Sprintf(`max_over_time(sum by (cluster) ((DCGM_FI_DEV_FB_USED{%s} + DCGM_FI_DEV_FB_FREE{%s}))[5m:])`, nsFilter, nsFilter)),
		rb.Rule("acm_rs:cluster:gpu_power_usage_watts:5m",
			fmt.Sprintf(`max_over_time(sum by (cluster) (accelerator_power_usage_watts{%s})[5m:])`, nsFilter)),
		rb.Rule("acm_rs:cluster:gpu_temperature_celsius:5m",
			fmt.Sprintf(`max_over_time(max by (cluster) (accelerator_temperature_celsius{%s})[5m:])`, nsFilter)),
		rb.Rule("acm_rs:cluster:gpu_sm_clock_hertz:5m",
			fmt.Sprintf(`max_over_time(max by (cluster) (accelerator_sm_clock_hertz{%s})[5m:])`, nsFilter)),
		rb.Rule("acm_rs:cluster:gpu_memory_clock_hertz:5m",
			fmt.Sprintf(`max_over_time(max by (cluster) (accelerator_memory_clock_hertz{%s})[5m:])`, nsFilter)),
	}
}

func buildGPUClusterRules1d(
	configData rightsizing.RSConfigMapData,
	rb *rightsizing.RuleBuilder,
) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	return []monitoringv1.Rule{
		rb.RuleWithLabels("acm_rs:cluster:gpu_request", `max_over_time(acm_rs:cluster:gpu_request:5m[1d])`),
		rb.RuleWithLabels("acm_rs:cluster:gpu_usage", `max_over_time(acm_rs:cluster:gpu_usage:5m[1d])`),
		rb.RuleWithLabels("acm_rs:cluster:gpu_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:cluster:gpu_usage:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:cluster:gpu_memory_used", `max_over_time(acm_rs:cluster:gpu_memory_used:5m[1d])`),
		rb.RuleWithLabels("acm_rs:cluster:gpu_memory_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs:cluster:gpu_memory_used:5m[1d]) * (%d/100)`, rp)),
		rb.RuleWithLabels("acm_rs:cluster:gpu_memory_total", `max_over_time(acm_rs:cluster:gpu_memory_total:5m[1d])`),
		rb.RuleWithLabels("acm_rs:cluster:gpu_power_usage_watts", `max_over_time(acm_rs:cluster:gpu_power_usage_watts:5m[1d])`),
		rb.RuleWithLabels("acm_rs:cluster:gpu_temperature_celsius", `max_over_time(acm_rs:cluster:gpu_temperature_celsius:5m[1d])`),
		rb.RuleWithLabels("acm_rs:cluster:gpu_sm_clock_hertz", `max_over_time(acm_rs:cluster:gpu_sm_clock_hertz:5m[1d])`),
		rb.RuleWithLabels("acm_rs:cluster:gpu_memory_clock_hertz", `max_over_time(acm_rs:cluster:gpu_memory_clock_hertz:5m[1d])`),
	}
}

// podWorkloadRelabelExpr is the same mapping expression used by workload RS.
// It's duplicated here so GPU can optionally include it when workload RS is not enabled.
func podWorkloadRelabelExpr(nsFilter string) string {
	return fmt.Sprintf(
		`(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        kube_pod_owner{%s, owner_kind=~"StatefulSet|DaemonSet"},
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "$1", "owner_kind", "(.*)"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          label_replace(
		            kube_pod_owner{%s, owner_kind="ReplicaSet"},
		            "replicaset", "$1", "owner_name", "(.*)"
		          )
		          * on (namespace, replicaset) group_left(owner_name)
		            topk by (namespace, replicaset) (
		              1,
		              max by (namespace, replicaset, owner_name) (
		                kube_replicaset_owner{%s, owner_kind="Deployment"}
		              )
		            )
		        ),
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "Deployment", "workload", ".*"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          label_replace(
		            kube_pod_owner{%s, owner_kind="ReplicaSet"},
		            "replicaset", "$1", "owner_name", "(.*)"
		          )
		          unless on (namespace, replicaset)
		            kube_replicaset_owner{%s, owner_kind="Deployment"}
		        ),
		        "workload", "$1", "replicaset", "(.*)"
		      ),
		      "workload_type", "ReplicaSet", "workload", ".*"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          label_replace(
		            kube_pod_owner{%s, owner_kind="Job"},
		            "job_name", "$1", "owner_name", "(.*)"
		          )
		          * on (namespace, job_name) group_left(owner_name)
		            max by (namespace, job_name, owner_name) (
		              kube_job_owner{%s, owner_kind="CronJob"}
		            )
		        ),
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "CronJob", "workload", ".*"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          kube_pod_owner{%s, owner_kind="Job"}
		          unless on (namespace, owner_name)
		            max by (namespace, owner_name) (
		              label_replace(
		                kube_job_owner{%s, owner_kind="CronJob"},
		                "owner_name", "$1", "job_name", "(.*)"
		              )
		            )
		        ),
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "Job", "workload", ".*"
		    )
		  )
		)`,
		nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter,
	)
}
