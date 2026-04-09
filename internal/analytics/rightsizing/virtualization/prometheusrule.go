package virtualization

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
			Name:      rightsizing.VirtualizationPrometheusRuleName,
			Namespace: rightsizing.MonitoringNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:     "acm-vm-right-sizing-namespace-5m.rule",
					Interval: &rightsizing.Duration5m,
					Rules:    buildNamespaceRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-vm-right-sizing-namespace-1d.rules",
					Interval: &rightsizing.Duration1d,
					Rules:    buildNamespaceRules1d(configData, rb),
				},
				{
					Name:     "acm-vm-right-sizing-cluster-5m.rule",
					Interval: &rightsizing.Duration5m,
					Rules:    buildClusterRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-vm-right-sizing-cluster-1d.rule",
					Interval: &rightsizing.Duration1d,
					Rules:    buildClusterRules1d(configData, rb),
				},
			},
		},
	}, nil
}

// buildNamespaceRules5m builds 5-minute recording rules for VM namespace-level resource metrics.
// Uses rb.Rule so the optional label_env join is appended (namespace label is preserved in aggregation).
func buildNamespaceRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.Rule(
			"acm_rs_vm:namespace:cpu_request:5m",
			fmt.Sprintf(
				`max_over_time(
					(
						count by (name, namespace) (kubevirt_vmi_vcpu_seconds_total{%s})
					)[5m:]
				)`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs_vm:namespace:memory_request:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  kubevirt_vm_resource_requests{%s, resource="memory"}
				) by (name,namespace)[5m:])`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs_vm:namespace:cpu_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  rate(kubevirt_vmi_cpu_usage_seconds_total{%s}[5m:])
				) by (name,namespace)[5m:])`,
				nsFilter,
			),
		),
		rb.Rule(
			"acm_rs_vm:namespace:memory_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  kubevirt_vmi_memory_available_bytes{%s} -
				  kubevirt_vmi_memory_usable_bytes{%s}
				) by (name,namespace)[5m:])`,
				nsFilter, nsFilter,
			),
		),
	}
}

// buildNamespaceRules1d builds 1-day aggregation recording rules for VM namespace-level metrics.
// Aggregates the 5m rules into daily summaries with profile/aggregation labels for dashboard selection.
func buildNamespaceRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}
	return []monitoringv1.Rule{
		rb.RuleWithLabels("acm_rs_vm:namespace:cpu_request", rightsizing.Build1dAggregationExpr("acm_rs_vm:namespace:cpu_request:5m")),
		rb.RuleWithLabels("acm_rs_vm:namespace:cpu_usage", rightsizing.Build1dAggregationExpr("acm_rs_vm:namespace:cpu_usage:5m")),
		rb.RuleWithLabels("acm_rs_vm:namespace:memory_request", rightsizing.Build1dAggregationExpr("acm_rs_vm:namespace:memory_request:5m")),
		rb.RuleWithLabels("acm_rs_vm:namespace:memory_usage", rightsizing.Build1dAggregationExpr("acm_rs_vm:namespace:memory_usage:5m")),
		rb.RuleWithLabels("acm_rs_vm:namespace:cpu_recommendation", rightsizing.BuildRecommendationExpr("acm_rs_vm:namespace:cpu_usage:5m", rp)),
		rb.RuleWithLabels("acm_rs_vm:namespace:memory_recommendation", rightsizing.BuildRecommendationExpr("acm_rs_vm:namespace:memory_usage:5m", rp)),
	}
}

// buildClusterRules5m builds 5-minute recording rules for VM cluster-level resource metrics.
// Uses rb.RuleNoJoin because aggregation by (cluster) removes the namespace label,
// making a post-aggregation "on (namespace)" label join invalid.
func buildClusterRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:cpu_request:5m",
			fmt.Sprintf(
				`max_over_time(
					(
						count by (cluster) (kubevirt_vmi_vcpu_seconds_total{%s})
					)[5m:]
				)`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:cpu_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  rate(kubevirt_vmi_cpu_usage_seconds_total{%s}[5m:])
				) by (cluster)[5m:])`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:memory_request:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  kubevirt_vm_resource_requests{%s, resource="memory"}
				) by (cluster)[5m:])`,
				nsFilter,
			),
		),
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:memory_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  kubevirt_vmi_memory_available_bytes{%s} -
				  kubevirt_vmi_memory_usable_bytes{%s}
				) by (cluster)[5m:])`,
				nsFilter, nsFilter,
			),
		),
	}
}

// buildClusterRules1d builds 1-day aggregation recording rules for VM cluster-level metrics.
// Aggregates the 5m cluster rules into daily summaries with profile/aggregation labels for dashboard selection.
func buildClusterRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}
	return []monitoringv1.Rule{
		rb.RuleWithLabels("acm_rs_vm:cluster:cpu_request", rightsizing.Build1dAggregationExpr("acm_rs_vm:cluster:cpu_request:5m")),
		rb.RuleWithLabels("acm_rs_vm:cluster:cpu_usage", rightsizing.Build1dAggregationExpr("acm_rs_vm:cluster:cpu_usage:5m")),
		rb.RuleWithLabels("acm_rs_vm:cluster:cpu_recommendation", rightsizing.BuildRecommendationExpr("acm_rs_vm:cluster:cpu_usage:5m", rp)),
		rb.RuleWithLabels("acm_rs_vm:cluster:memory_request", rightsizing.Build1dAggregationExpr("acm_rs_vm:cluster:memory_request:5m")),
		rb.RuleWithLabels("acm_rs_vm:cluster:memory_usage", rightsizing.Build1dAggregationExpr("acm_rs_vm:cluster:memory_usage:5m")),
		rb.RuleWithLabels("acm_rs_vm:cluster:memory_recommendation", rightsizing.BuildRecommendationExpr("acm_rs_vm:cluster:memory_usage:5m", rp)),
	}
}
