package otelcol

import (
	"fmt"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureExportersSecrets(otelCol *otelv1beta1.OpenTelemetryCollector, target authentication.Target, secret corev1.Secret) error {
	for exporterName, config := range otelCol.Spec.Config.Exporters.Object {
		if string(target) != exporterName {
			continue
		}
		var configMap map[string]interface{}
		if config == nil {
			configMap = make(map[string]interface{})
		} else {
			configMap = config.(map[string]interface{})
		}

		configureExporterSecrets(configMap, secret)

		otelCol.Spec.Config.Exporters.Object[string(target)] = configMap
	}
	return nil
}

func ConfigureExporters(otelCol *otelv1beta1.OpenTelemetryCollector, cm corev1.ConfigMap, clusterName string, annotation string) error {
	otelExporterName, ok := cm.Annotations[annotation]
	if !ok {
		return nil
	}

	for exporterName, config := range otelCol.Spec.Config.Exporters.Object {
		if otelExporterName != exporterName {
			continue
		}
		var exporterConfig map[string]interface{}
		if config == nil {
			exporterConfig = make(map[string]interface{})
		} else {
			exporterConfig = config.(map[string]interface{})
		}

		otelCol.Spec.Config.Exporters.Object[otelExporterName] = exporterConfig
	}
	return nil
}

func configureExporterSecrets(exporter map[string]interface{}, secret corev1.Secret) {
	certConfig := make(map[string]interface{})
	folder := fmt.Sprintf("/%s", secret.Name)
	certConfig["insecure"] = false
	certConfig["cert_file"] = fmt.Sprintf("%s/tls.crt", folder)
	certConfig["key_file"] = fmt.Sprintf("%s/tls.key", folder)
	certConfig["ca_file"] = fmt.Sprintf("%s/ca-bundle.crt", folder)

	exporter["tls"] = certConfig
}
