package rightsizing

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
				assert.NoError(t, err)
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
				assert.NoError(t, err)
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
}

func TestParseConfigMapData(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		data := map[string]string{
			"prometheusRuleConfig": `{"namespaceFilterCriteria":{"exclusionCriteria":["openshift.*"]},"recommendationPercentage":120}`,
		}
		result, err := ParseConfigMapData(data)
		assert.NoError(t, err)
		assert.Equal(t, 120, result.PrometheusRuleConfig.RecommendationPercentage)
		assert.Equal(t, []string{"openshift.*"}, result.PrometheusRuleConfig.NamespaceFilterCriteria.ExclusionCriteria)
		assert.Empty(t, result.PrometheusRuleConfig.NamespaceFilterCriteria.InclusionCriteria)
	})

	t.Run("empty data", func(t *testing.T) {
		result, err := ParseConfigMapData(map[string]string{})
		assert.NoError(t, err)
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
		assert.NoError(t, err)
		assert.Equal(t, 110, result.PrometheusRuleConfig.RecommendationPercentage)
		assert.Equal(t, []string{"openshift.*"}, result.PrometheusRuleConfig.NamespaceFilterCriteria.ExclusionCriteria)
		assert.Len(t, result.PlacementConfiguration.Spec.Predicates, 1)
		assert.Equal(t, "prod", result.PlacementConfiguration.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels["env"])
	})

	t.Run("invalid data", func(t *testing.T) {
		data := map[string]string{
			"prometheusRuleConfig": "{{invalid",
		}
		_, err := ParseConfigMapData(data)
		assert.Error(t, err)
	})
}
