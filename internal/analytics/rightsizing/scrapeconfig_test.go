package rightsizing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateScrapeConfig_NeverNil(t *testing.T) {
	tests := []struct {
		name          string
		includeNS     bool
		includeVirt   bool
		expectParams  bool
		expectRelabel bool
	}{
		{
			name:          "both false returns no-op ScrapeConfig",
			includeNS:     false,
			includeVirt:   false,
			expectParams:  false,
			expectRelabel: false,
		},
		{
			name:          "namespace only",
			includeNS:     true,
			includeVirt:   false,
			expectParams:  true,
			expectRelabel: true,
		},
		{
			name:          "virtualization only",
			includeNS:     false,
			includeVirt:   true,
			expectParams:  true,
			expectRelabel: true,
		},
		{
			name:          "both features",
			includeNS:     true,
			includeVirt:   true,
			expectParams:  true,
			expectRelabel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := GenerateScrapeConfig(tt.includeNS, tt.includeVirt)

			require.NotNil(t, sc, "ScrapeConfig must never be nil")
			assert.Equal(t, ScrapeConfigName, sc.Name)
			assert.Equal(t, "ScrapeConfig", sc.Kind)
			assert.NotNil(t, sc.Spec.JobName)
			assert.NotNil(t, sc.Spec.MetricsPath)

			if tt.expectParams {
				assert.NotEmpty(t, sc.Spec.Params["match[]"])
				assert.NotEmpty(t, sc.Spec.MetricRelabelConfigs)
			} else {
				assert.Empty(t, sc.Spec.Params["match[]"])
				assert.Empty(t, sc.Spec.MetricRelabelConfigs)
			}

			assert.Empty(t, sc.Spec.StaticConfigs,
				"StaticConfigs should not be set — enrichScrapeConfigForPlatform adds them later")
		})
	}
}

func TestGenerateScrapeConfig_MetricCounts(t *testing.T) {
	nsOnly := GenerateScrapeConfig(true, false)
	virtOnly := GenerateScrapeConfig(false, true)
	both := GenerateScrapeConfig(true, true)

	assert.Len(t, nsOnly.Spec.Params["match[]"], len(NamespaceMetrics))
	assert.Len(t, virtOnly.Spec.Params["match[]"], len(VirtualizationMetrics))
	assert.Len(t, both.Spec.Params["match[]"], len(NamespaceMetrics)+len(VirtualizationMetrics))
}
