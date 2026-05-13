package gpu

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePrometheusRule_IncludesNamespaceGPU(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			NamespaceFilterCriteria: struct {
				InclusionCriteria []string `json:"inclusionCriteria"`
				ExclusionCriteria []string `json:"exclusionCriteria"`
			}{
				InclusionCriteria: []string{"ns-a"},
			},
			RecommendationPercentage: 110,
		},
	}

	rule, err := GeneratePrometheusRuleWithMapping(config, true)
	require.NoError(t, err)
	assert.Equal(t, rightsizing.GPUPrometheusRuleName, rule.Name)
	assert.Len(t, rule.Spec.Groups, 6)
	assert.Contains(t, rule.Spec.Groups[0].Rules[0].Expr.String(), `resource=~"nvidia.com/gpu|amd.com/gpu"`)
}

func TestGeneratePrometheusRule_IncludesClusterGPUMetrics(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			NamespaceFilterCriteria: struct {
				InclusionCriteria []string `json:"inclusionCriteria"`
				ExclusionCriteria []string `json:"exclusionCriteria"`
			}{
				InclusionCriteria: []string{"ns-a"},
			},
			RecommendationPercentage: 110,
		},
	}

	rule, err := GeneratePrometheusRuleWithMapping(config, true)
	require.NoError(t, err)

	cluster5mRules := map[string]bool{}
	for _, rg := range rule.Spec.Groups {
		if rg.Name == "acm-right-sizing-gpu-cluster-5m.rules" {
			for _, r := range rg.Rules {
				cluster5mRules[r.Record] = true
			}
		}
	}

	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_request:5m"])
	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_usage:5m"])
	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_memory_used:5m"])
	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_memory_total:5m"])
	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_power_usage_watts:5m"])
	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_temperature_celsius:5m"])
	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_sm_clock_hertz:5m"])
	assert.True(t, cluster5mRules["acm_rs:cluster:gpu_memory_clock_hertz:5m"])
}

func TestGeneratePrometheusRule_RecommendationPercentage(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			NamespaceFilterCriteria: struct {
				InclusionCriteria []string `json:"inclusionCriteria"`
				ExclusionCriteria []string `json:"exclusionCriteria"`
			}{
				ExclusionCriteria: []string{"openshift.*"},
			},
			RecommendationPercentage: 120,
		},
	}

	rule, err := GeneratePrometheusRuleWithMapping(config, true)
	require.NoError(t, err)

	for _, rg := range rule.Spec.Groups {
		for _, r := range rg.Rules {
			if r.Record == "acm_rs:namespace:gpu_recommendation" {
				assert.Contains(t, r.Expr.String(), "(120/100)")
				return
			}
		}
	}
	t.Fatal("gpu_recommendation rule not found")
}

func TestGeneratePrometheusRule_WithoutMapping(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			RecommendationPercentage: 110,
		},
	}

	ruleWith, err := GeneratePrometheusRuleWithMapping(config, true)
	require.NoError(t, err)

	ruleWithout, err := GeneratePrometheusRuleWithMapping(config, false)
	require.NoError(t, err)

	var workload5mWith, workload5mWithout []string
	for _, rg := range ruleWith.Spec.Groups {
		if rg.Name == "acm-right-sizing-gpu-workload-5m.rules" {
			for _, r := range rg.Rules {
				workload5mWith = append(workload5mWith, r.Record)
			}
		}
	}
	for _, rg := range ruleWithout.Spec.Groups {
		if rg.Name == "acm-right-sizing-gpu-workload-5m.rules" {
			for _, r := range rg.Rules {
				workload5mWithout = append(workload5mWithout, r.Record)
			}
		}
	}

	assert.Contains(t, workload5mWith, "acm_rs:pod_workload:relabel:5m",
		"should include mapping when includePodWorkloadMapping=true")
	assert.NotContains(t, workload5mWithout, "acm_rs:pod_workload:relabel:5m",
		"should omit mapping when includePodWorkloadMapping=false")
	assert.Greater(t, len(workload5mWith), len(workload5mWithout))
}

func TestGeneratePrometheusRule_DefaultConfig(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			RecommendationPercentage: rightsizing.DefaultRecommendationPercentage,
		},
	}

	rule, err := GeneratePrometheusRule(config)
	require.NoError(t, err)

	assert.Equal(t, rightsizing.GPUPrometheusRuleName, rule.Name)
	assert.Equal(t, rightsizing.MonitoringNamespace, rule.Namespace)
	assert.Equal(t, "PrometheusRule", rule.Kind)
	assert.Equal(t, "monitoring.coreos.com/v1", rule.APIVersion)

	groupNames := make([]string, len(rule.Spec.Groups))
	for i, g := range rule.Spec.Groups {
		groupNames[i] = g.Name
	}
	assert.Contains(t, groupNames, "acm-right-sizing-gpu-namespace-5m.rules")
	assert.Contains(t, groupNames, "acm-right-sizing-gpu-workload-5m.rules")
	assert.Contains(t, groupNames, "acm-right-sizing-gpu-namespace-1d.rules")
	assert.Contains(t, groupNames, "acm-right-sizing-gpu-workload-1d.rules")
	assert.Contains(t, groupNames, "acm-right-sizing-gpu-cluster-5m.rules")
	assert.Contains(t, groupNames, "acm-right-sizing-gpu-cluster-1d.rules")
}

func TestAllGPUProfilesGenerated(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			RecommendationPercentage: rightsizing.DefaultRecommendationPercentage,
		},
	}
	rule, err := GeneratePrometheusRule(config)
	require.NoError(t, err)

	expectedProfiles := map[string]bool{
		"Max OverAll": false,
		"P99":         false,
		"P95":         false,
		"Avg":         false,
	}

	for _, rg := range rule.Spec.Groups {
		for _, r := range rg.Rules {
			if r.Record == "acm_rs:namespace:gpu_recommendation" {
				profile := r.Labels["profile"]
				if _, ok := expectedProfiles[profile]; ok {
					expectedProfiles[profile] = true
				}
			}
		}
	}

	for profile, found := range expectedProfiles {
		assert.True(t, found, "profile %q should generate gpu_recommendation rules", profile)
	}
}

func TestGPUProfileAggregationExpressions(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			RecommendationPercentage: rightsizing.DefaultRecommendationPercentage,
		},
	}
	rule, err := GeneratePrometheusRule(config)
	require.NoError(t, err)

	profileExprs := map[string]string{
		"Max OverAll": "max_over_time(",
		"P99":         "quantile_over_time(0.99,",
		"P95":         "quantile_over_time(0.95,",
		"Avg":         "avg_over_time(",
	}

	for _, rg := range rule.Spec.Groups {
		for _, r := range rg.Rules {
			if r.Record == "acm_rs:namespace:gpu_recommendation" {
				profile := r.Labels["profile"]
				if expectedPrefix, ok := profileExprs[profile]; ok {
					assert.Contains(t, r.Expr.String(), expectedPrefix,
						"profile %q should use %s", profile, expectedPrefix)
				}
			}
		}
	}
}
