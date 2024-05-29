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

// AuthenticationType defines an authentication method between two targets.
type AuthenticationType string

// Target defines the name of an endpoint that will be available to store
// signal data.
type Target string

// SecretKey defines a key pair (Name/Namespace) that points to a Secret on the
// hub cluster.
type SecretKey client.ObjectKey

// secretsProider an implementaton of the authentication package API
type secretsProvider struct {
	k8s client.Client
	// configResourceNamespace is the namespace where the root config resource
	// (ClusterLogForwarder or OpenTelemetryCollector) lives.
	configResourceNamespace string
	// addonNamespace is the namespace where the ManagedClusterAddon resources
	// lives also know as the namespace of the spoke cluster
	addonNamespace string
}

// NewSecretsProvider creates a new instance of secretsProvider.
func NewSecretsProvider(k8s client.Client, configResourceNamespace string, addonNamespace string) secretsProvider {
	return secretsProvider{
		k8s:                     k8s,
		configResourceNamespace: configResourceNamespace,
		addonNamespace:          addonNamespace,
	}
}

// GenerateSecrets requests Kubernetes secrets based on the specified
// authentication method for each target. The provided annotations map should
// contain a set of annotations that correspond to a set of Targets defined in
// the root config resource (CLF or OTELCol). With these annotations this
// package will build a representation of a Target and the corresponding
// AuthenticationType.
// The "targetSecretName" parameter should contain a list of
// the Targets and Secret names. This structure will be used if the user defined
// the AuthenticationType "SecretReference".
func (sp *secretsProvider) GenerateSecrets(ctx context.Context, annotations map[string]string, targetSecretName map[Target]string) (map[Target]SecretKey, error) {
	targetSecretTypes, err := buildAuthenticationFromAnnotations(annotations)
	if err != nil {
		return nil, err
	}

	secretKeys := make(map[Target]SecretKey, len(targetSecretTypes))
	for targetName, authType := range targetSecretTypes {
		switch authType {
		case SecretReference:
			obj, err := sp.getSecretReference(ctx, targetSecretName[targetName])
			if err != nil {
				return nil, err
			}
			secretKeys[targetName] = SecretKey(client.ObjectKeyFromObject(obj))
		default:
			return nil, kverrors.New("missing mutate implementation for authentication type", "type", authType)
		}
	}

	return secretKeys, nil
}

// FetchSecrets fetch the secrets from their keys and returns them as a map of Target/Secret
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

// getSecretReference given a secret name this function will return a secret
// with the same name in the namespace of the addon (spoke cluster).
// Alternatively if not secret with such name exists in the addon namespace it
// will return a secret with the same name in the namespace of the
// configResource.
// This function will return an error if no secret with such name is found in
// both namespaces.
func (sp *secretsProvider) getSecretReference(ctx context.Context, secretName string) (*corev1.Secret, error) {
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

// buildAuthenticationFromAnnotations given a set of annotations this function
// will return a map that has as keys the Targets and values AuthenticationTypes.
// The annotation used is defined in the contant "AnnotationAuthOutput"
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
