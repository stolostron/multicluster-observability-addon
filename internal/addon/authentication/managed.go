package authentication

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createManagedSecret generates a Kubernetes secret for managed authentication
// such as workload identity federation.
// TODO (JoaoBraveCoding) Currently not implemented, this should only work on
// STS/WIF enabeld clusters
func (k *secretsProvider) createManagedSecret(ctx context.Context, key client.ObjectKey) error {
	// Set additional keys for managed secret
	data := map[string][]byte{
		"roleARN":          []byte("foo"),
		"webIdentityToken": []byte("foo"),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	return k.createOrUpdate(ctx, secret)
}
