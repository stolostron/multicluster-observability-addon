package mcoagateway

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const GatewayRouteName = "mcoa-observatorium-api"

// GetGatewayRouteHost fetches the mcoa-gateway Route and returns its assigned
// host.
func GetGatewayRouteHost(ctx context.Context, k8s client.Client) (string, error) {
	route := &routev1.Route{}
	key := client.ObjectKey{Name: GatewayRouteName, Namespace: addoncfg.InstallNamespace}
	if err := k8s.Get(ctx, key, route); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get MCOA gateway route %s/%s: %w", key.Namespace, key.Name, err)
	}
	return route.Spec.Host, nil
}
