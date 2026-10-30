package workload

import (
	"strings"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluationIntervals(t *testing.T) {
	for _, gen := range []struct {
		name string
		fn   func(rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error)
	}{
		{"workload", GenerateWorkloadPrometheusRule},
		{"pod", GeneratePodPrometheusRule},
	} {
		t.Run(gen.name, func(t *testing.T) {
			config := rightsizing.GetDefaultWorkloadPrometheusRuleConfig()
			baseline, err := gen.fn(rightsizing.RSConfigMapData{PrometheusRuleConfig: config})
			require.NoError(t, err)
			require.Len(t, baseline.Spec.Groups, 2)

			for _, tc := range []struct {
				name, rolling, daily, wantRolling, wantDaily string
			}{
				{"missing keys", "", "", "5m", "15m"},
				{"only rolling", "10m", "", "10m", "15m"},
				{"only daily", "", "1h", "5m", "1h"},
				{"custom intervals", "10m", "30m", "10m", "30m"},
				{"ceilings", "1h", "6h", "1h", "6h"},
				{"floors in milliseconds", "300000ms", "900000ms", "300000ms", "900000ms"},
				{"rolling coarser than daily", "30m", "15m", "30m", "15m"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cfg := config
					cfg.EvaluationInterval = tc.rolling
					cfg.EvaluationInterval1d = tc.daily
					rule, err := gen.fn(rightsizing.RSConfigMapData{PrometheusRuleConfig: cfg})
					require.NoError(t, err)
					require.Len(t, rule.Spec.Groups, 2)
					require.NotNil(t, rule.Spec.Groups[0].Interval)
					require.NotNil(t, rule.Spec.Groups[1].Interval)
					require.NotEmpty(t, rule.Spec.Groups[0].Rules)
					assert.Equal(t, monitoringv1.Duration(tc.wantRolling), *rule.Spec.Groups[0].Interval)
					assert.Equal(t, monitoringv1.Duration(tc.wantDaily), *rule.Spec.Groups[1].Interval)

					// Inventory and daily aggregation keep their expressions and identities.
					// The daily rules still aggregate a full day of the stable :5m series.
					assert.Equal(t, baseline.Spec.Groups[0].Rules[0], rule.Spec.Groups[0].Rules[0])
					assert.Equal(t, baseline.Spec.Groups[1].Rules, rule.Spec.Groups[1].Rules)
					for i, group := range rule.Spec.Groups {
						assert.Equal(t, baseline.Spec.Groups[i].Name, group.Name)
						require.Len(t, group.Rules, len(baseline.Spec.Groups[i].Rules))
						for j, r := range group.Rules {
							assert.Equal(t, baseline.Spec.Groups[i].Rules[j].Record, r.Record)
							_, parseErr := parser.ParseExpr(r.Expr.String())
							require.NoError(t, parseErr, "expression of %s", r.Record)
							if i == 0 && j > 0 {
								assert.Contains(t, r.Expr.String(), "["+tc.wantRolling+":]", "lookback of %s", r.Record)
							}
						}
					}
				})
			}
		})
	}
}

func TestSeparateRulesKeepOwnerlessPodsAndWorkloadIdentity(t *testing.T) {
	config := rightsizing.RSConfigMapData{PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig()}
	podRule, err := GeneratePodPrometheusRule(config)
	require.NoError(t, err)
	workloadRule, err := GenerateWorkloadPrometheusRule(config)
	require.NoError(t, err)
	assert.Equal(t, rightsizing.PodPrometheusRuleName, podRule.Name)
	assert.Equal(t, rightsizing.WorkloadPrometheusRuleName, workloadRule.Name)
	podExpr := podRule.Spec.Groups[0].Rules[0].Expr.String()
	assert.Contains(t, podExpr, "kube_pod_info")
	assert.Contains(t, podExpr, "unless on (namespace, pod) kube_pod_owner")
	assert.Equal(t, "acm_rs:pod:info", podRule.Spec.Groups[0].Rules[0].Record)
	assert.Contains(t, rightsizing.GenerateScrapeConfigForFeatures(false, false, false, true).Spec.Params["match[]"], `{__name__="acm_rs:pod:info"}`)
	workloadExpr := workloadRule.Spec.Groups[0].Rules[1].Expr.String()
	assert.Contains(t, workloadExpr, `workload=~".+"`)
}

func TestGenerateWorkloadPrometheusRule(t *testing.T) {
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
					CpuAggregator:            rightsizing.DefaultCpuAggregator,
					MemoryAggregator:         rightsizing.DefaultMemoryAggregator,
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
					CpuAggregator:            rightsizing.DefaultCpuAggregator,
					MemoryAggregator:         rightsizing.DefaultMemoryAggregator,
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
			rule, err := GenerateWorkloadPrometheusRule(tt.configData)
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

func TestGeneratePodPrometheusRule(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	}
	rule, err := GeneratePodPrometheusRule(config)
	require.NoError(t, err)
	assert.Equal(t, rightsizing.PodPrometheusRuleName, rule.Name)
	assert.Equal(t, rightsizing.MonitoringNamespace, rule.Namespace)
	require.Len(t, rule.Spec.Groups, 2)
	assert.Equal(t, "acm-right-sizing-pod-5m.rules", rule.Spec.Groups[0].Name)
	assert.Equal(t, "acm-right-sizing-pod-1d.rules", rule.Spec.Groups[1].Name)
}

func TestPodRulesIncludeMappingRule(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	}
	config.PrometheusRuleConfig.NamespaceFilterCriteria.InclusionCriteria = []string{"ns-a"}
	config.PrometheusRuleConfig.NamespaceFilterCriteria.ExclusionCriteria = nil

	rule, err := GeneratePodPrometheusRule(config)
	require.NoError(t, err)
	assert.Equal(t, "acm_rs:pod:info", rule.Spec.Groups[0].Rules[0].Record)
	expr := rule.Spec.Groups[0].Rules[0].Expr.String()
	assert.Contains(t, expr, `namespace=~"ns-a"`)
	assert.Contains(t, expr, `owner_kind="Job"`)
	assert.Contains(t, expr, `kube_job_owner`)
	assert.Contains(t, expr, `owner_kind="CronJob"`)
	assert.Contains(t, expr, `"workload_type", "ReplicaSet"`)
}

func TestWorkloadRulesIncludeLimitRules(t *testing.T) {
	rule, err := GenerateWorkloadPrometheusRule(rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	})
	require.NoError(t, err)

	recordNames5m := make(map[string]string)
	for _, r := range rule.Spec.Groups[0].Rules {
		recordNames5m[r.Record] = r.Expr.String()
	}

	for _, name := range []string{
		"acm_rs:workload:cpu_limit:5m",
		"acm_rs:workload:memory_limit:5m",
	} {
		expr, ok := recordNames5m[name]
		assert.True(t, ok, "5m rule %q must be present", name)
		assert.Contains(t, expr, "kube_pod_container_resource_limits", "5m rule %q must use limits metric", name)
	}

	limit1d := map[string]int{}
	for _, r := range rule.Spec.Groups[1].Rules {
		switch r.Record {
		case "acm_rs:workload:cpu_limit", "acm_rs:workload:memory_limit":
			limit1d[r.Record]++
			assert.Contains(t, r.Expr.String(), r.Record+":5m")
			assert.Equal(t, "1d", r.Labels["aggregation"])
		}
	}
	assert.Equal(t, 3, limit1d["acm_rs:workload:cpu_limit"])
	assert.Equal(t, 3, limit1d["acm_rs:workload:memory_limit"])
}

func TestPodRulesIncludeLimitRules(t *testing.T) {
	rule, err := GeneratePodPrometheusRule(rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	})
	require.NoError(t, err)

	recordNames5m := make(map[string]string)
	for _, r := range rule.Spec.Groups[0].Rules {
		recordNames5m[r.Record] = r.Expr.String()
	}

	for _, name := range []string{
		"acm_rs:pod:cpu_limit:5m",
		"acm_rs:pod:memory_limit:5m",
	} {
		expr, ok := recordNames5m[name]
		assert.True(t, ok, "5m rule %q must be present", name)
		assert.Contains(t, expr, "kube_pod_container_resource_limits", "5m rule %q must use limits metric", name)
	}

	limit1d := map[string]int{}
	for _, r := range rule.Spec.Groups[1].Rules {
		switch r.Record {
		case "acm_rs:pod:cpu_limit", "acm_rs:pod:memory_limit":
			limit1d[r.Record]++
			assert.Contains(t, r.Expr.String(), r.Record+":5m")
			assert.Equal(t, "1d", r.Labels["aggregation"])
		}
	}
	assert.Equal(t, 3, limit1d["acm_rs:pod:cpu_limit"])
	assert.Equal(t, 3, limit1d["acm_rs:pod:memory_limit"])
}

func TestAllProfilesGenerated(t *testing.T) {
	rule, err := GenerateWorkloadPrometheusRule(rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	})
	require.NoError(t, err)

	expectedProfiles := map[string]bool{
		"Max OverAll": false,
		"P99":         false,
		"P95":         false,
	}
	for _, r := range rule.Spec.Groups[1].Rules {
		if r.Record == "acm_rs:workload:cpu_recommendation" {
			if _, ok := expectedProfiles[r.Labels["profile"]]; ok {
				expectedProfiles[r.Labels["profile"]] = true
			}
		}
	}
	for profile, found := range expectedProfiles {
		assert.True(t, found, "profile %q should generate cpu_recommendation rules", profile)
	}
}

func TestProfileAggregationExpressions(t *testing.T) {
	rule, err := GenerateWorkloadPrometheusRule(rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	})
	require.NoError(t, err)

	profileExprs := map[string]string{
		"Max OverAll": "max_over_time(",
		"P99":         "quantile_over_time(0.99,",
		"P95":         "quantile_over_time(0.95,",
	}
	matched := map[string]bool{}
	for _, r := range rule.Spec.Groups[1].Rules {
		if r.Record == "acm_rs:workload:cpu_recommendation" {
			profile := r.Labels["profile"]
			if expectedPrefix, ok := profileExprs[profile]; ok {
				assert.Contains(t, r.Expr.String(), expectedPrefix)
				matched[profile] = true
			}
		}
	}
	require.Len(t, matched, len(profileExprs))
}

func TestDifferentCpuAndMemoryAggregators(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.CpuAggregator = []string{"Max OverAll", "P99", "P95", "P90"}
	config.MemoryAggregator = []string{"Max OverAll", "P95"}

	rule, err := GeneratePodPrometheusRule(rightsizing.RSConfigMapData{PrometheusRuleConfig: config})
	require.NoError(t, err)

	cpuProfiles := map[string]bool{}
	memProfiles := map[string]bool{}
	for _, r := range rule.Spec.Groups[1].Rules {
		if r.Record == "acm_rs:pod:cpu_recommendation" {
			cpuProfiles[r.Labels["profile"]] = true
		}
		if r.Record == "acm_rs:pod:memory_recommendation" {
			memProfiles[r.Labels["profile"]] = true
		}
	}
	assert.Len(t, cpuProfiles, 4)
	assert.Len(t, memProfiles, 2)
	assert.True(t, cpuProfiles["P90"])
	assert.False(t, memProfiles["P90"])
}

func TestDefaultRecommendationPercentage(t *testing.T) {
	config := rightsizing.GetDefaultRSPrometheusRuleConfig()
	config.RecommendationPercentage = 0
	rule, err := GeneratePodPrometheusRule(rightsizing.RSConfigMapData{PrometheusRuleConfig: config})
	require.NoError(t, err)

	found := false
	for _, r := range rule.Spec.Groups[1].Rules {
		if r.Record == "acm_rs:pod:cpu_recommendation" && r.Labels["profile"] == "Max OverAll" {
			assert.Contains(t, r.Expr.String(), "110/100")
			found = true
		}
	}
	assert.True(t, found, "pod cpu_recommendation rule should exist")
}

func TestPhaseFilterExcludesNonRunningPods(t *testing.T) {
	config := rightsizing.RSConfigMapData{
		PrometheusRuleConfig: rightsizing.GetDefaultRSPrometheusRuleConfig(),
	}

	for _, gen := range []struct {
		name string
		fn   func(rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error)
	}{
		{"workload", GenerateWorkloadPrometheusRule},
		{"pod", GeneratePodPrometheusRule},
	} {
		t.Run(gen.name, func(t *testing.T) {
			rule, err := gen.fn(config)
			require.NoError(t, err)
			for _, r := range rule.Spec.Groups[0].Rules {
				expr := r.Expr.String()
				if strings.Contains(expr, "kube_pod_container_resource_requests") || strings.Contains(expr, "kube_pod_container_resource_limits") {
					assert.Contains(t, expr, `kube_pod_status_phase{phase="Running"}`,
						"rule %s should filter on Running phase to exclude completed pods", r.Record)
					assert.Contains(t, expr, `== 1`,
						"rule %s should use == 1 to drop non-running pods instead of zeroing them", r.Record)
				}
			}
		})
	}
}
