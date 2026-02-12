package config

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetImageOverrides(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	baseImages := map[string]string{
		"prometheus_config_reloader":    "quay.io/prometheus-operator/prometheus-config-reloader:v0.60.0",
		"kube_rbac_proxy":               "quay.io/brancz/kube-rbac-proxy:v0.14.0",
		"obo_prometheus_rhel9_operator": "quay.io/rhobs/obo-prometheus-operator:v0.0.1",
		"kube_state_metrics":            "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
		"node_exporter":                 "quay.io/prometheus/node-exporter:v1.0.0",
		"prometheus":                    "quay.io/prometheus/prometheus:v2.0.0",
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ImagesConfigMapObjKey.Name,
			Namespace: ImagesConfigMapObjKey.Namespace,
		},
		Data: baseImages,
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	tests := []struct {
		name       string
		registries []addonapiv1alpha1.ImageMirror
		expected   ImageOverrides
	}{
		{
			name:       "no overrides",
			registries: nil,
			expected: ImageOverrides{
				PrometheusConfigReloader:   "quay.io/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "quay.io/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "quay.io/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "quay.io/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "quay.io/prometheus/prometheus:v2.0.0",
			},
		},
		{
			name: "full image override",
			registries: []addonapiv1alpha1.ImageMirror{
				{
					Source: "quay.io/prometheus/prometheus",
					Mirror: "registry.example.com/prometheus/prometheus",
				},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "quay.io/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "quay.io/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "quay.io/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "quay.io/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "registry.example.com/prometheus/prometheus:v2.0.0",
			},
		},
		{
			name: "registry domain override - should be ignored",
			registries: []addonapiv1alpha1.ImageMirror{
				{
					Source: "quay.io",
					Mirror: "registry.example.com",
				},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "quay.io/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "quay.io/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "quay.io/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "quay.io/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "quay.io/prometheus/prometheus:v2.0.0",
			},
		},
		{
			name: "org override - should be ignored",
			registries: []addonapiv1alpha1.ImageMirror{
				{
					Source: "quay.io/prometheus",
					Mirror: "registry.example.com/prometheus",
				},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "quay.io/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "quay.io/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "quay.io/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "quay.io/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "quay.io/prometheus/prometheus:v2.0.0", // "quay.io/prometheus" is prefix of "quay.io/prometheus/prometheus" but not exact match for repo
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetImageOverrides(context.Background(), client, tt.registries, logr.Discard())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
