package addon

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SkipInstallCOO(ctx context.Context, k8s client.Client, logger logr.Logger, isHub bool) (bool, error) {
	// Currently, the skipInstallCOO option is only relevant for hub clusters
	// since we don't have k8s clients for the spokes
	if !isHub {
		return false, nil
	}

	cooSub := &operatorv1alpha1.Subscription{}
	key := client.ObjectKey{Name: COOSubscriptionName, Namespace: COOSubscriptionNamespace}
	if err := k8s.Get(ctx, key, cooSub, &client.GetOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to get cluster observability operator subscription: %w", err)
	}

	// Missing subscription means the operator is not installed
	if cooSub.Name == "" {
		return false, nil
	}

	// Wrong subscription channel means the operator is an error
	if cooSub.Spec.Channel != cooSubscriptionChannel {
		return false, errInvalidSubscriptionChannel
	}

	return true, nil
}
