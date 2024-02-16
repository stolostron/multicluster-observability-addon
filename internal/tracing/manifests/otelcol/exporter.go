package otelcol

import (
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureExportersSecrets(cfg map[string]interface{}, secret corev1.Secret, annotation string) error {
	otelExporterName, ok := secret.Annotations[annotation]
	if !ok {
		return nil
	}

	exporters, err := getExporters(cfg)
	if err != nil {
		return err
	}

	for exporterName, config := range exporters {
		if otelExporterName != exporterName {
			continue
		}
		var configMap map[interface{}]interface{}
		if config == nil {
			configMap = make(map[interface{}]interface{})
			exporters[otelExporterName] = configMap
		} else {
			configMap = config.(map[interface{}]interface{})
		}

		configureExporterSecrets(&configMap, secret)

	}
	return nil
}

func ConfigureExportersEndpoints(cfg map[string]interface{}, cm corev1.ConfigMap, annotation string) error {
	otelExporterName, ok := cm.Annotations[annotation]
	if !ok {
		return nil
	}

	exporters, err := getExporters(cfg)
	if err != nil {
		return err
	}

	for exporterName, config := range exporters {
		if otelExporterName != exporterName {
			continue
		}
		var configMap map[string]interface{}
		if config == nil {
			configMap = make(map[string]interface{})
			exporters[otelExporterName] = configMap
		} else {
			configMap = config.(map[string]interface{})
		}

		err := configureExporterEndpoint(configMap, cm)
		if err != nil {
			return err
		}
	}
	return nil
}

func getExporters(cfg map[string]interface{}) (map[string]interface{}, error) {
	exportersField, ok := cfg["exporters"]
	if !ok {
		return nil, kverrors.New("no exporters available as part of the configuration")
	}

	exporters := exportersField.(map[string]interface{})
	return exporters, nil
}

func configureExporterSecrets(exporter *map[interface{}]interface{}, secret corev1.Secret) {
	certConfig := make(map[string]interface{})
	folder := fmt.Sprintf("/%s", secret.Name)
	certConfig["insecure"] = false
	certConfig["cert_file"] = fmt.Sprintf("%s/tls.crt", folder)
	certConfig["key_file"] = fmt.Sprintf("%s/tls.key", folder)
	certConfig["ca_file"] = fmt.Sprintf("%s/ca-bundle.crt", folder)

	(*exporter)["tls"] = certConfig
}

func configureExporterEndpoint(exporter map[string]interface{}, cm corev1.ConfigMap) error {
	url := cm.Data["endpoint"]
	if url == "" {
		return kverrors.New("no value for 'endpoint' in configmap", "name", cm.Name)
	}
	exporter["endpoint"] = url
	return nil
}
