package manifests_test

import (
	"testing"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rhobs/multicluster-observability-addon/internal/metrics/handlers"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/manifests"
)

func TestBuildValues(t *testing.T) {
	testCases := map[string]struct {
		Options     handlers.Options
		ExpectError bool
		Expect      func(t *testing.T, values manifests.MetricsValues)
	}{
		"with platform resources": {
			Options: handlers.Options{
				Platform: handlers.Collector{
					PrometheusAgent: &prometheusalpha1.PrometheusAgent{
						Spec: prometheusalpha1.PrometheusAgentSpec{
							CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
								LogLevel: "info",
							},
						},
					},
				},
			},
			Expect: func(t *testing.T, values manifests.MetricsValues) {
				assert.True(t, values.PlatformEnabled)
				assert.False(t, values.UserWorkloadsEnabled)
				assert.NotEmpty(t, values.PlatformAgentSpec)
			},
		},
		"with user workloads resources": {
			Options: handlers.Options{
				UserWorkloads: handlers.Collector{
					PrometheusAgent: &prometheusalpha1.PrometheusAgent{
						Spec: prometheusalpha1.PrometheusAgentSpec{
							CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
								LogLevel: "info",
							},
						},
					},
					ScrapeConfigs: []*prometheusalpha1.ScrapeConfig{},
					Rules:         []*prometheusv1.PrometheusRule{},
				},
			},
			Expect: func(t *testing.T, values manifests.MetricsValues) {
				assert.False(t, values.PlatformEnabled)
				assert.True(t, values.UserWorkloadsEnabled)
				assert.NotEmpty(t, values.UserWorkloadsAgentSpec)
			},
		},
		"image overrides": {
			Options: handlers.Options{
				Images: handlers.ImagesOptions{
					PrometheusOperator:       "prometheus-operator:latest",
					PrometheusConfigReloader: "prometheus-config-reloader:latest",
				},
			},
			Expect: func(t *testing.T, values manifests.MetricsValues) {
				assert.Equal(t, "prometheus-operator:latest", values.Images.PrometheusOperator)
				assert.Equal(t, "prometheus-config-reloader:latest", values.Images.PrometheusConfigReloader)
			},
		},
		"with secrets": {
			Options: handlers.Options{
				Secrets: []*corev1.Secret{
					newSecret("a"),
					newSecret("b"),
				},
			},
			Expect: func(t *testing.T, values manifests.MetricsValues) {
				assert.Len(t, values.Secrets, 2)
				assert.Equal(t, "a", values.Secrets[0].Name)
				assert.Equal(t, "b", values.Secrets[1].Name)
			},
		},
		"with configmaps": {
			Options: handlers.Options{
				ConfigMaps: []*corev1.ConfigMap{
					newConfigmap("a"),
					newConfigmap("b"),
				},
			},
			Expect: func(t *testing.T, values manifests.MetricsValues) {
				assert.Len(t, values.ConfigMaps, 2)
				assert.Equal(t, "a", values.ConfigMaps[0].Name)
				assert.Equal(t, "b", values.ConfigMaps[1].Name)
			},
		},
		"with scrape configs": {
			Options: handlers.Options{
				Platform: handlers.Collector{
					ScrapeConfigs: []*prometheusalpha1.ScrapeConfig{
						newScrapeConfig("a"),
						newScrapeConfig("b"),
					},
				},
			},
			Expect: func(t *testing.T, values manifests.MetricsValues) {
				assert.Len(t, values.ScrapeConfigs, 2)
				assert.Equal(t, values.ScrapeConfigs[0].Name, "a")
				assert.Equal(t, values.ScrapeConfigs[1].Name, "b")
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			values, err := manifests.BuildValues(tc.Options)
			if tc.ExpectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tc.Expect(t, values)
		})
	}

}

func newSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newConfigmap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newScrapeConfig(name string) *prometheusalpha1.ScrapeConfig {
	return &prometheusalpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
