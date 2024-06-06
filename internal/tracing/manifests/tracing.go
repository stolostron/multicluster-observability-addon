package manifests

import (
	"encoding/json"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	addonOtelCol "github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests/otelcol"
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
	for target, secret := range resources.Secrets {
		if err := templateWithSecret(resources.OpenTelemetryCollector, target, secret); err != nil {
			return nil, err
		}
	}

	return &resources.OpenTelemetryCollector.Spec, nil
}

func templateWithSecret(otelcol *otelv1beta1.OpenTelemetryCollector, target addon.Target, secret corev1.Secret) error {
	err := addonOtelCol.ConfigureExportersSecrets(otelcol, target, secret)
	if err != nil {
		return err
	}

	addonOtelCol.ConfigureVolumes(otelcol, secret)
	addonOtelCol.ConfigureVolumeMounts(otelcol, secret)

	return nil
}
