package namespace

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
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
						InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
						ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
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
						InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
						ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
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
						InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
						ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
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
				assert.NoError(t, err)
				assert.Equal(t, rightsizing.NamespacePrometheusRuleName, rule.Name)
				assert.Equal(t, rightsizing.MonitoringNamespace, rule.Namespace)
				assert.Len(t, rule.Spec.Groups, 4)

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
	assert.NoError(t, err)

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
