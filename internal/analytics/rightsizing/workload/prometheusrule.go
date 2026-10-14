package workload

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateWorkloadPrometheusRule builds a PrometheusRule containing only workload-level recording rules.
func GenerateWorkloadPrometheusRule(configData rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error) {
	return generatePrometheusRule(configData, "workload")
}

// GeneratePodPrometheusRule builds a PrometheusRule containing only pod-level recording rules.
func GeneratePodPrometheusRule(configData rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error) {
	return generatePrometheusRule(configData, "pod")
}

func generatePrometheusRule(configData rightsizing.RSConfigMapData, level string) (monitoringv1.PrometheusRule, error) {
	nsFilter, err := rightsizing.BuildNamespaceFilter(configData.PrometheusRuleConfig)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	labelJoin, err := rightsizing.BuildLabelJoin(configData.PrometheusRuleConfig.LabelFilterCriteria)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	rb := rightsizing.NewRuleBuilder(labelJoin)

	ruleName := rightsizing.WorkloadPrometheusRuleName
	if level == "pod" {
		ruleName = rightsizing.PodPrometheusRuleName
	}
	groups := []monitoringv1.RuleGroup{}
	switch level {
	case "pod":
		groups = append(groups,
			monitoringv1.RuleGroup{Name: "acm-right-sizing-pod-5m.rules", Interval: &rightsizing.Duration5m, Rules: buildPodRules5m(nsFilter, rb)},
			monitoringv1.RuleGroup{Name: "acm-right-sizing-pod-1d.rules", Interval: &rightsizing.Duration1d, Rules: buildPodRules1d(configData, rb)},
		)
	default:
		groups = append(groups,
			monitoringv1.RuleGroup{Name: "acm-right-sizing-workload-5m.rules", Interval: &rightsizing.Duration5m, Rules: buildWorkloadOnlyRules5m(nsFilter, rb)},
			monitoringv1.RuleGroup{Name: "acm-right-sizing-workload-1d.rules", Interval: &rightsizing.Duration1d, Rules: buildWorkloadOnlyRules1d(configData, rb)},
		)
	}

	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: rightsizing.MonitoringNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: groups,
		},
	}, nil
}

// podWorkloadRelabelExpr maps pods to their owning workloads, handling
// Deployments (via ReplicaSets), StatefulSets, DaemonSets, CronJobs (via Jobs),
// standalone Jobs, standalone ReplicaSets, and ReplicationControllers. Pods with
// no recognized workload are retained with no workload/workload_type labels.
// Wrapping in topk(..., 1) ensures at most one series per (namespace, pod),
// preventing duplicate-series errors in downstream group_left joins if a pod
// matches multiple owner branches.
func podWorkloadRelabelExpr(nsFilter string) string {
	return fmt.Sprintf(
		`topk by (namespace, pod) (1,
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        kube_pod_owner{%s, owner_kind=~"StatefulSet|DaemonSet|ReplicationController"},
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
		)
		or
		(
		  max by (namespace, pod) (
		    kube_pod_info{%s}
		    unless on (namespace, pod) kube_pod_owner{%s, owner_kind=~"StatefulSet|DaemonSet|ReplicationController|ReplicaSet|Job"}
		  )
		)
		)`,
		nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter,
	)
}

func resource5mExpr(metric, resource, nsFilter, byLabels, mappingMetric string) string {
	return fmt.Sprintf(
		`max_over_time(sum by (%s) (
		  %s{%s, resource="%s", container!=""}
		  * on (namespace, pod) group_left()
		    kube_pod_status_phase{phase="Running"}
		  * on (namespace, pod) group_left(workload, workload_type)
		    %s
		)[5m:])`, byLabels, metric, nsFilter, resource, mappingMetric)
}

func usage5mExpr(metric, nsFilter, byLabels, mappingMetric string) string {
	return fmt.Sprintf(
		`max_over_time(sum by (%s) (
		  %s{%s, container!=""}
		  * on (namespace, pod) group_left(workload, workload_type)
		    %s
		)[5m:])`, byLabels, metric, nsFilter, mappingMetric)
}

func podResource5m(metric, resource, nsFilter, byLabels string) string {
	return resource5mExpr(metric, resource, nsFilter, byLabels, "acm_rs:pod_workload:relabel:5m")
}

func podUsage5m(metric, nsFilter, byLabels string) string {
	return usage5mExpr(metric, nsFilter, byLabels, "acm_rs:pod_workload:relabel:5m")
}

func workloadResource5m(metric, resource, nsFilter, byLabels string) string {
	return resource5mExpr(metric, resource, nsFilter, byLabels, "acm_rs:workload:pod_workload:relabel:5m{workload=~\".+\"}")
}

func workloadUsage5m(metric, nsFilter, byLabels string) string {
	return usage5mExpr(metric, nsFilter, byLabels, "acm_rs:workload:pod_workload:relabel:5m{workload=~\".+\"}")
}

func buildPodRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	by := "namespace, pod, workload, workload_type"
	return []monitoringv1.Rule{
		rb.Rule("acm_rs:pod:info", podWorkloadRelabelExpr(nsFilter)),
		rb.Rule("acm_rs:pod:cpu_request:5m", podResource5m("kube_pod_container_resource_requests", "cpu", nsFilter, by)),
		rb.Rule("acm_rs:pod:cpu_limit:5m", podResource5m("kube_pod_container_resource_limits", "cpu", nsFilter, by)),
		rb.Rule("acm_rs:pod:cpu_usage:5m", podUsage5m("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate", nsFilter, by)),
		rb.Rule("acm_rs:pod:memory_request:5m", podResource5m("kube_pod_container_resource_requests", "memory", nsFilter, by)),
		rb.Rule("acm_rs:pod:memory_limit:5m", podResource5m("kube_pod_container_resource_limits", "memory", nsFilter, by)),
		rb.Rule("acm_rs:pod:memory_usage:5m", podUsage5m("container_memory_working_set_bytes", nsFilter, by)),
	}
}

func buildWorkloadOnlyRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	by := "namespace, workload, workload_type"
	return []monitoringv1.Rule{
		rb.Rule("acm_rs:workload:pod_workload:relabel:5m", podWorkloadRelabelExpr(nsFilter)),
		rb.Rule("acm_rs:workload:cpu_request:5m", workloadResource5m("kube_pod_container_resource_requests", "cpu", nsFilter, by)),
		rb.Rule("acm_rs:workload:cpu_limit:5m", workloadResource5m("kube_pod_container_resource_limits", "cpu", nsFilter, by)),
		rb.Rule("acm_rs:workload:cpu_usage:5m", workloadUsage5m("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate", nsFilter, by)),
		rb.Rule("acm_rs:workload:memory_request:5m", workloadResource5m("kube_pod_container_resource_requests", "memory", nsFilter, by)),
		rb.Rule("acm_rs:workload:memory_limit:5m", workloadResource5m("kube_pod_container_resource_limits", "memory", nsFilter, by)),
		rb.Rule("acm_rs:workload:memory_usage:5m", workloadUsage5m("container_memory_working_set_bytes", nsFilter, by)),
	}
}

func buildPodRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	return buildLevelRules1d(configData, rb, true)
}

func buildWorkloadOnlyRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	return buildLevelRules1d(configData, rb, false)
}

func buildLevelRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder, pod bool) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}
	cp := rightsizing.ResolveCpuProfiles(configData.PrometheusRuleConfig)
	mp := rightsizing.ResolveMemoryProfiles(configData.PrometheusRuleConfig)
	level := "workload"
	if pod {
		level = "pod"
	}
	var rules []monitoringv1.Rule
	for _, profile := range cp {
		prb := rb.WithProfile(profile.Name)
		for _, metric := range []string{"cpu_request", "cpu_limit", "cpu_usage"} {
			rules = append(rules, prb.RuleWithLabels("acm_rs:"+level+":"+metric, profile.AggExpr("acm_rs:"+level+":"+metric+":5m")))
		}
		rules = append(rules, prb.RuleWithLabels("acm_rs:"+level+":cpu_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:"+level+":cpu_usage:5m", rp, profile)))
	}
	for _, profile := range mp {
		prb := rb.WithProfile(profile.Name)
		for _, metric := range []string{"memory_request", "memory_limit", "memory_usage"} {
			rules = append(rules, prb.RuleWithLabels("acm_rs:"+level+":"+metric, profile.AggExpr("acm_rs:"+level+":"+metric+":5m")))
		}
		rules = append(rules, prb.RuleWithLabels("acm_rs:"+level+":memory_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:"+level+":memory_usage:5m", rp, profile)))
	}
	return rules
}
