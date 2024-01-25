package authentication

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// AuthenticationType defines the type of authentication that will be used for a target.
type AuthenticationType string

// Signal defines the signal type that will be using an instance of the provisioner
type Signal string

type ProviderConfig struct {
	StaticAuthConfig manifests.StaticAuthenticationConfig
	MTLSConfig       manifests.MTLSConfig
}

// secretsProvider is a struct that holds the Kubernetes client and configuration.
type secretsProvider struct {
	k           client.Client
	clusterName string
	signal      Signal
	ProviderConfig
}

// NewSecretsProvider creates a new instance of K8sSecretGenerator.
func NewSecretsProvider(k client.Client, clusterName string, signal Signal, providerConfig *ProviderConfig) *secretsProvider {
	secretsProvider := &secretsProvider{
		k:           k,
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
func (sp *secretsProvider) GenerateSecrets(targetAuthType map[string]AuthenticationType) (map[string]client.ObjectKey, error) {
	ctx := context.Background()
	secretKeys := make(map[string]client.ObjectKey, len(targetAuthType))
	objects := make([]client.Object, len(targetAuthType))
	for targetName, authType := range targetAuthType {
		secretKey := client.ObjectKey{Name: fmt.Sprintf("%s-%s-auth", sp.signal, targetName), Namespace: sp.clusterName}
		var (
			obj client.Object
			err error
		)
		switch authType {
		case Static:
			obj, err = manifests.BuildStaticSecret(ctx, sp.k, secretKey, sp.StaticAuthConfig)
		case Managed:
			obj, err = manifests.BuildManagedSecret(secretKey)
		case MTLS:
			obj, err = manifests.BuildCertificate(secretKey, sp.MTLSConfig)
		case MCO:
			obj, err = manifests.BuildMCOSecret(secretKey)
		default:
			return nil, kverrors.New("missing mutate implementation for authentication type", "type", authType)
		}
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
		secretKeys[targetName] = secretKey
	}

	for _, obj := range objects {
		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := manifests.MutateFuncFor(obj, desired, nil)

		op, err := ctrl.CreateOrUpdate(ctx, sp.k, obj, mutateFn)
		if err != nil {
			klog.Error(err, "failed to configure resource")
			continue
		}

		msg := fmt.Sprintf("Resource has been %s", op)
		switch op {
		case ctrlutil.OperationResultNone:
			klog.Info(msg)
		default:
			klog.Info(msg)
		}
	}

	return secretKeys, nil
}
