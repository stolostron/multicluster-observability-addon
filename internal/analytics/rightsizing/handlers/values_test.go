package handlers

import (
	"encoding/json"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildValues_EmptyScrapeConfigIncluded(t *testing.T) {
	sc := rightsizing.GenerateScrapeConfig(false, false)

	opts := Options{
		NamespaceRightSizing: ComponentOptions{
			Enabled: true,
			PrometheusRules: []*monitoringv1.PrometheusRule{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rule"},
					Spec:       monitoringv1.PrometheusRuleSpec{Groups: []monitoringv1.RuleGroup{}},
				},
			},
		},
		ScrapeConfig: sc,
	}

	vals, err := BuildValues(opts)
	require.NoError(t, err)
	require.NotNil(t, vals)
	require.NotNil(t, vals.ScrapeConfig, "empty ScrapeConfig must still be included in values")

	assert.Equal(t, rightsizing.ScrapeConfigName, vals.ScrapeConfig.Name)

	var spec cooprometheusv1alpha1.ScrapeConfigSpec
	err = json.Unmarshal([]byte(vals.ScrapeConfig.Data), &spec)
	require.NoError(t, err)

	assert.Empty(t, spec.StaticConfigs,
		"no-op ScrapeConfig must not have static configs (no targets to scrape)")
	assert.Nil(t, spec.ScrapeClassName,
		"no-op ScrapeConfig must not have a scrape class")
}

func TestBuildValues_ActiveScrapeConfigEnriched(t *testing.T) {
	sc := rightsizing.GenerateScrapeConfig(true, false)

	opts := Options{
		NamespaceRightSizing: ComponentOptions{
			Enabled: true,
			PrometheusRules: []*monitoringv1.PrometheusRule{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-rule"},
					Spec:       monitoringv1.PrometheusRuleSpec{Groups: []monitoringv1.RuleGroup{}},
				},
			},
		},
		ScrapeConfig: sc,
	}

	vals, err := BuildValues(opts)
	require.NoError(t, err)
	require.NotNil(t, vals)
	require.NotNil(t, vals.ScrapeConfig)

	var spec cooprometheusv1alpha1.ScrapeConfigSpec
	err = json.Unmarshal([]byte(vals.ScrapeConfig.Data), &spec)
	require.NoError(t, err)

	assert.NotEmpty(t, spec.StaticConfigs,
		"active ScrapeConfig must have static configs for scraping")
	assert.NotNil(t, spec.ScrapeClassName,
		"active ScrapeConfig must have a scrape class")
}

func TestBuildValues_BothDisabledReturnsNil(t *testing.T) {
	opts := Options{
		ScrapeConfig: rightsizing.GenerateScrapeConfig(false, false),
	}

	vals, err := BuildValues(opts)
	require.NoError(t, err)
	assert.Nil(t, vals, "both features disabled should return nil values")
}
