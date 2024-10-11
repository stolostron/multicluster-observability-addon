package manifests

import "encoding/json"

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

func buildPrometheusAgentSpec(opts Options) (string, error) {
	agent := opts.Platform.PrometheusAgent
	agent.Spec.ServiceAccountName = "metrics-collector-agent"

	ret, err := json.Marshal(agent.Spec)
	if err != nil {
		return "", err
	}

	return string(ret), nil
}
