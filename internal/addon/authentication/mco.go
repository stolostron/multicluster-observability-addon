package authentication

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createMCOSecret creates a Kubernetes secret for authentication using the
// credentials provided by MCO
// TODO (JoaoBraveCoding) Not implemented
func (k *secretsProvider) createMCOSecret(ctx context.Context, key client.ObjectKey) error {
	return nil
}
