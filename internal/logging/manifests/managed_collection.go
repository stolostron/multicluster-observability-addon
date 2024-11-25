package manifests

import (
	"encoding/json"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
)

const managedCollectionServiceAccount = "mcoa-logging-managed-collector"

func buildManagedCLFSpec(opts Options) (loggingv1.ClusterLogForwarderSpec, error) {
	return loggingv1.ClusterLogForwarderSpec{
		ServiceAccount: loggingv1.ServiceAccount{
			Name: managedCollectionServiceAccount,
		},
		Outputs: []loggingv1.OutputSpec{
			{
				Name: "hub-lokistack",
				Type: loggingv1.OutputTypeOTLP,
				OTLP: &loggingv1.OTLP{
					URL: opts.Managed.LokiURL,
				},
				TLS: &loggingv1.OutputTLSSpec{
					TLSSpec: loggingv1.TLSSpec{
						CA: &loggingv1.ValueReference{
							Key:        "ca.crt",
							SecretName: "mcoa-managed-collector-tls",
						},
						Certificate: &loggingv1.ValueReference{
							Key:        "tls.crt",
							SecretName: "mcoa-managed-collector-tls",
						},
						Key: &loggingv1.SecretReference{
							Key:        "tls.key",
							SecretName: "mcoa-managed-collector-tls",
						},
					},
				},
			},
		},
		Pipelines: []loggingv1.PipelineSpec{
			{
				Name:       "infra-hub-lokistack",
				InputRefs:  []string{"infrastructure"},
				OutputRefs: []string{"hub-lokistack"},
			},
		},
	}, nil
}

func buildManagedCollectionSecrets(resources Options) ([]ResourceValue, error) {
	secretsValue := []ResourceValue{}
	for _, secret := range resources.Managed.Collection.Secrets {
		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return secretsValue, err
		}
		secretValue := ResourceValue{
			Name: secret.Name,
			Data: string(dataJSON),
		}
		secretsValue = append(secretsValue, secretValue)
	}
	return secretsValue, nil
}

func buildManagedCollectionConfigMaps(resources Options) ([]ResourceValue, error) {
	configmapsValue := []ResourceValue{}
	for _, configmap := range resources.Managed.Collection.ConfigMaps {
		dataJSON, err := json.Marshal(configmap.Data)
		if err != nil {
			return configmapsValue, err
		}
		configmapValue := ResourceValue{
			Name: configmap.Name,
			Data: string(dataJSON),
		}
		configmapsValue = append(configmapsValue, configmapValue)
	}
	return configmapsValue, nil
}
