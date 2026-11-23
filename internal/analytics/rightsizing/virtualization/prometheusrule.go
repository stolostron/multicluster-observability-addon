package virtualization

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// cpuUsageRateWindow is the window of the rate() that turns the VM CPU counter into
// cores. It sets the resolution of the measurement, not how far back a rule looks, so it
// is deliberately not tied to the configured evaluation interval: a wider window dilutes
// short bursts and a narrower one amplifies scrape jitter, either of which would make the
// recorded usage depend on how often it is recorded.
const cpuUsageRateWindow = "5m"

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

	evalInterval := rightsizing.ResolveEvaluationInterval(configData.PrometheusRuleConfig)
	eval1dInterval := rightsizing.ResolveEvaluationInterval1d(configData.PrometheusRuleConfig)
	intervalStr := string(evalInterval)

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
					Interval: &evalInterval,
					Rules:    buildNamespaceRulesRolling(nsFilter, rb, intervalStr),
				},
				{
					Name:     "acm-vm-right-sizing-namespace-1d.rules",
					Interval: &eval1dInterval,
					Rules:    buildNamespaceRules1d(configData, rb),
				},
				{
					Name:     "acm-vm-right-sizing-cluster-5m.rule",
					Interval: &evalInterval,
					Rules:    buildClusterRulesRolling(nsFilter, rb, intervalStr),
				},
				{
					Name:     "acm-vm-right-sizing-cluster-1d.rule",
					Interval: &eval1dInterval,
					Rules:    buildClusterRules1d(configData, rb),
				},
			},
		},
	}, nil
}

// buildNamespaceRulesRolling builds rolling-window recording rules for VM namespace-level resource metrics.
// interval is the configured evaluation interval, used as the PromQL subquery lookback
// window (e.g. "5m", "10m"). The rate() nested in cpu_usage keeps cpuUsageRateWindow instead.
// Uses rb.Rule so the optional label_env join is appended (namespace label is preserved in aggregation).
func buildNamespaceRulesRolling(nsFilter string, rb *rightsizing.RuleBuilder, interval string) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.Rule(
			"acm_rs_vm:namespace:cpu_request:5m",
			fmt.Sprintf(
				`max_over_time(
					(
						count by (name, namespace) (kubevirt_vmi_vcpu_seconds_total{%s})
					)[%s:]
				)`,
				nsFilter, interval,
			),
		),
		rb.Rule(
			"acm_rs_vm:namespace:memory_request:5m",
			fmt.Sprintf(
				`max_over_time(max (
				  kubevirt_vm_resource_requests{%s, resource="memory"}
				) by (name,namespace)[%s:])`,
				nsFilter, interval,
			),
		),
		rb.Rule(
			"acm_rs_vm:namespace:cpu_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  rate(kubevirt_vmi_cpu_usage_seconds_total{%s}[%s:])
				) by (name,namespace)[%s:])`,
				nsFilter, cpuUsageRateWindow, interval,
			),
		),
		rb.Rule(
			"acm_rs_vm:namespace:memory_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  kubevirt_vmi_memory_available_bytes{%s} -
				  kubevirt_vmi_memory_usable_bytes{%s}
				) by (name,namespace)[%s:])`,
				nsFilter, nsFilter, interval,
			),
		),
	}
}

// buildNamespaceRules1d builds 1-day aggregation recording rules for VM namespace-level metrics.
// CPU and memory profiles are resolved independently from the ConfigMap.
func buildNamespaceRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}

	cpuProfiles := rightsizing.ResolveCpuProfiles(configData.PrometheusRuleConfig)
	memProfiles := rightsizing.ResolveMemoryProfiles(configData.PrometheusRuleConfig)

	var rules []monitoringv1.Rule
	for _, profile := range cpuProfiles {
		prb := rb.WithProfile(profile.Name)
		rules = append(rules,
			prb.RuleWithLabels("acm_rs_vm:namespace:cpu_request", profile.AggExpr("acm_rs_vm:namespace:cpu_request:5m")),
			prb.RuleWithLabels("acm_rs_vm:namespace:cpu_usage", profile.AggExpr("acm_rs_vm:namespace:cpu_usage:5m")),
			prb.RuleWithLabels("acm_rs_vm:namespace:cpu_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs_vm:namespace:cpu_usage:5m", rp, profile)),
		)
	}
	for _, profile := range memProfiles {
		prb := rb.WithProfile(profile.Name)
		rules = append(rules,
			prb.RuleWithLabels("acm_rs_vm:namespace:memory_request", profile.AggExpr("acm_rs_vm:namespace:memory_request:5m")),
			prb.RuleWithLabels("acm_rs_vm:namespace:memory_usage", profile.AggExpr("acm_rs_vm:namespace:memory_usage:5m")),
			prb.RuleWithLabels("acm_rs_vm:namespace:memory_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs_vm:namespace:memory_usage:5m", rp, profile)),
		)
	}
	return rules
}

// buildClusterRulesRolling builds rolling-window recording rules for VM cluster-level resource metrics.
// interval is the configured evaluation interval, used as the PromQL subquery lookback
// window (e.g. "5m", "10m"). The rate() nested in cpu_usage keeps cpuUsageRateWindow instead.
// Uses rb.RuleNoJoin because aggregation by (cluster) removes the namespace label,
// making a post-aggregation "on (namespace)" label join invalid.
func buildClusterRulesRolling(nsFilter string, rb *rightsizing.RuleBuilder, interval string) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:cpu_request:5m",
			fmt.Sprintf(
				`max_over_time(
					(
						count by (cluster) (kubevirt_vmi_vcpu_seconds_total{%s})
					)[%s:]
				)`,
				nsFilter, interval,
			),
		),
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:cpu_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  rate(kubevirt_vmi_cpu_usage_seconds_total{%s}[%s:])
				) by (cluster)[%s:])`,
				nsFilter, cpuUsageRateWindow, interval,
			),
		),
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:memory_request:5m",
			fmt.Sprintf(
				`max_over_time(sum by (cluster) (
				  max by (name, namespace, cluster) (
				    kubevirt_vm_resource_requests{%s, resource="memory"}
				  )
				)[%s:])`,
				nsFilter, interval,
			),
		),
		rb.RuleNoJoin(
			"acm_rs_vm:cluster:memory_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum (
				  kubevirt_vmi_memory_available_bytes{%s} -
				  kubevirt_vmi_memory_usable_bytes{%s}
				) by (cluster)[%s:])`,
				nsFilter, nsFilter, interval,
			),
		),
	}
}

// buildClusterRules1d builds 1-day aggregation recording rules for VM cluster-level metrics.
// CPU and memory profiles are resolved independently from the ConfigMap.
func buildClusterRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}

	cpuProfiles := rightsizing.ResolveCpuProfiles(configData.PrometheusRuleConfig)
	memProfiles := rightsizing.ResolveMemoryProfiles(configData.PrometheusRuleConfig)

	var rules []monitoringv1.Rule
	for _, profile := range cpuProfiles {
		prb := rb.WithProfile(profile.Name)
		rules = append(rules,
			prb.RuleWithLabels("acm_rs_vm:cluster:cpu_request", profile.AggExpr("acm_rs_vm:cluster:cpu_request:5m")),
			prb.RuleWithLabels("acm_rs_vm:cluster:cpu_usage", profile.AggExpr("acm_rs_vm:cluster:cpu_usage:5m")),
			prb.RuleWithLabels("acm_rs_vm:cluster:cpu_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs_vm:cluster:cpu_usage:5m", rp, profile)),
		)
	}
	for _, profile := range memProfiles {
		prb := rb.WithProfile(profile.Name)
		rules = append(rules,
			prb.RuleWithLabels("acm_rs_vm:cluster:memory_request", profile.AggExpr("acm_rs_vm:cluster:memory_request:5m")),
			prb.RuleWithLabels("acm_rs_vm:cluster:memory_usage", profile.AggExpr("acm_rs_vm:cluster:memory_usage:5m")),
			prb.RuleWithLabels("acm_rs_vm:cluster:memory_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs_vm:cluster:memory_usage:5m", rp, profile)),
		)
	}
	return rules
}
