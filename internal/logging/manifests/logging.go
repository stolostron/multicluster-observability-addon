package manifests

import (
	"encoding/json"
	"slices"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	corev1 "k8s.io/api/core/v1"
)

func buildSubscriptionChannel(resources Options) string {
	adoc := resources.AddOnDeploymentConfig
	if adoc == nil || len(adoc.Spec.CustomizedVariables) == 0 {
		return defaultLoggingVersion
	}

	for _, keyvalue := range adoc.Spec.CustomizedVariables {
		if keyvalue.Name == subscriptionChannelValueKey {
			return keyvalue.Value
		}
	}
	return defaultLoggingVersion
}

func buildSecrets(resources Options) ([]SecretValue, error) {
	secretsValue := []SecretValue{}
	// Always go through map in order
	keys := make([]string, 0, len(resources.Secrets))
	for t := range resources.Secrets {
		keys = append(keys, string(t))
	}
	slices.Sort(keys)

	for _, key := range keys {
		secret := resources.Secrets[authentication.Target(key)]
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

func buildClusterLogForwarderSpec(resources Options) (*loggingv1.ClusterLogForwarderSpec, error) {
	clf := resources.ClusterLogForwarder
	for target, secret := range resources.Secrets {
		if err := templateWithSecret(&clf.Spec, target, secret); err != nil {
			return nil, err
		}
	}

	return &clf.Spec, nil
}

func templateWithSecret(spec *loggingv1.ClusterLogForwarderSpec, target authentication.Target, secret *corev1.Secret) error {
	// TODO(JoaoBraveCoding) Validate that clfOutputName actually exists
	// TODO(JoaoBraveCoding) Validate secret

	for k, output := range spec.Outputs {
		if output.Name == string(target) {
			output.Secret = &loggingv1.OutputSecretSpec{
				Name: secret.Name,
			}
			spec.Outputs[k] = output
		}
	}

	return nil
}
