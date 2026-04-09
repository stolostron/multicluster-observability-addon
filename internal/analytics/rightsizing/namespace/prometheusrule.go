package namespace

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GeneratePrometheusRule builds PrometheusRule based on configdata
func GeneratePrometheusRule(configData rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error) {
	nsFilter, err := rightsizing.BuildNamespaceFilter(configData.PrometheusRuleConfig)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	labelJoin, err := rightsizing.BuildLabelJoin(configData.PrometheusRuleConfig.LabelFilterCriteria)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	// Create rule builder with shared utilities
	rb := rightsizing.NewRuleBuilder(labelJoin)

	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespacePrometheusRuleName,
			Namespace: rightsizing.MonitoringNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:     "acm-right-sizing-namespace-5m.rule",
					Interval: &rightsizing.Duration5m,
					Rules:    buildNamespaceRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-right-sizing-namespace-1d.rules",
					Interval: &rightsizing.Duration1d,
					Rules:    buildNamespaceRules1d(configData, rb),
				},
				{
					Name:     "acm-right-sizing-cluster-5m.rule",
					Interval: &rightsizing.Duration5m,
					Rules:    buildClusterRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-right-sizing-cluster-1d.rule",
					Interval: &rightsizing.Duration1d,
					Rules:    buildClusterRules1d(configData, rb),
				},
			},
		},
	}, nil
}

// buildNamespaceRules5m builds 5-minute recording rules for namespace-level resource metrics.
// Uses rb.Rule so the optional label_env join is appended (namespace label is preserved in aggregation).
func buildNamespaceRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.Rule(
			"acm_rs:namespace:cpu_request_hard:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_resourcequota{resource=~"requests.cpu", type="hard", %s}) by (namespace)[5m:])`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs:namespace:cpu_request:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_pod_container_resource_requests{`+
					`%s, resource="cpu", container!=""}) by (namespace)[5m:])`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs:namespace:cpu_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum(node_namespace_pod_container:`+
					`container_cpu_usage_seconds_total:sum_irate{`+
					`%s, container!=""}) by (namespace)[5m:])`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs:namespace:memory_request_hard:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_resourcequota{resource=~"requests.memory", type="hard", %s}) by (namespace)[5m:])`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs:namespace:memory_request:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_pod_container_resource_requests{`+
					`%s, resource="memory", container!=""}) by (namespace)[5m:])`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs:namespace:memory_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum(container_memory_working_set_bytes{`+
					`%s, container!=""}) by (namespace)[5m:])`,
				nsFilter,
			),
		),
	}
}

// buildNamespaceRules1d builds 1-day aggregation recording rules for namespace-level metrics.
// Aggregates the 5m rules into daily summaries with profile/aggregation labels for dashboard selection.
func buildNamespaceRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}
	return []monitoringv1.Rule{
		rb.RuleWithLabels("acm_rs:namespace:cpu_request_hard", rightsizing.Build1dAggregationExpr("acm_rs:namespace:cpu_request_hard:5m")),
		rb.RuleWithLabels("acm_rs:namespace:cpu_request", rightsizing.Build1dAggregationExpr("acm_rs:namespace:cpu_request:5m")),
		rb.RuleWithLabels("acm_rs:namespace:cpu_usage", rightsizing.Build1dAggregationExpr("acm_rs:namespace:cpu_usage:5m")),
		rb.RuleWithLabels("acm_rs:namespace:cpu_recommendation", rightsizing.BuildRecommendationExpr("acm_rs:namespace:cpu_usage:5m", rp)),
		rb.RuleWithLabels("acm_rs:namespace:memory_request_hard", rightsizing.Build1dAggregationExpr("acm_rs:namespace:memory_request_hard:5m")),
		rb.RuleWithLabels("acm_rs:namespace:memory_request", rightsizing.Build1dAggregationExpr("acm_rs:namespace:memory_request:5m")),
		rb.RuleWithLabels("acm_rs:namespace:memory_usage", rightsizing.Build1dAggregationExpr("acm_rs:namespace:memory_usage:5m")),
		rb.RuleWithLabels("acm_rs:namespace:memory_recommendation", rightsizing.BuildRecommendationExpr("acm_rs:namespace:memory_usage:5m", rp)),
	}
}

// buildClusterRules5m builds 5-minute recording rules for cluster-level resource metrics.
// Uses rb.RuleNoJoin because aggregation by (cluster) removes the namespace label,
// making a post-aggregation "on (namespace)" label join invalid.
func buildClusterRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.RuleNoJoin(
			"acm_rs:cluster:cpu_request_hard:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_resourcequota{resource=~"requests.cpu", type="hard", %s}) by (cluster)[5m:])`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs:cluster:cpu_request:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_pod_container_resource_requests{`+
					`%s, resource="cpu", container!=""}) by (cluster)[5m:])`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs:cluster:cpu_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum(node_namespace_pod_container:`+
					`container_cpu_usage_seconds_total:sum_irate{`+
					`%s, container!=""}) by (cluster)[5m:])`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs:cluster:memory_request_hard:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_resourcequota{resource=~"requests.memory", type="hard", %s}) by (cluster)[5m:])`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs:cluster:memory_request:5m",
			fmt.Sprintf(
				`max_over_time(sum(kube_pod_container_resource_requests{`+
					`%s, resource="memory", container!=""}) by (cluster)[5m:])`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs:cluster:memory_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum(container_memory_working_set_bytes{`+
					`%s, container!=""}) by (cluster)[5m:])`,
				nsFilter,
			),
		),
	}
}

// buildClusterRules1d builds 1-day aggregation recording rules for cluster-level metrics.
// Aggregates the 5m cluster rules into daily summaries with profile/aggregation labels for dashboard selection.
func buildClusterRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}
	return []monitoringv1.Rule{
		rb.RuleWithLabels("acm_rs:cluster:cpu_request_hard", rightsizing.Build1dAggregationExpr("acm_rs:cluster:cpu_request_hard:5m")),
		rb.RuleWithLabels("acm_rs:cluster:cpu_request", rightsizing.Build1dAggregationExpr("acm_rs:cluster:cpu_request:5m")),
		rb.RuleWithLabels("acm_rs:cluster:cpu_usage", rightsizing.Build1dAggregationExpr("acm_rs:cluster:cpu_usage:5m")),
		rb.RuleWithLabels("acm_rs:cluster:cpu_recommendation", rightsizing.BuildRecommendationExpr("acm_rs:cluster:cpu_usage:5m", rp)),
		rb.RuleWithLabels("acm_rs:cluster:memory_request_hard", rightsizing.Build1dAggregationExpr("acm_rs:cluster:memory_request_hard:5m")),
		rb.RuleWithLabels("acm_rs:cluster:memory_request", rightsizing.Build1dAggregationExpr("acm_rs:cluster:memory_request:5m")),
		rb.RuleWithLabels("acm_rs:cluster:memory_usage", rightsizing.Build1dAggregationExpr("acm_rs:cluster:memory_usage:5m")),
		rb.RuleWithLabels("acm_rs:cluster:memory_recommendation", rightsizing.BuildRecommendationExpr("acm_rs:cluster:memory_usage:5m", rp)),
	}
}
