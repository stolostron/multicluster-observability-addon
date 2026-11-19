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
		cluster    *clusterv1.ManagedCluster
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
		{
			// The same registry-level source that the AODC path ignores is
			// applied when it comes from the ManagedCluster annotation.
			name: "cluster annotation registry override",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
					Annotations: map[string]string{
						clusterv1.ClusterImageRegistriesAnnotationKey: `{"registries":[{"mirror":"registry.example.com","source":"quay.io"}]}`,
					},
				},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "registry.example.com/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "registry.example.com/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "registry.example.com/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "registry.example.com/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "registry.example.com/prometheus/prometheus:v2.0.0",
				EndpointMonitoringOperator: "registry.example.com/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			// Disconnected cluster: the annotation's source overlaps an AODC
			// full-image pin, but the cluster can only reach its local
			// registry, so the annotation must win.
			name: "cluster annotation wins over AODC pin",
			registries: []addonapiv1beta1.ImageMirror{
				{
					Source: "quay.io/prometheus/prometheus",
					Mirror: "corp-mirror.example.com/prometheus",
				},
			},
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "disconnected-cluster",
					Annotations: map[string]string{
						clusterv1.ClusterImageRegistriesAnnotationKey: `{"registries":[{"mirror":"local-registry.dmz.corp","source":"quay.io"}]}`,
					},
				},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "local-registry.dmz.corp/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "local-registry.dmz.corp/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "local-registry.dmz.corp/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "local-registry.dmz.corp/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "local-registry.dmz.corp/prometheus/prometheus:v2.0.0",
				EndpointMonitoringOperator: "local-registry.dmz.corp/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
		{
			name: "AODC override applies when the cluster annotation does not claim the image",
			registries: []addonapiv1beta1.ImageMirror{
				{
					Source: "quay.io/prometheus/prometheus",
					Mirror: "registry.example.com/prometheus/prometheus",
				},
			},
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
					Annotations: map[string]string{
						clusterv1.ClusterImageRegistriesAnnotationKey: `{"registries":[{"mirror":"registry.stage.redhat.io/rhacm2","source":"registry.redhat.io/rhacm2"}]}`,
					},
				},
			},
			expected: ImageOverrides{
				PrometheusConfigReloader:   "quay.io/prometheus-operator/prometheus-config-reloader:v0.60.0",
				KubeRBACProxy:              "quay.io/brancz/kube-rbac-proxy:v0.14.0",
				CooPrometheusOperatorImage: "quay.io/rhobs/obo-prometheus-operator:v0.0.1",
				KubeStateMetrics:           "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0",
				NodeExporter:               "quay.io/prometheus/node-exporter:v1.0.0",
				Prometheus:                 "registry.example.com/prometheus/prometheus:v2.0.0",
				EndpointMonitoringOperator: "quay.io/stolostron/endpoint-monitoring-operator:v1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetImageOverrides(context.Background(), client, tt.registries, tt.cluster, logr.Discard())
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

// TestGetImageOverrides_OptionalImages checks that the optional images
// (thanos operator, memcached), which may be absent from the images-list,
// are resolved with the same precedence when they are present.
func TestGetImageOverrides_OptionalImages(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	imagesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ImagesConfigMapObjKey.Name,
			Namespace: ImagesConfigMapObjKey.Namespace,
		},
		Data: map[string]string{
			"prometheus_config_reloader":    "registry.redhat.io/rhacm2/prometheus-config-reloader-rhel9@sha256:dddd",
			"kube_rbac_proxy":               "registry.redhat.io/multicluster-engine/kube-rbac-proxy-rhel9@sha256:bbbb",
			"obo_prometheus_rhel9_operator": "registry.redhat.io/rhacm2/obo-prometheus-rhel9-operator@sha256:cccc",
			"kube_state_metrics":            "registry.redhat.io/rhacm2/kube-state-metrics-rhel9@sha256:eeee",
			"node_exporter":                 "registry.redhat.io/rhacm2/node-exporter-rhel9@sha256:ffff",
			"prometheus":                    "registry.redhat.io/rhacm2/prometheus-rhel9@sha256:aaaa",
			"endpoint_monitoring_operator":  "registry.redhat.io/rhacm2/endpoint-monitoring-rhel9-operator@sha256:1111",
			"thanos_operator":               "registry.redhat.io/rhacm2/thanos-rhel9-operator@sha256:2222",
			"memcached":                     "registry.redhat.io/rhacm2/memcached-rhel9@sha256:3333",
			"memcached_exporter":            "registry.redhat.io/rhacm2/memcached-exporter-rhel9@sha256:4444",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(imagesCM).Build()

	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster1",
			Annotations: map[string]string{
				clusterv1.ClusterImageRegistriesAnnotationKey: `{"registries":[{"mirror":"registry.stage.redhat.io/rhacm2","source":"registry.redhat.io/rhacm2"}]}`,
			},
		},
	}

	got, err := GetImageOverrides(context.Background(), client, nil, cluster, logr.Discard())
	require.NoError(t, err)

	assert.Equal(t, "registry.stage.redhat.io/rhacm2/thanos-rhel9-operator@sha256:2222", got.ThanosOperator)
	assert.Equal(t, "registry.stage.redhat.io/rhacm2/memcached-rhel9@sha256:3333", got.Memcached)
	assert.Equal(t, "registry.stage.redhat.io/rhacm2/memcached-exporter-rhel9@sha256:4444", got.MemcachedExporter)
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

func TestResolveImage(t *testing.T) {
	clusterRegs := &ClusterImageRegistries{
		Registries: []addonapiv1beta1.ImageMirror{
			{Source: "registry.redhat.io/rhacm2", Mirror: "registry.stage.redhat.io/rhacm2"},
		},
	}

	tests := []struct {
		name        string
		clusterRegs *ClusterImageRegistries
		registries  []addonapiv1beta1.ImageMirror
		image       string
		expected    string
	}{
		{
			name:        "annotation claims the image and wins over the AODC pin",
			clusterRegs: clusterRegs,
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "registry.redhat.io/rhacm2/prometheus-rhel9", Mirror: "corp-mirror.example.com/prometheus-rhel9"},
			},
			image:    "registry.redhat.io/rhacm2/prometheus-rhel9@sha256:abc123",
			expected: "registry.stage.redhat.io/rhacm2/prometheus-rhel9@sha256:abc123",
		},
		{
			name:        "AODC applies to an image the annotation does not claim",
			clusterRegs: clusterRegs,
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "quay.io/prometheus/prometheus", Mirror: "registry.example.com/prometheus/prometheus"},
			},
			image:    "quay.io/prometheus/prometheus:v2.0.0",
			expected: "registry.example.com/prometheus/prometheus:v2.0.0",
		},
		{
			// An identity mapping claims the image without changing it, so it
			// must still block the AODC fallback.
			name: "identity mapping claims the image and blocks the AODC pin",
			clusterRegs: &ClusterImageRegistries{
				Registries: []addonapiv1beta1.ImageMirror{
					{Source: "quay.io", Mirror: "quay.io"},
				},
			},
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "quay.io/prometheus/prometheus", Mirror: "corp-mirror.example.com/prometheus"},
			},
			image:    "quay.io/prometheus/prometheus:v2.0.0",
			expected: "quay.io/prometheus/prometheus:v2.0.0",
		},
		{
			name:        "annotation applies with no AODC registries",
			clusterRegs: clusterRegs,
			image:       "registry.redhat.io/rhacm2/prometheus-rhel9@sha256:abc123",
			expected:    "registry.stage.redhat.io/rhacm2/prometheus-rhel9@sha256:abc123",
		},
		{
			name: "AODC applies with no cluster annotation",
			registries: []addonapiv1beta1.ImageMirror{
				{Source: "quay.io/prometheus/prometheus", Mirror: "registry.example.com/prometheus/prometheus"},
			},
			image:    "quay.io/prometheus/prometheus:v2.0.0",
			expected: "registry.example.com/prometheus/prometheus:v2.0.0",
		},
		{
			name:        "other registry is unchanged",
			clusterRegs: clusterRegs,
			image:       "quay.io/prometheus/prometheus:v2.0.0",
			expected:    "quay.io/prometheus/prometheus:v2.0.0",
		},
		{
			name:        "empty image stays empty",
			clusterRegs: clusterRegs,
			image:       "",
			expected:    "",
		},
		{
			name:     "no overrides at all is a no-op",
			image:    "registry.redhat.io/rhacm2/prometheus-rhel9@sha256:abc123",
			expected: "registry.redhat.io/rhacm2/prometheus-rhel9@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ResolveImage(tt.image, tt.clusterRegs, tt.registries, logr.Discard()))
		})
	}
}
