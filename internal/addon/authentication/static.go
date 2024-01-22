package authentication

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StaticAuthenticationConfig struct {
	existingSecret client.ObjectKey
}

// createStaticSecret creates a Kubernetes secret for static authentication
// TODO (JoaoBraveCoding) In the future we will want to deprecate this
// authentication method as it's not ideal for multicluster authentication
func (k *secretsProvider) createStaticSecret(ctx context.Context, key client.ObjectKey) error {
	staticAuth := &corev1.Secret{}
	err := k.client.Get(ctx, k.StaticAuthConfig.existingSecret, staticAuth, &client.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing secret: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: staticAuth.Data, // Signal specific
	}

	return k.createOrUpdate(ctx, secret)
}
