package otelcol

import (
	"os"
	"testing"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var annotation = "tracing.mcoa.openshift.io/target-output-name"

func Test_ConfigureExportersSecrets(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		configPath                 string
		exporterName               string
		shouldContainConfiguration bool
	}{
		{
			name:                       "simplest",
			configPath:                 "./test_data/simplest.yaml",
			exporterName:               "debug",
			shouldContainConfiguration: false,
		},
		{
			name:                       "one_exporter",
			configPath:                 "./test_data/basic_otelhttp.yaml",
			exporterName:               "otlphttp",
			shouldContainConfiguration: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rawConfig, err := os.ReadFile(tc.configPath)
			require.NoError(t, err)
			otelCol := otelv1beta1.OpenTelemetryCollector{}

			err = yaml.Unmarshal(rawConfig, &otelCol.Spec.Config)
			require.NoError(t, err)

			secret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tracing-otlphttp-auth",
					Namespace: "cluster-1",
					Annotations: map[string]string{
						annotation: "otlphttp",
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte("data"),
					"ca.crt":  []byte("data"),
					"tls.key": []byte("data"),
				},
			}

			err = ConfigureExportersSecrets(&otelCol, secret, annotation)
			require.NoError(t, err)
			if tc.shouldContainConfiguration {
				exporterField := otelCol.Spec.Config.Exporters.Object[tc.exporterName]
				exporterCfg := exporterField.(map[string]interface{})
				require.NotNil(t, exporterCfg["tls"])
			} else {
				require.Nil(t, otelCol.Spec.Config.Exporters.Object[tc.exporterName])
			}
		})
	}
}

func Test_ConfigureExportersEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		configPath                 string
		exporterName               string
		shouldContainConfiguration bool
	}{
		{
			name:                       "simplest",
			configPath:                 "./test_data/simplest.yaml",
			exporterName:               "debug",
			shouldContainConfiguration: false,
		},
		{
			name:                       "one_exporter",
			configPath:                 "./test_data/basic_otelhttp.yaml",
			exporterName:               "otlphttp",
			shouldContainConfiguration: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rawConfig, err := os.ReadFile(tc.configPath)
			require.NoError(t, err)
			otelCol := otelv1beta1.OpenTelemetryCollector{}

			err = yaml.Unmarshal(rawConfig, &otelCol.Spec.Config)
			require.NoError(t, err)

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tracing-auth",
					Namespace: "open-cluster-management",
					Labels: map[string]string{
						"mcoa.openshift.io/signal": "tracing",
					},
					Annotations: map[string]string{
						annotation: "otlphttp",
					},
				},
				Data: map[string]string{
					"endpoint": "http://example.namespace.svc",
				},
			}
			err = ConfigureExporters(&otelCol, cm, "cluster", annotation)
			require.NoError(t, err)
			if tc.shouldContainConfiguration {
				exporterField := otelCol.Spec.Config.Exporters.Object[tc.exporterName]
				exporterCfg := exporterField.(map[string]interface{})
				require.NotNil(t, exporterCfg["endpoint"])
			} else {
				require.Nil(t, otelCol.Spec.Config.Exporters.Object[tc.exporterName])
			}
		})
	}
}

func Test_configureExporterSecrets(t *testing.T) {
	exporter := make(map[string]interface{})
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-otlphttp-auth",
			Namespace: "cluster-1",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("data"),
			"ca.crt":  []byte("data"),
			"tls.key": []byte("data"),
		},
	}
	configureExporterSecrets(exporter, secret)
	require.NotNil(t, exporter["tls"])
}

func Test_configureExporterEndpoint(t *testing.T) {
	exporter := make(map[string]interface{})
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-auth",
			Namespace: "open-cluster-management",
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "tracing",
			},
		},
		Data: map[string]string{
			"endpoint": "http://example.namespace.svc",
		},
	}

	err := configureExporterEndpoint(exporter, cm)
	require.NoError(t, err)

	require.NotNil(t, exporter["endpoint"])

	cm = corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-auth",
			Namespace: "open-cluster-management",
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "tracing",
			},
		},
		Data: map[string]string{
			"someting": "http://example.namespace.svc",
		},
	}

	err = configureExporterEndpoint(exporter, cm)
	require.Error(t, err)
}
