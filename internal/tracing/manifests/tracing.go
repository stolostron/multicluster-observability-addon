package manifests

import (
	"encoding/json"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
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

func buildOTELColSpec(opts Options) (*otelv1beta1.OpenTelemetryCollectorSpec, error) {
	otelColSpec := opts.OpenTelemetryCollector.Spec
	otelColSpec.ManagementState = otelv1beta1.ManagementStateManaged
	return &otelColSpec, nil
}
