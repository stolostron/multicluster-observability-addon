package authentication

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViaQ/logerr/v2/kverrors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AuthenticationType defines an authentication method between two endpoints
type AuthenticationType string

// Target defines the name of an endpoint that will be available to store
// signal data
type Target string

// SecretKey defines a key pair (Name/Namespace) that points to a Secret on the
// hub cluster in the namespace of the spoke cluster
type SecretKey client.ObjectKey

// secretsProvider an implementaton of the authentication package API
type secretsProvider struct {
	k8s                     client.Client
	configResourceNamespace string
	addonNamespace          string
}

// NewSecretsProvider creates a new instance of *secretsProvider.
func NewSecretsProvider(k8s client.Client, configResourceNamespace string, addonNamespace string) secretsProvider {
	return secretsProvider{
		k8s:                     k8s,
		configResourceNamespace: configResourceNamespace,
		addonNamespace:          addonNamespace,
	}
}

// GenerateSecrets requests Kubernetes secrets based on the specified
// authentication method for each target. The provided targetAuthType map
// represents a set of targets, where each key corresponds to a Target that that
// uses a specific AuthenticationType. This function returns a map with the same
// Target as keys, where the values are `SecretKey` referencing the Kubernetes
// secret created.
func (sp *secretsProvider) GenerateSecrets(ctx context.Context, annotations map[string]string, targetSecretName map[Target]string) (map[Target]SecretKey, error) {
	targetSecretTypes, err := buildAuthenticationFromAnnotations(annotations)
	if err != nil {
		return nil, err
	}

	secretKeys := make(map[Target]SecretKey, len(targetSecretTypes))
	for targetName, authType := range targetSecretTypes {
		switch authType {
		case SecretReference:
			obj, err := sp.discoverSecretRef(ctx, targetSecretName[targetName])
			if err != nil {
				return nil, err
			}
			secretKeys[targetName] = SecretKey(client.ObjectKeyFromObject(obj))
			continue
		default:
			return nil, kverrors.New("missing mutate implementation for authentication type", "type", authType)
		}
	}

	return secretKeys, nil
}

// FetchSecrets transforms a map of Target/SecretKey to a map of Target/Secret
func (sp *secretsProvider) FetchSecrets(ctx context.Context, targetsSecrets map[Target]SecretKey) (map[Target]corev1.Secret, error) {
	secrets := make(map[Target]corev1.Secret, len(targetsSecrets))
	for target, key := range targetsSecrets {
		secret := &corev1.Secret{}
		if err := sp.k8s.Get(ctx, client.ObjectKey(key), secret, &client.GetOptions{}); err != nil {
			return secrets, err
		}
		secrets[target] = *secret
	}
	return secrets, nil
}

func (sp *secretsProvider) discoverSecretRef(ctx context.Context, secretName string) (*corev1.Secret, error) {
	secretReference := &corev1.Secret{}
	key := client.ObjectKey{Name: secretName, Namespace: sp.addonNamespace}
	err := sp.k8s.Get(ctx, key, secretReference, &client.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		key = client.ObjectKey{Name: secretName, Namespace: sp.configResourceNamespace}
		err = sp.k8s.Get(ctx, key, secretReference, &client.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get existing secret with key %s/%s: %w", key.Namespace, key.Name, err)
		}
	case err != nil:
		return nil, fmt.Errorf("failed to get existing secret with key %s/%s: %w", key.Namespace, key.Name, err)
	}
	return secretReference, nil
}

func buildAuthenticationFromAnnotations(annotations map[string]string) (map[Target]AuthenticationType, error) {
	result := make(map[Target]AuthenticationType)
	for annotation, annValue := range annotations {
		if !strings.HasPrefix(annotation, AnnotationAuthOutput) {
			continue
		}
		split := strings.Split(annotation, "/")
		if len(split) != 2 {
			return result, kverrors.New("unable to extract output name from annotation", "annotation", annotation)
		}
		if split[1] == "" {
			return result, kverrors.New("output name not specified", "annotation", annotation)
		}
		target := Target(split[1])
		authType := AuthenticationType(annValue)
		result[target] = authType
	}

	return result, nil
}
