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
		{
			name:    "case insensitive percentile names",
			input:   []string{"p90", "p75"},
			wantLen: 2,
			wantExpr: map[string]string{
				"p90": "quantile_over_time(0.9,",
				"p75": "quantile_over_time(0.75,",
			},
		},
		{
			name:    "whitespace is trimmed",
			input:   []string{" Max OverAll ", " P99 "},
			wantLen: 2,
			wantExpr: map[string]string{
				"Max OverAll": "max_over_time(",
				"P99":         "quantile_over_time(0.99,",
			},
		},
		{
			name:    "boundary values P0 and P100 are rejected",
			input:   []string{"P0", "P100"},
			wantLen: 0,
		},
		{
			name:    "single Max OverAll profile",
			input:   []string{"Max OverAll"},
			wantLen: 1,
			wantExpr: map[string]string{
				"Max OverAll": "max_over_time(",
			},
		},
		{
			name:    "duplicate profiles are preserved",
			input:   []string{"P99", "P99"},
			wantLen: 2,
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

func TestParseAggregatorNamesPreservesOrder(t *testing.T) {
	input := []string{"P75", "Max OverAll", "P99", "P50", "P95"}
	profiles := ParseAggregatorNames(input)
	require.Len(t, profiles, 5)
	assert.Equal(t, "P75", profiles[0].Name)
	assert.Equal(t, "Max OverAll", profiles[1].Name)
	assert.Equal(t, "P99", profiles[2].Name)
	assert.Equal(t, "P50", profiles[3].Name)
	assert.Equal(t, "P95", profiles[4].Name)
}

func TestBuildPercentileAggregationExpr(t *testing.T) {
	tests := []struct {
		name       string
		percentile float64
		wantExpr   string
	}{
		{"P90", 0.9, `quantile_over_time(0.9, test_metric:5m[1d])`},
		{"P75", 0.75, `quantile_over_time(0.75, test_metric:5m[1d])`},
		{"P50", 0.5, `quantile_over_time(0.5, test_metric:5m[1d])`},
		{"P1", 0.01, `quantile_over_time(0.01, test_metric:5m[1d])`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := BuildPercentileAggregationExpr(tt.percentile)
			assert.Equal(t, tt.wantExpr, fn("test_metric:5m"))
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

func TestProfileRemovalScenario(t *testing.T) {
	t.Run("shrinking from 5 to 3 profiles drops removed profiles", func(t *testing.T) {
		expanded := ParseAggregatorNames([]string{"Max OverAll", "P99", "P95", "P90", "P75"})
		assert.Len(t, expanded, 5)

		shrunk := ParseAggregatorNames([]string{"Max OverAll", "P99", "P90"})
		assert.Len(t, shrunk, 3)

		shrunkNames := map[string]bool{}
		for _, p := range shrunk {
			shrunkNames[p.Name] = true
		}
		assert.True(t, shrunkNames["Max OverAll"])
		assert.True(t, shrunkNames["P99"])
		assert.True(t, shrunkNames["P90"])
		assert.False(t, shrunkNames["P95"], "P95 should be absent after removal")
		assert.False(t, shrunkNames["P75"], "P75 should be absent after removal")
	})
}

func TestBackwardCompatConfigWithoutAggregators(t *testing.T) {
	data := map[string]string{
		"prometheusRuleConfig": `{"namespaceFilterCriteria":{"exclusionCriteria":["openshift.*"]},"recommendationPercentage":110}`,
	}
	result, err := ParseConfigMapData(data)
	require.NoError(t, err)

	cpuProfiles := ResolveCpuProfiles(result.PrometheusRuleConfig)
	memProfiles := ResolveMemoryProfiles(result.PrometheusRuleConfig)
	assert.Len(t, cpuProfiles, len(RecommendationProfiles), "should fall back to defaults when aggregators absent")
	assert.Len(t, memProfiles, len(RecommendationProfiles), "should fall back to defaults when aggregators absent")
}

func TestDefaultConfigDataSerializationRoundTrip(t *testing.T) {
	configData := GetDefaultNamespaceConfigData()
	result, err := ParseConfigMapData(configData)
	require.NoError(t, err)

	assert.Equal(t, DefaultCpuAggregator, result.PrometheusRuleConfig.CpuAggregator)
	assert.Equal(t, DefaultMemoryAggregator, result.PrometheusRuleConfig.MemoryAggregator)
	assert.Equal(t, DefaultRecommendationPercentage, result.PrometheusRuleConfig.RecommendationPercentage)
}

func TestYAMLConfigMapWithAggregators(t *testing.T) {
	data := map[string]string{
		"prometheusRuleConfig": "namespaceFilterCriteria:\n  exclusionCriteria:\n  - openshift.*\nrecommendationPercentage: 110\ncpuAggregator:\n- Max OverAll\n- P99\n- P90\nmemoryAggregator:\n- Max OverAll\n- P99\n",
	}
	result, err := ParseConfigMapData(data)
	require.NoError(t, err)
	assert.Equal(t, []string{"Max OverAll", "P99", "P90"}, result.PrometheusRuleConfig.CpuAggregator)
	assert.Equal(t, []string{"Max OverAll", "P99"}, result.PrometheusRuleConfig.MemoryAggregator)
}

func TestNilInput(t *testing.T) {
	profiles := ParseAggregatorNames(nil)
	assert.Empty(t, profiles)

	config := RSPrometheusRuleConfig{}
	assert.Len(t, ResolveCpuProfiles(config), len(RecommendationProfiles))
	assert.Len(t, ResolveMemoryProfiles(config), len(RecommendationProfiles))
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

func TestValidateAggregatorNames(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		wantInvalid []string
	}{
		{
			name:        "all valid known profiles",
			input:       []string{"Max OverAll", "P99", "P95"},
			wantInvalid: nil,
		},
		{
			name:        "all valid with dynamic Pxx",
			input:       []string{"Max OverAll", "P90", "P75", "P50"},
			wantInvalid: nil,
		},
		{
			name:        "one invalid among valid",
			input:       []string{"Max OverAll", "P99", "foo"},
			wantInvalid: []string{"foo"},
		},
		{
			name:        "multiple invalid",
			input:       []string{"Max OverAll", "bar", "baz", "P99"},
			wantInvalid: []string{"bar", "baz"},
		},
		{
			name:        "all invalid",
			input:       []string{"foo", "bar", "P0", "P100"},
			wantInvalid: []string{"foo", "bar", "P0", "P100"},
		},
		{
			name:        "empty input",
			input:       []string{},
			wantInvalid: nil,
		},
		{
			name:        "nil input",
			input:       nil,
			wantInvalid: nil,
		},
		{
			name:        "case insensitive Pxx are valid",
			input:       []string{"p90", "p75"},
			wantInvalid: nil,
		},
		{
			name:        "whitespace-padded valid entries",
			input:       []string{" Max OverAll ", " P99 "},
			wantInvalid: nil,
		},
		{
			name:        "P999 is invalid",
			input:       []string{"P999"},
			wantInvalid: []string{"P999"},
		},
		{
			name:        "negative percentile is invalid",
			input:       []string{"P-5"},
			wantInvalid: []string{"P-5"},
		},
		{
			name:        "non-numeric after P is invalid",
			input:       []string{"Pabc"},
			wantInvalid: []string{"Pabc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalid := ValidateAggregatorNames(tt.input)
			assert.Equal(t, tt.wantInvalid, invalid)
		})
	}
}

func TestValidationRejectsEntireEditOnInvalid(t *testing.T) {
	t.Run("any invalid entry causes entire list to be rejected", func(t *testing.T) {
		currentConfig := []string{"P90", "P80"}
		editedConfig := []string{"P90", "P75", "M80"}

		invalid := ValidateAggregatorNames(editedConfig)
		assert.Equal(t, []string{"M80"}, invalid, "M80 should be detected as invalid")

		config := RSPrometheusRuleConfig{CpuAggregator: currentConfig}
		profiles := ResolveCpuProfiles(config)
		assert.Len(t, profiles, 2)
		assert.Equal(t, "P90", profiles[0].Name)
		assert.Equal(t, "P80", profiles[1].Name)
	})

	t.Run("all valid entries are accepted", func(t *testing.T) {
		config := RSPrometheusRuleConfig{
			CpuAggregator:    []string{"Max OverAll", "P99", "P90"},
			MemoryAggregator: []string{"Max OverAll", "P95"},
		}
		assert.Empty(t, ValidateAggregatorNames(config.CpuAggregator))
		assert.Empty(t, ValidateAggregatorNames(config.MemoryAggregator))

		cpuProfiles := ResolveCpuProfiles(config)
		memProfiles := ResolveMemoryProfiles(config)
		assert.Len(t, cpuProfiles, 3)
		assert.Len(t, memProfiles, 2)
	})

	t.Run("all invalid falls back to defaults when no previous config", func(t *testing.T) {
		editedConfig := []string{"foo", "bar"}
		invalid := ValidateAggregatorNames(editedConfig)
		assert.Len(t, invalid, 2)

		config := RSPrometheusRuleConfig{CpuAggregator: nil}
		profiles := ResolveCpuProfiles(config)
		assert.Len(t, profiles, len(RecommendationProfiles), "should fall back to defaults")
	})

	t.Run("invalid CPU does not affect valid memory", func(t *testing.T) {
		cpuEdited := []string{"Max OverAll", "bad-value"}
		memEdited := []string{"Max OverAll", "P99"}

		assert.NotEmpty(t, ValidateAggregatorNames(cpuEdited), "CPU edit has invalid entry")
		assert.Empty(t, ValidateAggregatorNames(memEdited), "memory edit is fully valid")

		previousCpu := []string{"P90", "P80"}
		config := RSPrometheusRuleConfig{
			CpuAggregator:    previousCpu,
			MemoryAggregator: memEdited,
		}
		cpuProfiles := ResolveCpuProfiles(config)
		memProfiles := ResolveMemoryProfiles(config)
		assert.Len(t, cpuProfiles, 2, "CPU should use previous valid config")
		assert.Equal(t, "P90", cpuProfiles[0].Name)
		assert.Equal(t, "P80", cpuProfiles[1].Name)
		assert.Len(t, memProfiles, 2, "memory should use the valid edit")
	})

	t.Run("valid edit with P75 added is accepted", func(t *testing.T) {
		editedConfig := []string{"P90", "P80", "P75"}
		assert.Empty(t, ValidateAggregatorNames(editedConfig))

		config := RSPrometheusRuleConfig{CpuAggregator: editedConfig}
		profiles := ResolveCpuProfiles(config)
		assert.Len(t, profiles, 3)
		assert.Equal(t, "P90", profiles[0].Name)
		assert.Equal(t, "P80", profiles[1].Name)
		assert.Equal(t, "P75", profiles[2].Name)
	})
}
