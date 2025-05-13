package addon

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InstallCOO(ctx context.Context, k8s client.Client, logger logr.Logger, isHub bool, opts Options) (bool, error) {
	if !cooDependantEnabled(opts) {
		return false, nil
	}

	// Currently, the InstallCOO option is only relevant for hub clusters
	// since we don't have k8s clients for the spokes
	if !isHub {
		return true, nil
	}

	cooSub := &operatorv1alpha1.Subscription{}
	key := client.ObjectKey{Name: cooSubscriptionName, Namespace: cooSubscriptionNamespace}
	if err := k8s.Get(ctx, key, cooSub, &client.GetOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return true, fmt.Errorf("failed to get cluster observability operator subscription: %w", err)
	}

	// Missing subscription means the operator is not installed
	if cooSub.Name == "" {
		return true, nil
	}

	// Wrong subscription channel means the operator is an error
	if cooSub.Spec.Channel != cooSubscriptionChannel {
		return false, errInvalidSubscriptionChannel
	}

	// If the subscription has our release label, install the operator
	if value, exists := cooSub.Labels["release"]; exists && value == "multicluster-observability-addon" {
		return true, nil
	}

	return false, nil
}

func cooDependantEnabled(opts Options) bool {
	if opts.Platform.Enabled {
		if opts.Platform.AnalyticsOptions.IncidentDetection.Enabled {
			return true
		}
		if opts.Platform.Metrics.UI {
			return true
		}
	}
	return false
}
