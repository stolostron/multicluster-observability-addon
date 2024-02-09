package otelcol

import (
	"github.com/ViaQ/logerr/v2/kverrors"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureExportersSecrets(cfg *map[interface{}]interface{}, secret corev1.Secret, annotation string) error {
	otelExporterName, ok := secret.Annotations[annotation]
	if !ok {
		return nil
	}

	exporters, err := getExporters(cfg)
	if err != nil {
		return err
	}

	for name, config := range *exporters {
		exporterName := name.(string)
		if otelExporterName != exporterName {
			continue
		}
		var configMap map[interface{}]interface{}
		if config == nil {
			configMap = make(map[interface{}]interface{})
			(*exporters)[otelExporterName] = configMap
		} else {
			configMap = config.(map[interface{}]interface{})
		}

		configureExporterSecrets(&configMap, secret)

	}
	return nil
}

func ConfigureExportersEndpoints(cfg *map[interface{}]interface{}, cm corev1.ConfigMap, annotation string) error {
	otelExporterName, ok := cm.Annotations[annotation]
	if !ok {
		return nil
	}

	exporters, err := getExporters(cfg)
	if err != nil {
		return err
	}

	for name, config := range *exporters {
		exporterName := name.(string)
		if otelExporterName != exporterName {
			continue
		}
		var configMap map[interface{}]interface{}
		if config == nil {
			configMap = make(map[interface{}]interface{})
			(*exporters)[otelExporterName] = configMap
		} else {
			configMap = config.(map[interface{}]interface{})
		}

		err := configureExporterEndpoint(&configMap, cm)
		if err != nil {
			return err
		}
	}
	return nil
}

func getExporters(cfg *map[interface{}]interface{}) (*map[interface{}]interface{}, error) {
	exportersField, ok := (*cfg)["exporters"]
	if !ok {
		return nil, kverrors.New("no exporters available as part of the configuration")
	}

	exporters, ok := exportersField.(map[interface{}]interface{})
	if !ok {
		return nil, kverrors.New("exporters field doesn't contain valid components")
	}
	return &exporters, nil
}

func configureExporterSecrets(exporter *map[interface{}]interface{}, secret corev1.Secret) {
	certConfig := make(map[string]interface{})
	certConfig["insecure"] = false
	certConfig["cert_file"] = "/certs/tls.crt"
	certConfig["key_file"] = "/certs/tls.key"
	certConfig["ca_file"] = "/certs/ca.crt"

	(*exporter)["tls"] = certConfig
}

func configureExporterEndpoint(exporter *map[interface{}]interface{}, cm corev1.ConfigMap) error {
	url := cm.Data["endpoint"]
	if url == "" {
		return kverrors.New("no value for 'url' in configmap", "name", cm.Name)
	}
	(*exporter)["endpoint"] = url
	return nil
}
