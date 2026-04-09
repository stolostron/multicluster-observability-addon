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
					InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
					ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
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
					InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
					ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
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
					InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
					ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
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
	tests := []struct {
		name        string
		data        map[string]string
		expectError bool
	}{
		{
			name: "valid config",
			data: map[string]string{
				"prometheusRuleConfig": `
namespaceFilterCriteria:
  exclusionCriteria:
    - openshift.*
recommendationPercentage: 120
`,
			},
			expectError: false,
		},
		{
			name:        "empty data",
			data:        map[string]string{},
			expectError: false,
		},
		{
			name: "invalid yaml",
			data: map[string]string{
				"prometheusRuleConfig": "invalid: yaml: content:",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfigMapData(tt.data)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
