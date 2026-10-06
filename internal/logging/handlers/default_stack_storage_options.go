package handlers

import (
	"context"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/mcoagateway"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// buildDefaultStackStorageOptions loads LokiStack helm values only when this
// cluster's ManagedClusterAddOn references a LokiStack (the hub today; another
// cluster later). Spokes without that ref skip storage.
func buildDefaultStackStorageOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1beta1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	if len(common.GetObjectKeys(mcAddon.Status.ConfigReferences, lokiv1.GroupVersion.Group, addoncfg.LokiStacksResource)) == 0 {
		return nil
	}

	ls, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, lokiv1.GroupVersion.Group, addoncfg.LokiStacksResource, &lokiv1.LokiStack{})
	if err != nil {
		if common.IsAbsentOwnedConfig(err) {
			return nil
		}
		return err
	}

	objStorageSecret, err := common.GetSecret(ctx, k8s, ls.Namespace, mcAddon.Namespace, ls.Spec.Storage.Secret.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	lokiClientCASecret, err := common.GetSecret(ctx, k8s, addoncfg.InstallNamespace, mcAddon.Namespace, mcoagateway.DefaultLokiClientMTLSSecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	opts.DefaultStack.Storage.LokiStack = ls
	opts.DefaultStack.Storage.ObjStorageSecret = *objStorageSecret
	opts.DefaultStack.Storage.LokiClientCASecret = *lokiClientCASecret

	return nil
}
