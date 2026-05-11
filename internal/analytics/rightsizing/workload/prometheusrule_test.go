package workload

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePrometheusRule(t *testing.T) {
	tests := []struct {
		name        string
		configData  rightsizing.RSConfigMapData
		expectError bool
	}{
		{
			name: "default config",
			configData: rightsizing.RSConfigMapData{
				PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
			},
		},
		{
			name: "with inclusion filter",
			configData: rightsizing.RSConfigMapData{
				PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
					NamespaceFilterCriteria: struct {
						InclusionCriteria []string `json:"inclusionCriteria"`
						ExclusionCriteria []string `json:"exclusionCriteria"`
					}{
						InclusionCriteria: []string{"default", "my-namespace"},
					},
					RecommendationPercentage: 120,
				},
			},
		},
		{
			name: "with exclusion filter",
			configData: rightsizing.RSConfigMapData{
				PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
					NamespaceFilterCriteria: struct {
						InclusionCriteria []string `json:"inclusionCriteria"`
						ExclusionCriteria []string `json:"exclusionCriteria"`
					}{
						ExclusionCriteria: []string{"openshift.*", "kube-.*"},
					},
					RecommendationPercentage: 110,
				},
			},
		},
		{
			name: "invalid config - both inclusion and exclusion",
			configData: rightsizing.RSConfigMapData{
				PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
					NamespaceFilterCriteria: struct {
						InclusionCriteria []string `json:"inclusionCriteria"`
						ExclusionCriteria []string `json:"exclusionCriteria"`
					}{
						InclusionCriteria: []string{"default"},
						ExclusionCriteria: []string{"openshift.*"},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := GeneratePrometheusRule(tt.configData)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, rightsizing.WorkloadPrometheusRuleName, rule.Name)
				assert.Equal(t, rightsizing.MonitoringNamespace, rule.Namespace)
				require.Len(t, rule.Spec.Groups, 2)

				assert.Equal(t, "acm-right-sizing-workload-5m.rules", rule.Spec.Groups[0].Name)
				assert.Equal(t, "acm-right-sizing-workload-1d.rules", rule.Spec.Groups[1].Name)
			}
		})
	}
}

func TestGeneratePrometheusRule_IncludesMappingRule(t *testing.T) {
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

	rule, err := GeneratePrometheusRule(config)
	require.NoError(t, err)
	assert.Equal(t, "acm_rs:pod_workload:relabel:5m", rule.Spec.Groups[0].Rules[0].Record)
	expr := rule.Spec.Groups[0].Rules[0].Expr.String()
	assert.Contains(t, expr, `namespace=~"ns-a"`)
	assert.Contains(t, expr, `owner_kind="Job"`)
	assert.Contains(t, expr, `kube_job_owner`)
	assert.Contains(t, expr, `owner_kind="CronJob"`)
	assert.Contains(t, expr, `"workload_type", "ReplicaSet"`)
}

func TestGeneratePrometheusRule_IncludesLimitRules(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			NamespaceFilterCriteria: struct {
				InclusionCriteria []string `json:"inclusionCriteria"`
				ExclusionCriteria []string `json:"exclusionCriteria"`
			}{
				ExclusionCriteria: []string{"openshift.*"},
			},
			RecommendationPercentage: 110,
		},
	}

	rule, err := GeneratePrometheusRule(config)
	require.NoError(t, err)

	recordNames5m := make(map[string]string)
	for _, r := range rule.Spec.Groups[0].Rules {
		recordNames5m[r.Record] = r.Expr.String()
	}
	recordNames1d := make(map[string]string)
	for _, r := range rule.Spec.Groups[1].Rules {
		recordNames1d[r.Record] = r.Expr.String()
	}

	for _, name := range []string{
		"acm_rs:pod:cpu_limit:5m",
		"acm_rs:pod:memory_limit:5m",
		"acm_rs:workload:cpu_limit:5m",
		"acm_rs:workload:memory_limit:5m",
	} {
		expr, ok := recordNames5m[name]
		assert.True(t, ok, "5m rule %q must be present", name)
		assert.Contains(t, expr, "kube_pod_container_resource_limits", "5m rule %q must use limits metric", name)
	}

	for _, name := range []string{
		"acm_rs:pod:cpu_limit",
		"acm_rs:pod:memory_limit",
		"acm_rs:workload:cpu_limit",
		"acm_rs:workload:memory_limit",
	} {
		expr, ok := recordNames1d[name]
		assert.True(t, ok, "1d rule %q must be present", name)
		assert.Contains(t, expr, name+":5m", "1d rule %q must reference its 5m counterpart", name)
	}
}

func TestDefaultRecommendationPercentage(t *testing.T) {
	configData := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			RecommendationPercentage: 0,
		},
	}

	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	found := false
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:pod:cpu_recommendation" {
				assert.Contains(t, r.Expr.String(), "110/100")
				found = true
				break
			}
		}
	}
	assert.True(t, found, "pod cpu_recommendation rule should exist")
}

func TestAllProfilesGenerated(t *testing.T) {
	configData := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	expectedProfiles := map[string]bool{
		"Max OverAll": false,
		"P99":         false,
		"P95":         false,
		"Avg":         false,
	}

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:pod:cpu_recommendation" {
				profile := r.Labels["profile"]
				if _, ok := expectedProfiles[profile]; ok {
					expectedProfiles[profile] = true
				}
			}
		}
	}

	for profile, found := range expectedProfiles {
		assert.True(t, found, "profile %q should generate pod cpu_recommendation rules", profile)
	}
}

func TestProfileAggregationExpressions(t *testing.T) {
	configData := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	profileExprs := map[string]string{
		"Max OverAll": "max_over_time(",
		"P99":         "quantile_over_time(0.99,",
		"P95":         "quantile_over_time(0.95,",
		"Avg":         "avg_over_time(",
	}

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:workload:cpu_recommendation" {
				profile := r.Labels["profile"]
				if expectedPrefix, ok := profileExprs[profile]; ok {
					assert.Contains(t, r.Expr.String(), expectedPrefix,
						"profile %q should use %s", profile, expectedPrefix)
				}
			}
		}
	}
}
