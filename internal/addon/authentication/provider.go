package authentication

import (
	"context"
	"fmt"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// AuthenticationType defines the type of authentication that will be used for a target.
type AuthenticationType string

type ProviderConfig struct {
	StaticAuthConfig manifests.StaticAuthenticationConfig
	MTLSConfig       manifests.MTLSConfig
}

// secretsProvider is a struct that holds the Kubernetes client and configuration.
type secretsProvider struct {
	k8s         client.Client
	clusterName string
	signal      addon.Signal
	ProviderConfig
}

// NewSecretsProvider creates a new instance of K8sSecretGenerator.
func NewSecretsProvider(k8s client.Client, clusterName string, signal addon.Signal, providerConfig *ProviderConfig) *secretsProvider {
	secretsProvider := &secretsProvider{
		k8s:         k8s,
		clusterName: clusterName,
		signal:      signal,
	}

	if providerConfig != nil {
		secretsProvider.ProviderConfig = *providerConfig
		return secretsProvider
	}

	switch signal {
	case addon.Metrics:
		secretsProvider.ProviderConfig = metricsDefaults
	case addon.Logging:
		secretsProvider.ProviderConfig = loggingDefaults
	case addon.Tracing:
		secretsProvider.ProviderConfig = tracingDefaults
	}

	return secretsProvider
}

// GenerateSecrets generates Kubernetes secrets based on the specified authentication types for each target.
// The provided targetAuthType map represents a set of targets, where each key corresponds to a target that
// will receive signal data using a specific authentication type. This function returns a map with the same target
// keys, where the values are `client.ObjectKey` representing the Kubernetes secret created for each target.
func (sp *secretsProvider) GenerateSecrets(targetAuthType map[string]string) (map[string]client.ObjectKey, error) {
	ctx := context.Background()
	secretKeys := make(map[string]client.ObjectKey, len(targetAuthType))
	objects := make([]client.Object, 0, len(targetAuthType))
	for targetName, authType := range targetAuthType {
		secretKey := client.ObjectKey{Name: fmt.Sprintf("%s-%s-auth", sp.signal, targetName), Namespace: sp.clusterName}
		var (
			obj client.Object
			err error
		)
		switch AuthenticationType(authType) {
		case Static:
			obj, err = manifests.BuildStaticSecret(ctx, sp.k8s, secretKey, sp.StaticAuthConfig)
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

		op, err := ctrl.CreateOrUpdate(ctx, sp.k8s, obj, mutateFn)
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

func (sp *secretsProvider) FetchSecrets(targetsSecret map[string]client.ObjectKey, targetAnnotation string) ([]corev1.Secret, error) {
	secrets := make([]corev1.Secret, 0, len(targetsSecret))
	for target, key := range targetsSecret {
		secret := &corev1.Secret{}
		if err := sp.k8s.Get(context.Background(), key, secret, &client.GetOptions{}); err != nil {
			return secrets, err
		}
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[targetAnnotation] = target
		secrets = append(secrets, *secret)
	}
	return secrets, nil
}
