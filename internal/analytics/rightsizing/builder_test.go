package rightsizing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildNamespaceFilter(t *testing.T) {
	tests := []struct {
		name        string
		config      RSPrometheusRuleConfig
		expected    string
		expectError bool
	}{
		{
			name:     "empty filter returns default",
			config:   RSPrometheusRuleConfig{},
			expected: `namespace!=""`,
		},
		{
			name: "inclusion filter",
			config: RSPrometheusRuleConfig{
				NamespaceFilterCriteria: struct {
					InclusionCriteria []string `json:"inclusionCriteria"`
					ExclusionCriteria []string `json:"exclusionCriteria"`
				}{
					InclusionCriteria: []string{"default", "kube-system"},
				},
			},
			expected: `namespace=~"default|kube-system"`,
		},
		{
			name: "exclusion filter",
			config: RSPrometheusRuleConfig{
				NamespaceFilterCriteria: struct {
					InclusionCriteria []string `json:"inclusionCriteria"`
					ExclusionCriteria []string `json:"exclusionCriteria"`
				}{
					ExclusionCriteria: []string{"openshift.*", "kube-.*"},
				},
			},
			expected: `namespace!~"openshift.*|kube-.*"`,
		},
		{
			name: "both inclusion and exclusion returns error",
			config: RSPrometheusRuleConfig{
				NamespaceFilterCriteria: struct {
					InclusionCriteria []string `json:"inclusionCriteria"`
					ExclusionCriteria []string `json:"exclusionCriteria"`
				}{
					InclusionCriteria: []string{"default"},
					ExclusionCriteria: []string{"openshift.*"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildNamespaceFilter(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBuildLabelJoin(t *testing.T) {
	tests := []struct {
		name        string
		filters     []RSLabelFilter
		expected    string
		expectError bool
	}{
		{
			name:     "empty filters",
			filters:  nil,
			expected: "",
		},
		{
			name: "filter without label_env",
			filters: []RSLabelFilter{
				{LabelName: "label_team"},
			},
			expected: "",
		},
		{
			name: "label_env with inclusion",
			filters: []RSLabelFilter{
				{
					LabelName:         "label_env",
					InclusionCriteria: []string{"prod", "staging"},
				},
			},
			expected: `* on (namespace) group_left() (kube_namespace_labels{label_env=~"prod|staging"} or kube_namespace_labels{label_env=""})`,
		},
		{
			name: "label_env with exclusion",
			filters: []RSLabelFilter{
				{
					LabelName:         "label_env",
					ExclusionCriteria: []string{"dev", "test"},
				},
			},
			expected: `* on (namespace) group_left() (kube_namespace_labels{label_env!~"dev|test"} or kube_namespace_labels{label_env=""})`,
		},
		{
			name: "label_env with both inclusion and exclusion returns error",
			filters: []RSLabelFilter{
				{
					LabelName:         "label_env",
					InclusionCriteria: []string{"prod"},
					ExclusionCriteria: []string{"dev"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildLabelJoin(tt.filters)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetDefaultRSPrometheusRuleConfig(t *testing.T) {
	config := GetDefaultRSPrometheusRuleConfig()

	assert.Equal(t, DefaultRecommendationPercentage, config.RecommendationPercentage)
	assert.Equal(t, []string{"openshift.*"}, config.NamespaceFilterCriteria.ExclusionCriteria)
	assert.Empty(t, config.NamespaceFilterCriteria.InclusionCriteria)
	assert.Equal(t, DefaultCpuAggregator, config.CpuAggregator)
	assert.Equal(t, DefaultMemoryAggregator, config.MemoryAggregator)
}

func TestParseAggregatorNames(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantLen  int
		wantExpr map[string]string
	}{
		{
			name:    "default profiles",
			input:   []string{"Max OverAll", "P99", "P95"},
			wantLen: 3,
			wantExpr: map[string]string{
				"Max OverAll": "max_over_time(",
				"P99":         "quantile_over_time(0.99,",
				"P95":         "quantile_over_time(0.95,",
			},
		},
		{
			name:    "extended profiles with P90 and P75",
			input:   []string{"Max OverAll", "P99", "P95", "P90", "P75"},
			wantLen: 5,
			wantExpr: map[string]string{
				"Max OverAll": "max_over_time(",
				"P99":         "quantile_over_time(0.99,",
				"P95":         "quantile_over_time(0.95,",
				"P90":         "quantile_over_time(0.9,",
				"P75":         "quantile_over_time(0.75,",
			},
		},
		{
			name:    "P50 percentile",
			input:   []string{"P50"},
			wantLen: 1,
			wantExpr: map[string]string{
				"P50": "quantile_over_time(0.5,",
			},
		},
		{
			name:    "invalid names are skipped",
			input:   []string{"Max OverAll", "invalid", "P999"},
			wantLen: 1,
			wantExpr: map[string]string{
				"Max OverAll": "max_over_time(",
			},
		},
		{
			name:    "empty input",
			input:   []string{},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles := ParseAggregatorNames(tt.input)
			assert.Len(t, profiles, tt.wantLen)
			for _, p := range profiles {
				if expectedPrefix, ok := tt.wantExpr[p.Name]; ok {
					result := p.AggExpr("test_metric:5m")
					assert.Contains(t, result, expectedPrefix,
						"profile %q should produce expression with %s", p.Name, expectedPrefix)
				}
			}
		})
	}
}

func TestResolveCpuProfiles(t *testing.T) {
	t.Run("empty config uses defaults", func(t *testing.T) {
		config := RSPrometheusRuleConfig{}
		profiles := ResolveCpuProfiles(config)
		assert.Len(t, profiles, len(RecommendationProfiles))
	})

	t.Run("custom config overrides defaults", func(t *testing.T) {
		config := RSPrometheusRuleConfig{
			CpuAggregator: []string{"Max OverAll", "P99", "P95", "P90", "P75"},
		}
		profiles := ResolveCpuProfiles(config)
		assert.Len(t, profiles, 5)
		assert.Equal(t, "P90", profiles[3].Name)
		assert.Equal(t, "P75", profiles[4].Name)
	})
}

func TestResolveMemoryProfiles(t *testing.T) {
	t.Run("empty config uses defaults", func(t *testing.T) {
		config := RSPrometheusRuleConfig{}
		profiles := ResolveMemoryProfiles(config)
		assert.Len(t, profiles, len(RecommendationProfiles))
	})

	t.Run("custom config overrides defaults", func(t *testing.T) {
		config := RSPrometheusRuleConfig{
			MemoryAggregator: []string{"Max OverAll", "P99"},
		}
		profiles := ResolveMemoryProfiles(config)
		assert.Len(t, profiles, 2)
	})
}

func TestDifferentCpuAndMemoryProfiles(t *testing.T) {
	config := RSPrometheusRuleConfig{
		CpuAggregator:    []string{"Max OverAll", "P99", "P95", "P90", "P75"},
		MemoryAggregator: []string{"Max OverAll", "P99", "P95"},
	}
	cpuProfiles := ResolveCpuProfiles(config)
	memProfiles := ResolveMemoryProfiles(config)
	assert.Len(t, cpuProfiles, 5, "CPU should have 5 profiles")
	assert.Len(t, memProfiles, 3, "Memory should have 3 profiles")
}

func TestParseConfigMapData(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		data := map[string]string{
			"prometheusRuleConfig": `{"namespaceFilterCriteria":{"exclusionCriteria":["openshift.*"]},"recommendationPercentage":120}`,
		}
		result, err := ParseConfigMapData(data)
		require.NoError(t, err)
		assert.Equal(t, 120, result.PrometheusRuleConfig.RecommendationPercentage)
		assert.Equal(t, []string{"openshift.*"}, result.PrometheusRuleConfig.NamespaceFilterCriteria.ExclusionCriteria)
		assert.Empty(t, result.PrometheusRuleConfig.NamespaceFilterCriteria.InclusionCriteria)
	})

	t.Run("config with custom aggregators", func(t *testing.T) {
		data := map[string]string{
			"prometheusRuleConfig": `{"recommendationPercentage":110,"cpuAggregator":["Max OverAll","P99","P95","P90","P75"],"memoryAggregator":["Max OverAll","P99","P95"]}`,
		}
		result, err := ParseConfigMapData(data)
		require.NoError(t, err)
		assert.Equal(t, []string{"Max OverAll", "P99", "P95", "P90", "P75"}, result.PrometheusRuleConfig.CpuAggregator)
		assert.Equal(t, []string{"Max OverAll", "P99", "P95"}, result.PrometheusRuleConfig.MemoryAggregator)
	})

	t.Run("empty data", func(t *testing.T) {
		result, err := ParseConfigMapData(map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, 0, result.PrometheusRuleConfig.RecommendationPercentage)
		assert.Empty(t, result.PrometheusRuleConfig.NamespaceFilterCriteria.InclusionCriteria)
		assert.Empty(t, result.PrometheusRuleConfig.NamespaceFilterCriteria.ExclusionCriteria)
		assert.Equal(t, GetDefaultRSPlacement(), result.PlacementConfiguration)
	})

	t.Run("YAML format from MCO", func(t *testing.T) {
		data := map[string]string{
			"prometheusRuleConfig":   "namespaceFilterCriteria:\n  inclusionCriteria: []\n  exclusionCriteria:\n  - openshift.*\nlabelFilterCriteria: []\nrecommendationPercentage: 110\n",
			"placementConfiguration": "spec:\n  predicates:\n  - requiredClusterSelector:\n      labelSelector:\n        matchLabels:\n          env: prod\n",
		}
		result, err := ParseConfigMapData(data)
		require.NoError(t, err)
		assert.Equal(t, 110, result.PrometheusRuleConfig.RecommendationPercentage)
		assert.Equal(t, []string{"openshift.*"}, result.PrometheusRuleConfig.NamespaceFilterCriteria.ExclusionCriteria)
		assert.Len(t, result.PlacementConfiguration.Spec.Predicates, 1)
		assert.Equal(t, "prod", result.PlacementConfiguration.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels["env"])
	})

	t.Run("MCO placement with intstr.IntOrString as object (graceful fallback)", func(t *testing.T) {
		data := map[string]string{
			"prometheusRuleConfig":   `{"namespaceFilterCriteria":{"exclusionCriteria":["openshift.*"]},"recommendationPercentage":110}`,
			"placementConfiguration": "spec:\n  predicates: []\n  decisionstrategy:\n    groupstrategy:\n      clustersperdecisiongroup:\n        type: 0\n        intval: 0\n        strval: \"\"\n",
		}
		result, err := ParseConfigMapData(data)
		require.NoError(t, err)
		assert.Equal(t, 110, result.PrometheusRuleConfig.RecommendationPercentage)
		assert.Empty(t, result.PlacementConfiguration.Spec.Predicates)
	})

	t.Run("invalid data", func(t *testing.T) {
		data := map[string]string{
			"prometheusRuleConfig": "{{invalid",
		}
		_, err := ParseConfigMapData(data)
		assert.Error(t, err)
	})
}
