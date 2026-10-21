package namespace

import (
	"strings"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/promql/parser"
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

	matched := map[string]bool{}
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_recommendation" {
				profile := r.Labels["profile"]
				if expectedPrefix, ok := profileExprs[profile]; ok {
					assert.Contains(t, r.Expr.String(), expectedPrefix,
						"profile %q should use %s", profile, expectedPrefix)
					matched[profile] = true
				}
			}
		}
	}
	require.Len(t, matched, len(profileExprs), "all profiles must be found in generated rules")
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

// TestProfileRemovalUpdatesRules verifies that shrinking the ConfigMap profile list
// removes the old profiles from generated rules.
func TestProfileRemovalUpdatesRules(t *testing.T) {
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

	expandedCpu := collectProfiles(expandedRule, "acm_rs:namespace:cpu_recommendation")
	shrunkCpu := collectProfiles(shrunkRule, "acm_rs:namespace:cpu_recommendation")
	expandedMem := collectProfiles(expandedRule, "acm_rs:namespace:memory_recommendation")
	shrunkMem := collectProfiles(shrunkRule, "acm_rs:namespace:memory_recommendation")

	assert.Len(t, expandedCpu, 5)
	assert.Len(t, shrunkCpu, 3)
	assert.NotContains(t, shrunkCpu, "P95", "P95 should be removed from CPU rules")
	assert.NotContains(t, shrunkCpu, "P75", "P75 should be removed from CPU rules")

	assert.Len(t, expandedMem, 3)
	assert.Len(t, shrunkMem, 2)
	assert.NotContains(t, shrunkMem, "P95", "P95 should be removed from memory rules")
}

// TestSingleProfileOnly verifies that a config with only one profile generates correct rules.
func TestSingleProfileOnly(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"Max OverAll"}
	config.MemoryAggregator = []string{"P99"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	cpuProfiles := collectProfiles(rule, "acm_rs:namespace:cpu_recommendation")
	memProfiles := collectProfiles(rule, "acm_rs:namespace:memory_recommendation")

	assert.Equal(t, []string{"Max OverAll"}, cpuProfiles)
	assert.Equal(t, []string{"P99"}, memProfiles)
}

// TestDynamicPercentilePromQL verifies that dynamically parsed percentiles (P90, P75)
// produce correct quantile_over_time expressions in the generated rules.
func TestDynamicPercentilePromQL(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"P90", "P75"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	expectedExprs := map[string]string{
		"P90": "quantile_over_time(0.9,",
		"P75": "quantile_over_time(0.75,",
	}

	matched := map[string]bool{}
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_usage" {
				profile := r.Labels["profile"]
				if expected, ok := expectedExprs[profile]; ok {
					assert.Contains(t, r.Expr.String(), expected,
						"profile %q should produce %s expression", profile, expected)
					matched[profile] = true
				}
			}
		}
	}
	require.Len(t, matched, len(expectedExprs), "all dynamic percentile profiles must be found")
}

// TestCustomRecommendationPercentageWithCustomProfiles verifies that a non-default
// recommendation percentage is applied correctly alongside custom profiles.
func TestCustomRecommendationPercentageWithCustomProfiles(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.RecommendationPercentage = 130
	config.CpuAggregator = []string{"P90"}
	config.MemoryAggregator = []string{"P75"}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	foundCpu, foundMem := false, false
	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_recommendation" {
				assert.Contains(t, r.Expr.String(), "130/100")
				assert.Contains(t, r.Expr.String(), "quantile_over_time(0.9,")
				assert.Equal(t, "P90", r.Labels["profile"])
				foundCpu = true
			}
			if r.Record == "acm_rs:namespace:memory_recommendation" {
				assert.Contains(t, r.Expr.String(), "130/100")
				assert.Contains(t, r.Expr.String(), "quantile_over_time(0.75,")
				assert.Equal(t, "P75", r.Labels["profile"])
				foundMem = true
			}
		}
	}
	require.True(t, foundCpu, "CPU recommendation rule must be generated")
	require.True(t, foundMem, "Memory recommendation rule must be generated")
}

// TestLabelFilterWithCustomProfiles verifies that label_env filters
// are correctly combined with custom CPU/memory profiles.
func TestLabelFilterWithCustomProfiles(t *testing.T) {
	config := rightsizing.RSPrometheusRuleConfig{
		LabelFilterCriteria: []rightsizing.RSLabelFilter{
			{
				LabelName:         "label_env",
				InclusionCriteria: []string{"prod"},
			},
		},
		RecommendationPercentage: 110,
		CpuAggregator:            []string{"Max OverAll", "P90"},
		MemoryAggregator:         []string{"Max OverAll"},
	}

	configData := rightsizing.RSConfigMapData{PrometheusRuleConfig: config}
	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)

	cpuProfiles := collectProfiles(rule, "acm_rs:namespace:cpu_recommendation")
	memProfiles := collectProfiles(rule, "acm_rs:namespace:memory_recommendation")
	assert.Len(t, cpuProfiles, 2)
	assert.Len(t, memProfiles, 1)

	for _, group := range rule.Spec.Groups {
		for _, r := range group.Rules {
			if r.Record == "acm_rs:namespace:cpu_request:5m" {
				assert.Contains(t, r.Expr.String(), "label_env")
			}
		}
	}
}

// TestNamespaceRuleCountConsistency verifies that each profile generates exactly
// 4 CPU rules and 4 memory rules at the namespace level.
func TestNamespaceRuleCountConsistency(t *testing.T) {
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
				case containsPrefix(r.Record, "acm_rs:namespace:cpu_") || containsPrefix(r.Record, "acm_rs:cluster:cpu_"):
					cpuRecords[r.Labels["profile"]]++
				case containsPrefix(r.Record, "acm_rs:namespace:memory_") || containsPrefix(r.Record, "acm_rs:cluster:memory_"):
					memRecords[r.Labels["profile"]]++
				}
			}
		}
	}

	require.Len(t, cpuRecords, 2, "should have 2 CPU profiles")
	require.Len(t, memRecords, 3, "should have 3 memory profiles")
	for profile, count := range cpuRecords {
		assert.Equal(t, 8, count, "CPU profile %q should have 8 rules (4 namespace + 4 cluster)", profile)
	}
	for profile, count := range memRecords {
		assert.Equal(t, 8, count, "Memory profile %q should have 8 rules (4 namespace + 4 cluster)", profile)
	}
}

func TestCustomEvaluationInterval(t *testing.T) {
	configData := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			NamespaceFilterCriteria: struct {
				InclusionCriteria []string `json:"inclusionCriteria"`
				ExclusionCriteria []string `json:"exclusionCriteria"`
			}{ExclusionCriteria: []string{"openshift.*"}},
			EvaluationInterval:   "10m",
			EvaluationInterval1d: "15m",
		},
	}

	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)
	require.Len(t, rule.Spec.Groups, 4)

	interval10m := monitoringv1.Duration("10m")
	interval15m := monitoringv1.Duration("15m")

	// Rolling-window groups must use the configured 10m interval.
	assert.Equal(t, &interval10m, rule.Spec.Groups[0].Interval, "namespace rolling group interval")
	assert.Equal(t, &interval10m, rule.Spec.Groups[2].Interval, "cluster rolling group interval")

	// Daily-aggregation groups must use evaluationInterval1d (15m here).
	assert.Equal(t, &interval15m, rule.Spec.Groups[1].Interval, "namespace 1d group interval")
	assert.Equal(t, &interval15m, rule.Spec.Groups[3].Interval, "cluster 1d group interval")

	// The PromQL subquery range of every rolling rule must be [10m:].
	for _, r := range rule.Spec.Groups[0].Rules {
		assert.Contains(t, r.Expr.String(), "[10m:]", "namespace rolling rule %q should use [10m:]", r.Record)
		assert.NotContains(t, r.Expr.String(), "[5m:]", "namespace rolling rule %q should not contain old [5m:]", r.Record)
	}
	for _, r := range rule.Spec.Groups[2].Rules {
		assert.Contains(t, r.Expr.String(), "[10m:]", "cluster rolling rule %q should use [10m:]", r.Record)
		assert.NotContains(t, r.Expr.String(), "[5m:]", "cluster rolling rule %q should not contain old [5m:]", r.Record)
	}
}

func TestCustomEvaluationInterval1d(t *testing.T) {
	configData := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.RSPrometheusRuleConfig{
			NamespaceFilterCriteria: struct {
				InclusionCriteria []string `json:"inclusionCriteria"`
				ExclusionCriteria []string `json:"exclusionCriteria"`
			}{ExclusionCriteria: []string{"openshift.*"}},
			EvaluationInterval1d: "30m",
			// EvaluationInterval intentionally left empty → should default to Duration5m
		},
	}

	rule, err := GeneratePrometheusRule(configData)
	require.NoError(t, err)
	require.Len(t, rule.Spec.Groups, 4)

	interval5m := monitoringv1.Duration(rightsizing.DefaultEvaluationInterval)
	interval30m := monitoringv1.Duration("30m")

	// Rolling groups must fall back to the default 5m when EvaluationInterval is unset.
	assert.Equal(t, &interval5m, rule.Spec.Groups[0].Interval, "namespace rolling group should default to 5m")
	assert.Equal(t, &interval5m, rule.Spec.Groups[2].Interval, "cluster rolling group should default to 5m")

	// Only the 1d groups should use the configured 30m.
	assert.Equal(t, &interval30m, rule.Spec.Groups[1].Interval, "namespace 1d group interval")
	assert.Equal(t, &interval30m, rule.Spec.Groups[3].Interval, "cluster 1d group interval")
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

// TestRecordNamesIgnoreEvaluationIntervals pins every recorded series to the name it has at
// the default intervals. The daily rules and the dashboards select these series by name,
// the ":5m" suffix included, so a name that followed the configured interval would leave
// them reading nothing. The daily expressions must not change either: the intervals may
// only move the group intervals and the rolling subquery ranges.
func TestRecordNamesIgnoreEvaluationIntervals(t *testing.T) {
	defaultRule, err := GeneratePrometheusRule(rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	})
	require.NoError(t, err)

	for _, intervals := range []struct{ rolling, daily string }{
		{"10m", "1h"}, {"1h", "6h"}, {"300000ms", "900000ms"},
	} {
		t.Run(intervals.rolling+"/"+intervals.daily, func(t *testing.T) {
			config := rightsizing.GetDefaultRSPrometheusRuleConfig()
			config.EvaluationInterval = intervals.rolling
			config.EvaluationInterval1d = intervals.daily
			rule, err := GeneratePrometheusRule(rightsizing.RSConfigMapData{PrometheusRuleConfig: config})
			require.NoError(t, err)

			require.Len(t, rule.Spec.Groups, len(defaultRule.Spec.Groups))
			for i, group := range rule.Spec.Groups {
				want := defaultRule.Spec.Groups[i]
				require.Equal(t, want.Name, group.Name)
				require.Len(t, group.Rules, len(want.Rules), "rules in %s", group.Name)
				for j, r := range group.Rules {
					assert.Equal(t, want.Rules[j].Record, r.Record, "rule %d in %s", j, group.Name)
					if strings.Contains(group.Name, "-1d.rule") {
						assert.Equal(t, want.Rules[j].Expr, r.Expr, "expression of %s in %s", r.Record, group.Name)
					}
				}
			}
		})
	}
}

// TestGeneratedExpressionsParse runs every generated expression through the PromQL parser.
// The configs between them reach each builder branch that changes an expression: namespace
// inclusion and exclusion, the label_env join with inclusion and with exclusion, custom
// percentile profiles, and custom intervals, including millisecond spellings. A broken
// expression would otherwise only show up on the spoke, where the rules are rejected.
func TestGeneratedExpressionsParse(t *testing.T) {
	withIntervals := func(rolling, daily string) rightsizing.RSPrometheusRuleConfig {
		config := rightsizing.GetDefaultRSPrometheusRuleConfig()
		config.EvaluationInterval = rolling
		config.EvaluationInterval1d = daily
		return config
	}

	included := withIntervals("10m", "1h")
	included.NamespaceFilterCriteria.ExclusionCriteria = nil
	included.NamespaceFilterCriteria.InclusionCriteria = []string{"team-.*", "prod"}
	included.LabelFilterCriteria = []rightsizing.RSLabelFilter{{LabelName: "label_env", InclusionCriteria: []string{"prod"}}}
	included.CpuAggregator = []string{"Max OverAll", "P90", "p50"}
	included.MemoryAggregator = []string{"P99"}

	excluded := withIntervals("1h", "6h")
	excluded.LabelFilterCriteria = []rightsizing.RSLabelFilter{{LabelName: "label_env", ExclusionCriteria: []string{"dev"}}}

	for _, tc := range []struct {
		name   string
		config rightsizing.RSPrometheusRuleConfig
	}{
		{"defaults", rightsizing.GetDefaultRSPrometheusRuleConfig()},
		{"floors in milliseconds", withIntervals("300000ms", "900000ms")},
		{"inclusion filters and custom profiles", included},
		{"label exclusion at the ceilings", excluded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := GeneratePrometheusRule(rightsizing.RSConfigMapData{PrometheusRuleConfig: tc.config})
			require.NoError(t, err)
			require.NotEmpty(t, rule.Spec.Groups)
			for _, group := range rule.Spec.Groups {
				require.NotEmpty(t, group.Rules, "rules in %s", group.Name)
				for _, r := range group.Rules {
					_, parseErr := parser.ParseExpr(r.Expr.String())
					assert.NoError(t, parseErr, "%s in %s: %s", r.Record, group.Name, r.Expr.String())
				}
			}
		})
	}
}
