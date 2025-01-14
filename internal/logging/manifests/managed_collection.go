package manifests

import (
	"encoding/json"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const managedCollectionServiceAccount = "mcoa-logging-managed-collector"

func buildManagedCLFSpec(opts Options) (loggingv1.ClusterLogForwarderSpec, error) {
	sa := loggingv1.ServiceAccount{
		Name: managedCollectionServiceAccount,
	}
	outputs := []loggingv1.OutputSpec{
		{
			Name: "hub-lokistack",
			Type: loggingv1.OutputTypeOTLP,
			OTLP: &loggingv1.OTLP{
				URL: opts.ManagedStack.LokiURL,
			},
			TLS: &loggingv1.OutputTLSSpec{
				// TODO(JoaoBraveCoding): currently this is required due to LokiStack not
				// being configued with mTLS
				InsecureSkipVerify: true,
				TLSSpec: loggingv1.TLSSpec{
					CA: &loggingv1.ValueReference{
						Key:        "ca.crt",
						SecretName: ManagedCollectionSecretName,
					},
					Certificate: &loggingv1.ValueReference{
						Key:        corev1.TLSCertKey,
						SecretName: ManagedCollectionSecretName,
					},
					Key: &loggingv1.SecretReference{
						Key:        corev1.TLSPrivateKeyKey,
						SecretName: ManagedCollectionSecretName,
					},
				},
			},
		},
	}
	pipelines := []loggingv1.PipelineSpec{
		{
			Name:       "infra-hub-lokistack",
			InputRefs:  []string{"infrastructure"},
			OutputRefs: []string{"hub-lokistack"},
		},
	}

	clfSpec := opts.ManagedStack.Collection.ClusterLogForwarder.Spec
	clfSpec.ManagementState = loggingv1.ManagementStateManaged
	clfSpec.ServiceAccount = sa
	clfSpec.Outputs = outputs
	clfSpec.Pipelines = pipelines

	return clfSpec, nil
}

func buildManagedCollectionSecrets(resources Options) ([]ResourceValue, error) {
	secretsValue := []ResourceValue{}
	for _, secret := range resources.ManagedStack.Collection.Secrets {
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
	for _, configmap := range resources.ManagedStack.Collection.ConfigMaps {
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

func BuildSSAClusterLogForwarder(opts Options) (*loggingv1.ClusterLogForwarder, error) {
	clfSpec, err := buildManagedCLFSpec(opts)
	if err != nil {
		return nil, err
	}
	clfSpec.ManagementState = loggingv1.ManagementStateUnmanaged

	return &loggingv1.ClusterLogForwarder{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterLogForwarder",
			APIVersion: loggingv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			// TODO(JoaoBraveCoding) should have the placement as the suffix
			Name:      addon.DefaultStackPrefix,
			Namespace: addon.InstallNamespace,
		},
		Spec: clfSpec,
	}, nil
}
