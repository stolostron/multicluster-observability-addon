package handlers

import (
	"context"
	"errors"
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/mcoagateway"
	corev1 "k8s.io/api/core/v1"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const lokiOTLPWritePathSuffix = "/api/logs/v1/otlp/v1/logs"

var errGatewayRouteNoHost = errors.New("MCOA gateway route has no host assigned")

func buildDefaultStackOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1beta1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	clf, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource, &loggingv1.ClusterLogForwarder{})
	if err != nil {
		return err
	}
	opts.DefaultStack.Collection.ClusterLogForwarder = clf

	mTLSSecret, err := common.GetSecret(ctx, k8s, clf.Namespace, mcAddon.Namespace, mcoagateway.DefaultCollectionMTLSSecretName)
	if err != nil {
		return err
	}
	opts.DefaultStack.Collection.Secrets = []corev1.Secret{*mTLSSecret}

	if opts.IsHub {
		opts.DefaultStack.LokiURL = fmt.Sprintf("https://mcoa-observability-observatorium-api.%s.svc:8080%s", addoncfg.InstallNamespace, lokiOTLPWritePathSuffix)
	} else {
		host, err := mcoagateway.GetGatewayRouteHost(ctx, k8s)
		if err != nil {
			return fmt.Errorf("failed to get MCOA gateway route for spoke logging endpoint: %w", err)
		}
		if host == "" {
			return fmt.Errorf("%w: %q", errGatewayRouteNoHost, mcoagateway.GatewayRouteName)
		}
		opts.DefaultStack.LokiURL = "https://" + host + lokiOTLPWritePathSuffix
	}

	if opts.IsHub {
		ls, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, lokiv1.GroupVersion.Group, addoncfg.LokiStacksResource, &lokiv1.LokiStack{})
		if err != nil {
			return err
		}

		opts.DefaultStack.Storage.LokiStack = ls

		objStorageSecret, err := common.GetSecret(ctx, k8s, ls.Namespace, mcAddon.Namespace, ls.Spec.Storage.Secret.Name)
		if err != nil {
			return err
		}
		opts.DefaultStack.Storage.ObjStorageSecret = *objStorageSecret

		mTLSSecret, err := common.GetSecret(ctx, k8s, addoncfg.InstallNamespace, mcAddon.Namespace, mcoagateway.DefaultStorageMTLSSecretName)
		if err != nil {
			return err
		}
		opts.DefaultStack.Storage.MTLSSecret = *mTLSSecret

		lokiClientCASecret, err := common.GetSecret(ctx, k8s, addoncfg.InstallNamespace, mcAddon.Namespace, mcoagateway.DefaultLokiClientMTLSSecretName)
		if err != nil {
			return err
		}
		opts.DefaultStack.Storage.LokiClientCASecret = *lokiClientCASecret

		return nil
	}

	return nil
}
