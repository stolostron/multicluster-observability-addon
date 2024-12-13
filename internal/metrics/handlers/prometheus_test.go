package handlers_test

import (
	"fmt"
	"reflect"
	"testing"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	handler "github.com/rhobs/multicluster-observability-addon/internal/metrics/handlers"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPrometheusAgentBuilder_EnforcedFields(t *testing.T) {
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "test-app",
		},
	}
	promAgent := &prometheusalpha1.PrometheusAgent{
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				// Enforced fields
				ExternalLabels:                  map[string]string{"cluster": "other-cluster", "custom": "label"},
				ScrapeConfigSelector:            labelSelector,
				ServiceMonitorSelector:          labelSelector,
				ServiceMonitorNamespaceSelector: labelSelector,
				PodMonitorSelector:              labelSelector,
				PodMonitorNamespaceSelector:     labelSelector,
				ProbeSelector:                   labelSelector,
				ProbeNamespaceSelector:          labelSelector,
				Version:                         "invalid",
				Image:                           ptr.To("invalid"),
				Replicas:                        ptr.To(int32(10)),
				Shards:                          ptr.To(int32(10)),
				EnableFeatures:                  []prometheusv1.EnableFeature{"tt"},
				ExternalURL:                     "http://example.com",
				ServiceAccountName:              "invalid",
				AutomountServiceAccountToken:    ptr.To(false),
				ConfigMaps:                      []string{"invalid"},
			},
		},
	}

	builder := handler.PrometheusAgentBuilder{
		Agent:               promAgent,
		Name:                "test-agent",
		RemoteWriteEndpoint: "https://example.com/write",
		ClusterName:         "test-cluster",
		ClusterID:           "test-cluster-id",
		EnvoyConfigMapName:  "envoy-config",
		EnvoyProxyImage:     "envoy:latest",
		PrometheusImage:     "prometheus:latest",
		MatchLabels:         map[string]string{"app": "test-app"},
	}

	result := builder.Build()

	assert.Equal(t, int32(1), *result.Spec.CommonPrometheusFields.Replicas)
	assert.True(t, builder.Agent.Spec.CommonPrometheusFields.ArbitraryFSAccessThroughSMs.Deny)

	// Remote write
	assert.Len(t, result.Spec.CommonPrometheusFields.Secrets, 2)
	assert.Equal(t, "https://example.com/write", result.Spec.CommonPrometheusFields.RemoteWrite[0].URL)
	assert.Contains(t, builder.Agent.Spec.CommonPrometheusFields.RemoteWrite[0].TLSConfig.CAFile, "ca.crt")
	assert.Contains(t, builder.Agent.Spec.CommonPrometheusFields.RemoteWrite[0].TLSConfig.CertFile, "tls.crt")
	assert.Contains(t, builder.Agent.Spec.CommonPrometheusFields.RemoteWrite[0].TLSConfig.KeyFile, "tls.key")
	// Check that relabelling is added to the remote write config
	assert.Equal(t, *result.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[0].Replacement, "test-cluster")
	assert.Equal(t, result.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[0].TargetLabel, "cluster")
	assert.Equal(t, *result.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[1].Replacement, "test-cluster-id")
	assert.Equal(t, result.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[1].TargetLabel, "clusterID")

	// Version and Image
	assert.Equal(t, "", result.Spec.CommonPrometheusFields.Version)
	assert.Equal(t, "prometheus:latest", *result.Spec.CommonPrometheusFields.Image)

	// Resources selectors, only ScrapeConfigNamespaceSelector is not enforced by the builder
	assert.Equal(t, builder.MatchLabels, builder.Agent.Spec.CommonPrometheusFields.ScrapeConfigSelector.MatchLabels)
	assert.Nil(t, builder.Agent.Spec.CommonPrometheusFields.ServiceMonitorNamespaceSelector)
	assert.Nil(t, builder.Agent.Spec.CommonPrometheusFields.ServiceMonitorSelector)
	assert.Nil(t, builder.Agent.Spec.CommonPrometheusFields.PodMonitorNamespaceSelector)
	assert.Nil(t, builder.Agent.Spec.CommonPrometheusFields.PodMonitorSelector)
	assert.Nil(t, builder.Agent.Spec.CommonPrometheusFields.ProbeNamespaceSelector)
	assert.Nil(t, builder.Agent.Spec.CommonPrometheusFields.ProbeSelector)

	// Envoy sidecar
	containers := builder.Agent.Spec.CommonPrometheusFields.Containers
	assert.Len(t, containers, 1)
	assert.Equal(t, "envoy", containers[0].Name)
	assert.Equal(t, "envoy:latest", containers[0].Image)
	assert.Len(t, builder.Agent.Spec.CommonPrometheusFields.Volumes, 2)
	assert.Len(t, builder.Agent.Spec.CommonPrometheusFields.VolumeMounts, 0)
}

func TestPrometheusAgentBuilder_ConfigurableFields(t *testing.T) {
	promAgent := &prometheusalpha1.PrometheusAgent{
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				ScrapeConfigNamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-app",
					},
				},
				Paused:                    true,
				ImagePullPolicy:           corev1.PullNever,
				LogLevel:                  "debug",
				LogFormat:                 "json",
				ScrapeInterval:            "1h",
				ScrapeTimeout:             "1h",
				EnableRemoteWriteReceiver: true,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("123m"),
					},
				},
				NodeSelector: map[string]string{"node": "selector"},
			},
		},
	}
	builder := handler.PrometheusAgentBuilder{
		Agent:               promAgent.DeepCopy(),
		Name:                "test-agent",
		RemoteWriteEndpoint: "https://example.com/write",
		ClusterName:         "test-cluster",
		ClusterID:           "test-cluster-id",
		EnvoyConfigMapName:  "envoy-config",
		EnvoyProxyImage:     "envoy:latest",
		PrometheusImage:     "prometheus:latest",
		MatchLabels:         map[string]string{"app": "test-app"},
	}

	builtSpec := reflect.ValueOf(builder.Build().Spec.CommonPrometheusFields)
	configuredSpec := reflect.ValueOf(promAgent.Spec.CommonPrometheusFields)

	for i := 0; i < configuredSpec.NumField(); i++ {
		field := configuredSpec.Field(i)
		fieldType := configuredSpec.Type().Field(i)
		fmt.Printf("Field %s: %v\n", fieldType.Name, field.Interface())

		if isZeroValue(field) {
			continue
		}

		// Field should be the configured value
		assert.Equal(t, field.Interface(), builtSpec.Field(i).Interface())
	}
}

func isZeroValue(v reflect.Value) bool {
	zeroValue := reflect.Zero(v.Type()).Interface()
	return reflect.DeepEqual(v.Interface(), zeroValue)
}
