package authentication

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AuthenticationType defines the type of authentication that will be used for a target.
type AuthenticationType string

// Signal defines the signal type that will be using an instance of the provisioner
type Signal string

type ProviderConfig struct {
	StaticAuthConfig StaticAuthenticationConfig
	MTLSConfig       MTLSConfig
}

// secretsProvider is a struct that holds the Kubernetes client and configuration.
type secretsProvider struct {
	client      client.Client
	clusterName string
	signal      Signal
	ProviderConfig
}

// NewSecretsProvider creates a new instance of K8sSecretGenerator.
func NewSecretsProvider(client client.Client, clusterName string, signal Signal, providerConfig *ProviderConfig) *secretsProvider {
	secretsProvider := &secretsProvider{
		client:      client,
		clusterName: clusterName,
		signal:      signal,
	}

	if providerConfig != nil {
		secretsProvider.ProviderConfig = *providerConfig
		return secretsProvider
	}

	switch signal {
	case Metrics:
		secretsProvider.ProviderConfig = metricsDefaults
	case Logging:
		secretsProvider.ProviderConfig = loggingDefaults
	case Tracing:
		secretsProvider.ProviderConfig = tracingDefaults
	}

	return secretsProvider
}

// GenerateSecrets generates Kubernetes secrets based on the specified authentication types for each target.
// The provided targetAuthType map represents a set of targets, where each key corresponds to a target that
// will receive signal data using a specific authentication type. This function returns a map with the same target
// keys, where the values are `client.ObjectKey` representing the Kubernetes secret created for each target.
func (k *secretsProvider) GenerateSecrets(targetAuthType map[string]AuthenticationType) (map[string]client.ObjectKey, error) {
	ctx := context.Background()
	secretKeys := make(map[string]client.ObjectKey, len(targetAuthType))
	for targetName, authType := range targetAuthType {
		secretKey := client.ObjectKey{Name: fmt.Sprintf("%s-%s-auth", k.signal, targetName), Namespace: k.clusterName}
		switch authType {
		case Static:
			err := k.createStaticSecret(ctx, secretKey)
			if err != nil {
				return nil, err
			}

		case Managed:
			err := k.createManagedSecret(ctx, secretKey)
			if err != nil {
				return nil, err
			}

		case MTLS:
			err := k.createMTLSSecret(ctx, secretKey)
			if err != nil {
				return nil, err
			}

		case MCO:
			err := k.createMCOSecret(ctx, secretKey)
			if err != nil {
				return nil, err
			}
		}
		secretKeys[targetName] = secretKey
	}

	return secretKeys, nil
}

// createOrUpdateSecret creates or updates a Kubernetes resource. If the resource
// already exists, it will be updated with the new data; otherwise, it will be created.
func (k *secretsProvider) createOrUpdate(ctx context.Context, obj client.Object) error {
	err := k.client.Create(ctx, obj, &client.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = k.client.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update resource %s: %w", obj.GetName(), err)
			}
			return nil
		}
		return fmt.Errorf("failed to create resource %s: %w", obj.GetName(), err)
	}
	return nil
}
