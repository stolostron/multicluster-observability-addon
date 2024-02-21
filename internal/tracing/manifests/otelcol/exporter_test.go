package otelcol

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var annotation = "tracing.mcoa.openshift.io/target-output-name"

func Test_ConfigureExportersSecrets(t *testing.T) {
	b, err := os.ReadFile("./test_data/simplest.yaml")
	require.NoError(t, err)
	otelColConfig := string(b)
	cfg, err := ConfigFromString(otelColConfig)
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

	err = ConfigureExportersSecrets(cfg, secret, annotation)
	require.NoError(t, err)

	exportersField := cfg["exporters"]
	exporters := exportersField.(map[string]interface{})
	require.Nil(t, exporters["debug"])

	b, err = os.ReadFile("./test_data/basic_otelhttp.yaml")
	require.NoError(t, err)
	otelColConfig = string(b)
	cfg, err = ConfigFromString(otelColConfig)
	require.NoError(t, err)

	err = ConfigureExportersSecrets(cfg, secret, annotation)
	require.NoError(t, err)

	exportersField = cfg["exporters"]
	exporters = exportersField.(map[string]interface{})
	otlphttpField := exporters["otlphttp"]
	otlphttp := otlphttpField.(map[interface{}]interface{})
	require.NotNil(t, otlphttp["tls"])
}

func Test_ConfigureExportersEndpoints(t *testing.T) {
	b, err := os.ReadFile("./test_data/simplest.yaml")
	require.NoError(t, err)
	otelColConfig := string(b)
	cfg, err := ConfigFromString(otelColConfig)
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

	err = ConfigureExporters(cfg, cm, "cluster", annotation)
	require.NoError(t, err)

	exportersField := cfg["exporters"]
	exporters := exportersField.(map[string]interface{})
	require.Nil(t, exporters["debug"])

	b, err = os.ReadFile("./test_data/basic_otelhttp.yaml")
	require.NoError(t, err)
	otelColConfig = string(b)
	cfg, err = ConfigFromString(otelColConfig)
	require.NoError(t, err)

	err = ConfigureExporters(cfg, cm, "cluster", annotation)
	require.NoError(t, err)

	exportersField = cfg["exporters"]
	exporters = exportersField.(map[string]interface{})
	otlphttpField := exporters["otlphttp"]
	otlphttp := otlphttpField.(map[string]interface{})
	require.NotNil(t, otlphttp["endpoint"])
}

func Test_getExporters(t *testing.T) {
	b, err := os.ReadFile("./test_data/simplest.yaml")
	require.NoError(t, err)
	otelColConfig := string(b)
	cfg, err := ConfigFromString(otelColConfig)
	require.NoError(t, err)

	exporters, err := getExporters(cfg)
	require.NoError(t, err)
	require.Len(t, exporters, 1)

	cfg = make(map[string]interface{})
	_, err = getExporters(cfg)
	require.Error(t, err)
}

func Test_configureExporterSecrets(t *testing.T) {
	exporter := make(map[interface{}]interface{})
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
	configureExporterSecrets(&exporter, secret)
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
