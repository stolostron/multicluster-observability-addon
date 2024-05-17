package authentication

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// AuthenticationType defines an authentication method between two endpoints
type AuthenticationType string

// Target defines the name of an endpoint that will be available to store
// signal data
type Target string

// SecretKey defines a key pair (Name/Namespace) that points to a Secret on the
// hub cluster in the namespace of the spoke cluster
type SecretKey client.ObjectKey

// Config defines the configuration supported by the authentication package
// to adapt the secret generation to the needs of each signal
type Config struct {
	DefaultNamespace string
	TargetSecret     map[Target]SecretKey
	StaticAuthConfig manifests.StaticAuthenticationConfig
	MTLSConfig       manifests.MTLSConfig
}

// secretsProvider an implementaton of the authentication package API
type secretsProvider struct {
	k8s         client.Client
	clusterName string
	signal      addon.Signal
	Config
}

// NewSecretsProvider creates a new instance of *secretsProvider.
func NewSecretsProvider(k8s client.Client, clusterName string, signal addon.Signal, config *Config) (*secretsProvider, error) {
	secretsProvider := &secretsProvider{
		k8s:         k8s,
		clusterName: clusterName,
		signal:      signal,
	}

	if config == nil {
		return nil, kverrors.New("secrets provider missing config", "signal", signal)
	}
	secretsProvider.Config = *config

	return secretsProvider, nil
}

// GenerateSecrets requests Kubernetes secrets based on the specified
// authentication method for each target. The provided targetAuthType map
// represents a set of targets, where each key corresponds to a Target that that
// uses a specific AuthenticationType. This function returns a map with the same
// Target as keys, where the values are `SecretKey` referencing the Kubernetes
// secret created.
func (sp *secretsProvider) GenerateSecrets(ctx context.Context, targetAuthType map[Target]AuthenticationType) (map[Target]SecretKey, error) {
	secretKeys := make(map[Target]SecretKey, len(targetAuthType))
	objects := make([]client.Object, 0, len(targetAuthType))
	for targetName, authType := range targetAuthType {
		secretKey := client.ObjectKey{Name: fmt.Sprintf("%s-%s-auth", sp.signal, targetName), Namespace: sp.clusterName}
		var (
			obj client.Object
			err error
		)
		switch authType {
		case Static:
			obj, err = manifests.BuildStaticSecret(ctx, sp.k8s, secretKey, sp.StaticAuthConfig)
		case Managed:
			obj, err = manifests.BuildManagedSecret(secretKey)
		case MTLS:
			obj, err = manifests.BuildCertificate(secretKey, sp.MTLSConfig)
		case MCO:
			obj, err = manifests.BuildMCOSecret(secretKey)
		case ExistingSecret:
			obj, err = sp.getExistingSecret(ctx, sp.k8s, sp.TargetSecret[targetName].Name)
			if err != nil {
				return nil, err
			}
			secretKey = client.ObjectKeyFromObject(obj)
			secretKeys[targetName] = SecretKey(secretKey)
			continue
		default:
			return nil, kverrors.New("missing mutate implementation for authentication type", "type", authType)
		}
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
		secretKeys[targetName] = SecretKey(secretKey)
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

	err := sp.injectCA(ctx, targetAuthType, secretKeys)
	if err != nil {
		return nil, err
	}

	return secretKeys, nil
}

// FetchSecrets transforms a map of Target/SecretKey to a map of Target/Secret
func (sp *secretsProvider) FetchSecrets(ctx context.Context, targetsSecret map[Target]SecretKey) (map[Target]*corev1.Secret, error) {
	secrets := make(map[Target]*corev1.Secret, len(targetsSecret))
	for target, key := range targetsSecret {
		secret := &corev1.Secret{}
		if err := sp.k8s.Get(ctx, client.ObjectKey(key), secret, &client.GetOptions{}); err != nil {
			return secrets, err
		}
		secrets[target] = secret
	}
	return secrets, nil
}

// FetchSecretsAndAnnotate given a map of Target and SecretKey it will get the Secret from
// the hub cluster and add an annotation to it with Target. The goal of the
// annotation is to preseve the link betweeen Target and Secret.
// Note: the secret is not updated on the cluster with the annotation
func (sp *secretsProvider) FetchSecretsAndAnnotate(ctx context.Context, targetsSecret map[Target]SecretKey, targetAnnotation string) ([]corev1.Secret, error) {
	secrets := make([]corev1.Secret, 0, len(targetsSecret))
	for target, key := range targetsSecret {
		secret := &corev1.Secret{}
		if err := sp.k8s.Get(ctx, client.ObjectKey(key), secret, &client.GetOptions{}); err != nil {
			return secrets, err
		}
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[targetAnnotation] = string(target)
		secrets = append(secrets, *secret)
	}
	return secrets, nil
}

// injectCA will for Target's that requested mTLS authentication inject in the secret
// an "ca-bundle.crt" key containing the CA configured in the secretsProvider Config
func (sp *secretsProvider) injectCA(ctx context.Context, targetAuthType map[Target]AuthenticationType, targetsSecret map[Target]SecretKey) error {
	if sp.MTLSConfig.CAToInject == "" {
		return nil
	}

	objects := []client.Object{}
	for target, authType := range targetAuthType {
		switch authType {
		case MTLS:
			secret := &corev1.Secret{}
			key := client.ObjectKey(targetsSecret[target])
			if err := sp.k8s.Get(ctx, key, secret, &client.GetOptions{}); err != nil {
				return err
			}
			manifests.InjectCA(secret, sp.MTLSConfig.CAToInject)
			objects = append(objects, secret)
		}
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
	return nil
}

func (sp *secretsProvider) getExistingSecret(ctx context.Context, k client.Client, secretName string) (*corev1.Secret, error) {
	if sp.DefaultNamespace == "" {
		return nil, kverrors.New("default namespace not defined in secrets provider config")
	}
	existingSecret := &corev1.Secret{}
	key := client.ObjectKey{Name: secretName, Namespace: sp.clusterName}
	err := k.Get(ctx, key, existingSecret, &client.GetOptions{})
	if apierrors.IsNotFound(err) {
		key = client.ObjectKey{Name: secretName, Namespace: sp.DefaultNamespace}
		err = k.Get(ctx, key, existingSecret, &client.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get existing secret: %w", err)
	}
	return existingSecret, nil
}

func BuildAuthenticationMap(inputMap map[string]string) map[Target]AuthenticationType {
	result := make(map[Target]AuthenticationType, len(inputMap))

	for key, value := range inputMap {
		target := Target(key)
		authType := AuthenticationType(value)
		result[target] = authType
	}

	return result
}

func BuildAuthenticationFromAnnotations(annotations map[string]string) (map[Target]AuthenticationType, error) {
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
