package manifests

import (
	"encoding/json"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests/otelcol"
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

func buildOtelColSpec(resources Options) (*otelv1beta1.OpenTelemetryCollectorSpec, error) {
	for _, secret := range resources.Secrets {
		if err := templateWithSecret(resources.OpenTelemetryCollector, secret); err != nil {
			return nil, err
		}
	}

	for _, configmap := range resources.ConfigMaps {
		if err := templateWithConfigMap(&resources, configmap); err != nil {
			return nil, err
		}
	}

	return &resources.OpenTelemetryCollector.Spec, nil
}

func templateWithSecret(otelCol *otelv1beta1.OpenTelemetryCollector, secret corev1.Secret) error {
	err := otelcol.ConfigureExportersSecrets(otelCol, secret, AnnotationTargetOutputName)
	if err != nil {
		return err
	}

	otelcol.ConfigureVolumes(otelCol, secret)
	otelcol.ConfigureVolumeMounts(otelCol, secret)

	return nil
}

func templateWithConfigMap(resource *Options, configmap corev1.ConfigMap) error {
	return otelcol.ConfigureExporters((*resource).OpenTelemetryCollector, configmap, resource.ClusterName, AnnotationTargetOutputName)
}
