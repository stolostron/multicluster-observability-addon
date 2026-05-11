package workload

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GeneratePrometheusRule builds a PrometheusRule containing workload+pod level recording rules.
func GeneratePrometheusRule(configData rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error) {
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
			Name:      rightsizing.WorkloadPrometheusRuleName,
			Namespace: rightsizing.MonitoringNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:     "acm-right-sizing-workload-5m.rules",
					Interval: &rightsizing.Duration5m,
					Rules:    buildWorkloadRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-right-sizing-workload-1d.rules",
					Interval: &rightsizing.Duration1d,
					Rules:    buildWorkloadRules1d(configData, rb),
				},
			},
		},
	}, nil
}

// podWorkloadRelabelExpr builds a PromQL expression that maps pods to their owning workloads,
// handling Deployments (via ReplicaSets), StatefulSets, DaemonSets, CronJobs (via Jobs),
// standalone Jobs, and standalone ReplicaSets.
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

func buildWorkloadRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rules := []monitoringv1.Rule{
		rb.Rule("acm_rs:pod_workload:relabel:5m", podWorkloadRelabelExpr(nsFilter)),
	}

	// Pod-level rules
	rules = append(rules,
		rb.Rule("acm_rs:pod:cpu_request:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  kube_pod_container_resource_requests{%s, resource="cpu", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:cpu_limit:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  kube_pod_container_resource_limits{%s, resource="cpu", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:cpu_usage:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{%s, container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:memory_request:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  kube_pod_container_resource_requests{%s, resource="memory", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:memory_limit:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  kube_pod_container_resource_limits{%s, resource="memory", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:pod:memory_usage:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, pod, workload, workload_type) (
			  container_memory_working_set_bytes{%s, container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
	)

	// Workload-level rules (aggregate pods by workload)
	rules = append(rules,
		rb.Rule("acm_rs:workload:cpu_request:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  kube_pod_container_resource_requests{%s, resource="cpu", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:cpu_limit:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  kube_pod_container_resource_limits{%s, resource="cpu", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:cpu_usage:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{%s, container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:memory_request:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  kube_pod_container_resource_requests{%s, resource="memory", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:memory_limit:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  kube_pod_container_resource_limits{%s, resource="memory", container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
		rb.Rule("acm_rs:workload:memory_usage:5m", fmt.Sprintf(
			`max_over_time(sum by (namespace, workload, workload_type) (
			  container_memory_working_set_bytes{%s, container!=""}
			  * on (namespace, pod) group_left(workload, workload_type)
			    acm_rs:pod_workload:relabel:5m
			)[5m:])`, nsFilter)),
	)

	return rules
}

// buildWorkloadRules1d builds 1-day aggregation recording rules for pod and workload metrics
// across all recommendation profiles (Max, P99, P95, Avg).
func buildWorkloadRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}

	var rules []monitoringv1.Rule
	for _, profile := range rightsizing.RecommendationProfiles {
		prb := rb.WithProfile(profile.Name)

		// Pod 1d aggregations
		rules = append(rules,
			prb.RuleWithLabels("acm_rs:pod:cpu_request", profile.AggExpr("acm_rs:pod:cpu_request:5m")),
			prb.RuleWithLabels("acm_rs:pod:cpu_limit", profile.AggExpr("acm_rs:pod:cpu_limit:5m")),
			prb.RuleWithLabels("acm_rs:pod:cpu_usage", profile.AggExpr("acm_rs:pod:cpu_usage:5m")),
			prb.RuleWithLabels("acm_rs:pod:cpu_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:pod:cpu_usage:5m", rp, profile)),
			prb.RuleWithLabels("acm_rs:pod:memory_request", profile.AggExpr("acm_rs:pod:memory_request:5m")),
			prb.RuleWithLabels("acm_rs:pod:memory_limit", profile.AggExpr("acm_rs:pod:memory_limit:5m")),
			prb.RuleWithLabels("acm_rs:pod:memory_usage", profile.AggExpr("acm_rs:pod:memory_usage:5m")),
			prb.RuleWithLabels("acm_rs:pod:memory_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:pod:memory_usage:5m", rp, profile)),
		)

		// Workload 1d aggregations
		rules = append(rules,
			prb.RuleWithLabels("acm_rs:workload:cpu_request", profile.AggExpr("acm_rs:workload:cpu_request:5m")),
			prb.RuleWithLabels("acm_rs:workload:cpu_limit", profile.AggExpr("acm_rs:workload:cpu_limit:5m")),
			prb.RuleWithLabels("acm_rs:workload:cpu_usage", profile.AggExpr("acm_rs:workload:cpu_usage:5m")),
			prb.RuleWithLabels("acm_rs:workload:cpu_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:workload:cpu_usage:5m", rp, profile)),
			prb.RuleWithLabels("acm_rs:workload:memory_request", profile.AggExpr("acm_rs:workload:memory_request:5m")),
			prb.RuleWithLabels("acm_rs:workload:memory_limit", profile.AggExpr("acm_rs:workload:memory_limit:5m")),
			prb.RuleWithLabels("acm_rs:workload:memory_usage", profile.AggExpr("acm_rs:workload:memory_usage:5m")),
			prb.RuleWithLabels("acm_rs:workload:memory_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:workload:memory_usage:5m", rp, profile)),
		)
	}

	return rules
}
