package namespace

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratePrometheusRule validates namespace PrometheusRule generation across
// namespace filter configurations: default, inclusion-only, exclusion-only,
// and the mutually-exclusive error case.
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
			expectError: false,
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
			expectError: false,
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
			expectError: false,
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
				assert.Equal(t, rightsizing.NamespacePrometheusRuleName, rule.Name)
				assert.Equal(t, rightsizing.MonitoringNamespace, rule.Namespace)
				require.Len(t, rule.Spec.Groups, 4)

				// Check group names
				assert.Equal(t, "acm-right-sizing-namespace-5m.rule", rule.Spec.Groups[0].Name)
				assert.Equal(t, "acm-right-sizing-namespace-1d.rules", rule.Spec.Groups[1].Name)
				assert.Equal(t, "acm-right-sizing-cluster-5m.rule", rule.Spec.Groups[2].Name)
				assert.Equal(t, "acm-right-sizing-cluster-1d.rule", rule.Spec.Groups[3].Name)
			}
		})
	}
}

// TestDefaultRecommendationPercentage verifies that a zero RecommendationPercentage
// falls back to the default (110%) in the generated 1d recommendation rules.
func TestDefaultRecommendationPercentage(t *testing.T) {
	configData := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			RecommendationPercentage: 0, // Zero should default to 110
		},
	}

	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	// Check that the 1d rules contain the default recommendation percentage (110)
	found := false
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_recommendation" {
				assert.Contains(t, r.Expr.String(), "110/100")
				found = true
				break
			}
		}
	}
	assert.True(t, found, "cpu_recommendation rule should exist")
}

// TestAllProfilesGenerated verifies that 1d rules are generated for all recommendation profiles.
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
	}

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_recommendation" {
				profile := r.Labels["profile"]
				if _, ok := expectedProfiles[profile]; ok {
					expectedProfiles[profile] = true
				}
			}
		}
	}

	for profile, found := range expectedProfiles {
		assert.True(t, found, "profile %q should generate cpu_recommendation rules", profile)
	}
}

// TestProfileAggregationExpressions verifies that each profile uses the correct aggregation function.
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
	}

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_recommendation" {
				profile := r.Labels["profile"]
				if expectedPrefix, ok := profileExprs[profile]; ok {
					assert.Contains(t, r.Expr.String(), expectedPrefix,
						"profile %q should use %s", profile, expectedPrefix)
				}
			}
		}
	}
}

// TestCustomCpuAggregatorProfiles verifies that custom CPU aggregator profiles from ConfigMap
// generate the correct PrometheusRules with additional percentiles.
func TestCustomCpuAggregatorProfiles(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"Max OverAll", "P99", "P95", "P90", "P75"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	cpuProfiles := map[string]bool{}
	memProfiles := map[string]bool{}
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_recommendation" {
				cpuProfiles[r.Labels["profile"]] = true
			}
			if r.Record == "acm_rs:namespace:memory_recommendation" {
				memProfiles[r.Labels["profile"]] = true
			}
		}
	}

	assert.Len(t, cpuProfiles, 5, "CPU should have 5 profiles")
	assert.True(t, cpuProfiles["P90"], "CPU should have P90 profile")
	assert.True(t, cpuProfiles["P75"], "CPU should have P75 profile")
	assert.Len(t, memProfiles, 3, "Memory should keep default 3 profiles")
}

// TestDifferentCpuAndMemoryAggregators verifies that CPU and memory can have different profile sets.
func TestDifferentCpuAndMemoryAggregators(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"Max OverAll", "P99", "P95", "P90", "P75"}
	config.MemoryAggregator = []string{"Max OverAll", "P99", "P95"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	cpuRecommendationCount := 0
	memRecommendationCount := 0
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_recommendation" || r.Record == "acm_rs:cluster:cpu_recommendation" {
				cpuRecommendationCount++
			}
			if r.Record == "acm_rs:namespace:memory_recommendation" || r.Record == "acm_rs:cluster:memory_recommendation" {
				memRecommendationCount++
			}
		}
	}

	assert.Equal(t, 10, cpuRecommendationCount, "5 CPU profiles x 2 levels (namespace+cluster)")
	assert.Equal(t, 6, memRecommendationCount, "3 memory profiles x 2 levels (namespace+cluster)")
}
