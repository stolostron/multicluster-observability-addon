package addon

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AuthenticationTypes defines the type of authentication that will be used for a target.
type AuthenticationTypes string

const (
	// Static represents static authentication type.
	Static AuthenticationTypes = "StaticAuthentication"
	// Managed represents managed authentication type.
	Managed AuthenticationTypes = "ManagedAuthentication"
	// MTLS represents mTLS authentication type.
	MTLS AuthenticationTypes = "mTLS"
)

// secretsProvider is a struct that holds the Kubernetes client and configuration.
type secretsProvider struct {
	client      client.Client
	clusterName string
}

// NewSecretsProvider creates a new instance of K8sSecretGenerator.
func NewSecretsProvider(client client.Client, clusterName string) *secretsProvider {
	return &secretsProvider{
		client:      client,
		clusterName: clusterName,
	}
}

// GenerateSecrets generates Kubernetes secrets based on the specified authentication types for each target.
// The provided targetAuthType map represents a set of targets, where each key corresponds to a target that
// will receive signal data using a specific authentication type. This function returns a map with the same target
// keys, where the values are `client.ObjectKey` representing the Kubernetes secret created for each target.
func (k *secretsProvider) GenerateSecrets(targetAuthType map[string]AuthenticationTypes) (map[string]client.ObjectKey, error) {
	secretKeys := make(map[string]client.ObjectKey, len(targetAuthType))

	ctx := context.Background()
	for targetName, authType := range targetAuthType {
		switch authType {
		case Static:
			createdSecret, err := k.genStaticSecret(ctx, targetName)
			if err != nil {
				return nil, err
			}
			secretKeys[targetName] = client.ObjectKey{Name: createdSecret.Name, Namespace: createdSecret.Namespace}

		case Managed:
			createdSecret, err := k.genManagedSecret(ctx, targetName)
			if err != nil {
				return nil, err
			}
			secretKeys[targetName] = client.ObjectKey{Name: createdSecret.Name, Namespace: createdSecret.Namespace}

		case MTLS:
			createdSecret, err := k.genMTLSSecret(ctx, targetName)
			if err != nil {
				return nil, err
			}
			secretKeys[targetName] = client.ObjectKey{Name: createdSecret.Name, Namespace: createdSecret.Namespace}
		}
	}

	return secretKeys, nil
}

// genStaticSecret creates a Kubernetes secret for static authentication
// TODO (JoaoBraveCoding) In the future we will want to deprecate this
// authentication method as it's not ideal for multicluster authentication
func (k *secretsProvider) genStaticSecret(ctx context.Context, targetName string) (*corev1.Secret, error) {
	key := client.ObjectKey{Name: "static-authentication", Namespace: "open-cluster-management"}
	staticAuth := &corev1.Secret{}
	err := k.client.Get(ctx, key, staticAuth, &client.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get existing secret: %w", err)
	}

	// Create the new secret with the fetched data
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-auth", targetName),
			Namespace: k.clusterName,
		},
		Data: staticAuth.Data,
	}

	return secret, k.createOrUpdate(ctx, secret)
}

// genManagedSecret generates a Kubernetes secret for managed authentication
// such as workload identity federation.
// TODO (JoaoBraveCoding) Currently not implemented, this should only work on
// STS/WIF enabeld clusters
func (k *secretsProvider) genManagedSecret(ctx context.Context, targetName string) (*corev1.Secret, error) {
	// Set additional keys for managed secret
	data := map[string][]byte{
		"roleARN":          []byte("foo"),
		"webIdentityToken": []byte("foo"),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-auth", targetName),
			Namespace: k.clusterName,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	return secret, k.createOrUpdate(ctx, secret)
}

// genMTLSSecret generates a Kubernetes secret for mTLS authentication. This is
// done using Cert-Manager CR.
func (k *secretsProvider) genMTLSSecret(ctx context.Context, targetName string) (client.ObjectKey, error) {
	secretKey := client.ObjectKey{Name: fmt.Sprintf("%s-auth", targetName), Namespace: k.clusterName}
	certKey := client.ObjectKey{Name: fmt.Sprintf("%s-cert", targetName), Namespace: k.clusterName}
	certManagerCert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certKey.Name,
			Namespace: certKey.Namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: secretKey.Name,
			// TODO(JoaoBraveCoding) Add missing parts
		},
	}

	// Return the client.ObjectKey pointing to the secret created by Cert-Manager
	return secretKey, k.createOrUpdate(ctx, certManagerCert)
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
