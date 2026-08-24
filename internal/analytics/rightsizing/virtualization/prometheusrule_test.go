package virtualization

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratePrometheusRule validates VM PrometheusRule generation across
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
				assert.Equal(t, rightsizing.VirtualizationPrometheusRuleName, rule.Name)
				assert.Equal(t, rightsizing.MonitoringNamespace, rule.Namespace)
				require.Len(t, rule.Spec.Groups, 4)

				assert.Equal(t, "acm-vm-right-sizing-namespace-5m.rule", rule.Spec.Groups[0].Name)
				assert.Equal(t, "acm-vm-right-sizing-namespace-1d.rules", rule.Spec.Groups[1].Name)
				assert.Equal(t, "acm-vm-right-sizing-cluster-5m.rule", rule.Spec.Groups[2].Name)
				assert.Equal(t, "acm-vm-right-sizing-cluster-1d.rule", rule.Spec.Groups[3].Name)
			}
		})
	}
}

// TestDefaultRecommendationPercentage verifies that a zero RecommendationPercentage
// falls back to the default (110%) in the generated 1d recommendation rules.
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
			if r.Record == "acm_rs_vm:namespace:cpu_recommendation" {
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
			if r.Record == "acm_rs_vm:namespace:cpu_recommendation" {
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
			if r.Record == "acm_rs_vm:namespace:cpu_recommendation" {
				profile := r.Labels["profile"]
				if expectedPrefix, ok := profileExprs[profile]; ok {
					assert.Contains(t, r.Expr.String(), expectedPrefix,
						"profile %q should use %s", profile, expectedPrefix)
				}
			}
		}
	}
}

// TestCustomCpuAggregatorVirtualization verifies that custom CPU aggregator profiles
// generate additional VM-level PrometheusRules.
func TestCustomCpuAggregatorVirtualization(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"Max OverAll", "P99", "P95", "P90", "P75"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	cpuProfiles := map[string]bool{}
	memProfiles := map[string]bool{}
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs_vm:namespace:cpu_recommendation" {
				cpuProfiles[r.Labels["profile"]] = true
			}
			if r.Record == "acm_rs_vm:namespace:memory_recommendation" {
				memProfiles[r.Labels["profile"]] = true
			}
		}
	}

	assert.Len(t, cpuProfiles, 5, "CPU should have 5 profiles including P90 and P75")
	assert.True(t, cpuProfiles["P90"])
	assert.True(t, cpuProfiles["P75"])
	assert.Len(t, memProfiles, 3, "Memory should keep default 3 profiles")
}

// TestDifferentCpuAndMemoryAggregatorsVirtualization verifies that VM rules generate
// different counts for CPU and memory when their aggregator lists differ.
func TestDifferentCpuAndMemoryAggregatorsVirtualization(t *testing.T) {
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
			if r.Record == "acm_rs_vm:namespace:cpu_recommendation" || r.Record == "acm_rs_vm:cluster:cpu_recommendation" {
				cpuRecommendationCount++
			}
			if r.Record == "acm_rs_vm:namespace:memory_recommendation" || r.Record == "acm_rs_vm:cluster:memory_recommendation" {
				memRecommendationCount++
			}
		}
	}

	assert.Equal(t, 10, cpuRecommendationCount, "5 CPU profiles x 2 levels (namespace+cluster)")
	assert.Equal(t, 6, memRecommendationCount, "3 memory profiles x 2 levels (namespace+cluster)")
}

// TestProfileRemovalVirtualization verifies that shrinking the profile list
// removes the old profiles from generated VM rules.
func TestProfileRemovalVirtualization(t *testing.T) {
	expandedConfig := rightsizing.GetDefaultRSPrometheusRuleConfig()
	expandedConfig.CpuAggregator = []string{"Max OverAll", "P99", "P95", "P90", "P75"}
	expandedConfig.MemoryAggregator = []string{"Max OverAll", "P99", "P95"}

	shrunkConfig := rightsizing.GetDefaultRSPrometheusRuleConfig()
	shrunkConfig.CpuAggregator = []string{"Max OverAll", "P99", "P90"}
	shrunkConfig.MemoryAggregator = []string{"Max OverAll", "P99"}

	expandedRule, err := GeneratePrometheusRule(rightsizing.RSConfigMapData{PrometheusRuleConfig: expandedConfig})
	require.NoError(t, err)

	shrunkRule, err := GeneratePrometheusRule(rightsizing.RSConfigMapData{PrometheusRuleConfig: shrunkConfig})
	require.NoError(t, err)

	expandedCpu := collectProfiles(expandedRule, "acm_rs_vm:namespace:cpu_recommendation")
	shrunkCpu := collectProfiles(shrunkRule, "acm_rs_vm:namespace:cpu_recommendation")
	expandedMem := collectProfiles(expandedRule, "acm_rs_vm:namespace:memory_recommendation")
	shrunkMem := collectProfiles(shrunkRule, "acm_rs_vm:namespace:memory_recommendation")

	assert.Len(t, expandedCpu, 5)
	assert.Len(t, shrunkCpu, 3)
	assert.NotContains(t, shrunkCpu, "P95", "P95 should be removed from VM CPU rules")
	assert.NotContains(t, shrunkCpu, "P75", "P75 should be removed from VM CPU rules")

	assert.Len(t, expandedMem, 3)
	assert.Len(t, shrunkMem, 2)
	assert.NotContains(t, shrunkMem, "P95", "P95 should be removed from VM memory rules")
}

// TestSingleProfileVirtualization verifies VM rule generation with a single profile per resource.
func TestSingleProfileVirtualization(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"Max OverAll"}
	config.MemoryAggregator = []string{"P99"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	cpuProfiles := collectProfiles(rule, "acm_rs_vm:namespace:cpu_recommendation")
	memProfiles := collectProfiles(rule, "acm_rs_vm:namespace:memory_recommendation")

	assert.Equal(t, []string{"Max OverAll"}, cpuProfiles)
	assert.Equal(t, []string{"P99"}, memProfiles)
}

// TestDynamicPercentilePromQLVirtualization verifies that P90 and P75 produce correct
// quantile_over_time expressions in VM rules.
func TestDynamicPercentilePromQLVirtualization(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"P90", "P75"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	expectedExprs := map[string]string{
		"P90": "quantile_over_time(0.9,",
		"P75": "quantile_over_time(0.75,",
	}

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs_vm:namespace:cpu_usage" {
				profile := r.Labels["profile"]
				if expected, ok := expectedExprs[profile]; ok {
					assert.Contains(t, r.Expr.String(), expected,
						"VM profile %q should produce %s expression", profile, expected)
				}
			}
		}
	}
}

// TestCustomRecommendationPercentageVirtualization verifies that a custom recommendation
// percentage is applied alongside custom profiles in VM rules.
func TestCustomRecommendationPercentageVirtualization(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.RecommendationPercentage = 125
	config.CpuAggregator = []string{"P90"}
	config.MemoryAggregator = []string{"P75"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs_vm:namespace:cpu_recommendation" {
				assert.Contains(t, r.Expr.String(), "125/100")
				assert.Contains(t, r.Expr.String(), "quantile_over_time(0.9,")
				assert.Equal(t, "P90", r.Labels["profile"])
			}
			if r.Record == "acm_rs_vm:namespace:memory_recommendation" {
				assert.Contains(t, r.Expr.String(), "125/100")
				assert.Contains(t, r.Expr.String(), "quantile_over_time(0.75,")
				assert.Equal(t, "P75", r.Labels["profile"])
			}
		}
	}
}

// TestVMRuleCountConsistency verifies that each profile generates
// exactly 3 CPU rules and 3 memory rules at each level (namespace and cluster).
func TestVMRuleCountConsistency(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"Max OverAll", "P99"}
	config.MemoryAggregator = []string{"Max OverAll", "P99", "P95"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	cpuRecords := map[string]int{}
	memRecords := map[string]int{}
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Labels != nil {
				switch {
				case containsPrefix(r.Record, "acm_rs_vm:namespace:cpu_") || containsPrefix(r.Record, "acm_rs_vm:cluster:cpu_"):
					cpuRecords[r.Labels["profile"]]++
				case containsPrefix(r.Record, "acm_rs_vm:namespace:memory_") || containsPrefix(r.Record, "acm_rs_vm:cluster:memory_"):
					memRecords[r.Labels["profile"]]++
				}
			}
		}
	}

	for profile, count := range cpuRecords {
		assert.Equal(t, 6, count, "VM CPU profile %q should have 6 rules (3 namespace + 3 cluster)", profile)
	}
	for profile, count := range memRecords {
		assert.Equal(t, 6, count, "VM Memory profile %q should have 6 rules (3 namespace + 3 cluster)", profile)
	}
}

func collectProfiles(rule monitoringv1.PrometheusRule, recordName string) []string {
	seen := map[string]bool{}
	var profiles []string
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == recordName {
				p := r.Labels["profile"]
				if !seen[p] {
					seen[p] = true
					profiles = append(profiles, p)
				}
			}
		}
	}
	return profiles
}

func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
