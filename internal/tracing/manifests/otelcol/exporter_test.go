package otelcol

import (
	"fmt"
	"os"
	"testing"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ConfigureExportersSecrets(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		configPath                 string
		exporterName               string
		secretTarget               string
		shouldContainConfiguration bool
	}{
		{
			name:                       "simplest",
			configPath:                 "./test_data/simplest.yaml",
			exporterName:               "debug",
			shouldContainConfiguration: false,
			secretTarget:               "",
		},
		{
			name:                       "one_exporter",
			configPath:                 "./test_data/basic_otelhttp.yaml",
			exporterName:               "otlphttp",
			secretTarget:               "otlphttp",
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
					Name:      fmt.Sprintf("tracing-%s-auth", tc.secretTarget),
					Namespace: "cluster-1",
				},
				Data: map[string][]byte{
					"tls.crt": []byte("data"),
					"ca.crt":  []byte("data"),
					"tls.key": []byte("data"),
				},
			}

			err = ConfigureExportersSecrets(&otelCol, addon.Endpoint(tc.secretTarget), secret)
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
