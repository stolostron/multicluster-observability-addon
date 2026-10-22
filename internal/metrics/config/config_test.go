package config

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
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
		"endpoint_monitoring_operator":  "quay.io/stolostron/endpoint-monitoring-operator:v1.0.0",
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
		registries []addonapiv1beta1.ImageMirror
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
				EndpointMonitoringOperator: "quay.io/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			name: "full image override",
			registries: []addonapiv1beta1.ImageMirror{
				{
					Source: "quay.io/prometheus/prometheus",
					Mirror: "registry.example.com/prometheus/prometheus",
				},
				{
					Source: "quay.io/stolostron/endpoint-monitoring-operator",
					Mirror: "registry.example.com/stolostron/endpoint-monitoring-operator",
				},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "quay.io/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "quay.io/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "quay.io/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "quay.io/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "registry.example.com/prometheus/prometheus:v2.0.0",
				EndpointMonitoringOperator: "registry.example.com/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			name: "registry domain override - should be ignored",
			registries: []addonapiv1beta1.ImageMirror{
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
				EndpointMonitoringOperator: "quay.io/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			name: "org override - should be ignored",
			registries: []addonapiv1beta1.ImageMirror{
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
				EndpointMonitoringOperator: "quay.io/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetImageOverrides(context.Background(), client, tt.registries, logr.Discard())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseClusterImageRegistries(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *clusterv1.ManagedCluster
		expected *ClusterImageRegistries
	}{
		{
			name:     "nil cluster",
			cluster:  nil,
			expected: nil,
		},
		{
			name: "no annotations",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			expected: nil,
		},
		{
			name: "empty annotation",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{clusterv1.ClusterImageRegistriesAnnotationKey: ""},
				},
			},
			expected: nil,
		},
		{
			name: "invalid JSON",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{clusterv1.ClusterImageRegistriesAnnotationKey: "{invalid"},
				},
			},
			expected: nil,
		},
		{
			name: "valid registries",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						clusterv1.ClusterImageRegistriesAnnotationKey: `{"registries":[{"mirror":"registry.stage.redhat.io/rhacm2","source":"registry.redhat.io/rhacm2"},{"mirror":"registry.stage.redhat.io/multicluster-engine","source":"registry.redhat.io/multicluster-engine"}]}`,
					},
				},
			},
			expected: &ClusterImageRegistries{
				Registries: []addonapiv1beta1.ImageMirror{
					{Source: "registry.redhat.io/rhacm2", Mirror: "registry.stage.redhat.io/rhacm2"},
					{Source: "registry.redhat.io/multicluster-engine", Mirror: "registry.stage.redhat.io/multicluster-engine"},
				},
			},
		},
		{
			name: "registries with pull secret",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						clusterv1.ClusterImageRegistriesAnnotationKey: `{"pullSecret":"my-ns.my-secret","registries":[{"mirror":"registry.stage.redhat.io/rhacm2","source":"registry.redhat.io/rhacm2"}]}`,
					},
				},
			},
			expected: &ClusterImageRegistries{
				PullSecret: "my-ns.my-secret",
				Registries: []addonapiv1beta1.ImageMirror{
					{Source: "registry.redhat.io/rhacm2", Mirror: "registry.stage.redhat.io/rhacm2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseClusterImageRegistries(tt.cluster)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestApplyClusterRegistryOverrides(t *testing.T) {
	tests := []struct {
		name       string
		images     ImageOverrides
		registries []addonapiv1beta1.ImageMirror
		expected   ImageOverrides
	}{
		{
			name: "no registries - no change",
			images: ImageOverrides{
				Prometheus:                 "registry.redhat.io/rhacm2/prometheus:v2.0.0",
				EndpointMonitoringOperator: "registry.redhat.io/rhacm2/endpoint-monitoring-operator:v1.0.0",
			},
			registries: nil,
			expected: ImageOverrides{
				Prometheus:                 "registry.redhat.io/rhacm2/prometheus:v2.0.0",
				EndpointMonitoringOperator: "registry.redhat.io/rhacm2/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			name: "registry prefix override",
			images: ImageOverrides{
				PrometheusConfigReloader:   "registry.redhat.io/rhacm2/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "registry.redhat.io/multicluster-engine/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "registry.redhat.io/rhacm2/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "registry.redhat.io/rhacm2/kube-state-metrics:v2.0.0",
				NodeExporter:               "registry.redhat.io/rhacm2/node-exporter:v1.0.0",
				Prometheus:                 "registry.redhat.io/rhacm2/prometheus:v2.0.0",
				EndpointMonitoringOperator: "registry.redhat.io/rhacm2/endpoint-monitoring-operator:v1.0.0",
			},
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "registry.redhat.io/rhacm2", Mirror: "registry.stage.redhat.io/rhacm2"},
				{Source: "registry.redhat.io/multicluster-engine", Mirror: "registry.stage.redhat.io/multicluster-engine"},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "registry.stage.redhat.io/rhacm2/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "registry.stage.redhat.io/multicluster-engine/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "registry.stage.redhat.io/rhacm2/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "registry.stage.redhat.io/rhacm2/kube-state-metrics:v2.0.0",
				NodeExporter:               "registry.stage.redhat.io/rhacm2/node-exporter:v1.0.0",
				Prometheus:                 "registry.stage.redhat.io/rhacm2/prometheus:v2.0.0",
				EndpointMonitoringOperator: "registry.stage.redhat.io/rhacm2/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			name: "no matching prefix - no change",
			images: ImageOverrides{
				Prometheus:                 "quay.io/prometheus/prometheus:v2.0.0",
				EndpointMonitoringOperator: "quay.io/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "registry.redhat.io/rhacm2", Mirror: "registry.stage.redhat.io/rhacm2"},
			},
			expected: ImageOverrides{
				Prometheus:                 "quay.io/prometheus/prometheus:v2.0.0",
				EndpointMonitoringOperator: "quay.io/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			name: "sha256 digest images",
			images: ImageOverrides{
				EndpointMonitoringOperator: "registry.redhat.io/rhacm2/endpoint-monitoring-rhel9-operator@sha256:abc123",
			},
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "registry.redhat.io/rhacm2", Mirror: "registry.stage.redhat.io/rhacm2"},
			},
			expected: ImageOverrides{
				EndpointMonitoringOperator: "registry.stage.redhat.io/rhacm2/endpoint-monitoring-rhel9-operator@sha256:abc123",
			},
		},
		{
			name: "optional thanos images",
			images: ImageOverrides{
				ThanosOperator:    "registry.redhat.io/rhacm2/thanos-operator:v1.0.0",
				ThanosStore:       "registry.redhat.io/rhacm2/thanos-store:v1.0.0",
				Memcached:         "registry.redhat.io/rhacm2/memcached:1.6.3",
				MemcachedExporter: "registry.redhat.io/rhacm2/memcached-exporter:v0.9.0",
			},
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "registry.redhat.io/rhacm2", Mirror: "registry.stage.redhat.io/rhacm2"},
			},
			expected: ImageOverrides{
				ThanosOperator:    "registry.stage.redhat.io/rhacm2/thanos-operator:v1.0.0",
				ThanosStore:       "registry.stage.redhat.io/rhacm2/thanos-store:v1.0.0",
				Memcached:         "registry.stage.redhat.io/rhacm2/memcached:1.6.3",
				MemcachedExporter: "registry.stage.redhat.io/rhacm2/memcached-exporter:v0.9.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images := tt.images
			images.ApplyClusterRegistryOverrides(tt.registries)
			assert.Equal(t, tt.expected, images)
		})
	}
}

func TestRegistryPrefixReplace(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		mirror   string
		image    string
		expected string
	}{
		{
			name:     "matching registry prefix",
			source:   "registry.redhat.io/rhacm2",
			mirror:   "registry.stage.redhat.io/rhacm2",
			image:    "registry.redhat.io/rhacm2/endpoint-monitoring-operator:v1.0.0",
			expected: "registry.stage.redhat.io/rhacm2/endpoint-monitoring-operator:v1.0.0",
		},
		{
			name:     "no match",
			source:   "registry.redhat.io/rhacm2",
			mirror:   "registry.stage.redhat.io/rhacm2",
			image:    "quay.io/prometheus/prometheus:v2.0.0",
			expected: "quay.io/prometheus/prometheus:v2.0.0",
		},
		{
			name:     "trailing slash in source",
			source:   "registry.redhat.io/rhacm2/",
			mirror:   "registry.stage.redhat.io/rhacm2/",
			image:    "registry.redhat.io/rhacm2/prometheus:v2.0.0",
			expected: "registry.stage.redhat.io/rhacm2/prometheus:v2.0.0",
		},
		{
			name:     "empty source replaces everything before image name",
			source:   "",
			mirror:   "myregistry.io",
			image:    "registry.redhat.io/rhacm2/prometheus:v2.0.0",
			expected: "myregistry.io/prometheus:v2.0.0",
		},
		{
			name:     "empty source and mirror returns image name only",
			source:   "",
			mirror:   "",
			image:    "registry.redhat.io/rhacm2/prometheus:v2.0.0",
			expected: "prometheus:v2.0.0",
		},
		{
			name:     "digest image",
			source:   "registry.redhat.io/rhacm2",
			mirror:   "registry.stage.redhat.io/rhacm2",
			image:    "registry.redhat.io/rhacm2/endpoint-monitoring-rhel9-operator@sha256:abc123",
			expected: "registry.stage.redhat.io/rhacm2/endpoint-monitoring-rhel9-operator@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registryPrefixReplace(tt.source, tt.mirror, tt.image)
			assert.Equal(t, tt.expected, got)
		})
	}
}
