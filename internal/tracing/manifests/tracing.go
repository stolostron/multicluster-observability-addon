package manifests

import (
	"encoding/json"
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests/otelcol"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

func buildSecrets(resources Options) ([]SecretValue, error) {
	secretsValue := []SecretValue{}
	for _, secret := range resources.Secrets {
		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return secretsValue, err
		}
		secretValue := SecretValue{
			Name: secret.Name,
			Data: string(dataJSON),
		}
		secretsValue = append(secretsValue, secretValue)
	}
	return secretsValue, nil
}

func buildOtelColSpec(resources Options) (*otelv1alpha1.OpenTelemetryCollectorSpec, error) {
	for _, secret := range resources.Secrets {
		if err := templateWithSecret(&resources.OpenTelemetryCollector.Spec, secret); err != nil {
			return nil, err
		}
	}

	for _, configmap := range resources.ConfigMaps {
		if err := templateWithConfigMap(&resources.OpenTelemetryCollector.Spec, configmap); err != nil {
			return nil, err
		}
	}

	return &resources.OpenTelemetryCollector.Spec, nil
}

func templateWithSecret(spec *otelv1alpha1.OpenTelemetryCollectorSpec, secret corev1.Secret) error {
	cfg, err := configFromString(spec.Config)
	if err != nil {
		return nil
	}

	// iblancasa: add verifications for the exporters

	err = otelcol.ConfigureExportersSecrets(&cfg, secret, AnnotationTargetOutputName)
	if err != nil {
		return err
	}

	yamlConfig, err := yaml.Marshal(&cfg)
	if err != nil {
		return kverrors.New(fmt.Sprint("error while marshaling OTEL Configuration: %w", err))
	}
	spec.Config = string(yamlConfig)

	otelcol.ConfigureVolumes(spec, secret)
	otelcol.ConfigureVolumeMounts(spec, secret)

	return nil
}

func templateWithConfigMap(spec *otelv1alpha1.OpenTelemetryCollectorSpec, configmap corev1.ConfigMap) error {
	cfg, err := configFromString(spec.Config)
	if err != nil {
		return nil
	}
	err = otelcol.ConfigureExportersEndpoints(&cfg, configmap, AnnotationTargetOutputName)
	if err != nil {
		return nil
	}

	yamlConfig, err := yaml.Marshal(&cfg)
	if err != nil {
		return kverrors.New(fmt.Sprint("error while marshaling OTEL Configuration: %w", err))
	}
	spec.Config = string(yamlConfig)
	fmt.Println(string(yamlConfig))
	return nil
}

func configFromString(configStr string) (map[interface{}]interface{}, error) {
	config := make(map[interface{}]interface{})
	if err := yaml.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, kverrors.New("couldn't parse the opentelemetry-collector configuration")
	}

	return config, nil
}
