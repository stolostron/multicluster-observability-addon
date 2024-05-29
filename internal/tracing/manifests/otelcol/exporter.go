package otelcol

import (
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureExportersSecrets(cfg map[string]interface{}, target authentication.Target, secret corev1.Secret) error {
	exporters, err := GetExporters(cfg)
	if err != nil {
		return err
	}

	for exporterName, config := range exporters {
		if string(target) != exporterName {
			continue
		}
		var configMap map[string]interface{}
		if config == nil {
			configMap = make(map[string]interface{})
			exporters[string(target)] = configMap
		} else {
			configMap = config.(map[string]interface{})
		}

		configureExporterSecrets(configMap, secret)

	}
	return nil
}

func GetExporters(cfg map[string]interface{}) (map[string]interface{}, error) {
	exportersField, ok := cfg["exporters"]
	if !ok {
		return nil, kverrors.New("no exporters available as part of the configuration")
	}

	exporters := exportersField.(map[string]interface{})
	return exporters, nil
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
