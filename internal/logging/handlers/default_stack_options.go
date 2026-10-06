package handlers

import (
	"context"
	"fmt"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/mcoagateway"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const lokiOTLPWritePathSuffix = "/api/logs/v1/otlp/v1/logs"

func buildDefaultStackCollectionOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1beta1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	// The hub withholds the CLF placement config until LokiStack is requested.
	// Until that reference shows up, skip collection so the rest of the addon
	// (metrics included) can still render.
	if len(common.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource)) == 0 {
		return nil
	}

	clf, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource, &loggingv1.ClusterLogForwarder{})
	if err != nil {
		if common.IsAbsentOwnedConfig(err) {
			return nil
		}
		return err
	}
	opts.DefaultStack.Collection.ClusterLogForwarder = clf

	mTLSSecret, err := common.GetSecret(ctx, k8s, clf.Namespace, mcAddon.Namespace, mcoagateway.DefaultCollectionMTLSSecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			opts.DefaultStack.Collection.ClusterLogForwarder = nil
			return nil
		}
		return err
	}
	opts.DefaultStack.Collection.Secrets = []corev1.Secret{*mTLSSecret}

	if opts.IsHub {
		opts.DefaultStack.LokiURL = fmt.Sprintf("https://mcoa-observability-observatorium-api.%s.svc:8080%s", addoncfg.InstallNamespace, lokiOTLPWritePathSuffix)
		return nil
	}

	host, err := mcoagateway.GetGatewayRouteHost(ctx, k8s)
	if err != nil || host == "" {
		opts.DefaultStack.Collection.ClusterLogForwarder = nil
		return nil
	}
	if err := mcoagateway.ServerCertIncludesHost(ctx, k8s, host); err != nil {
		opts.DefaultStack.Collection.ClusterLogForwarder = nil
		return nil
	}
	opts.DefaultStack.LokiURL = "https://" + host + lokiOTLPWritePathSuffix

	return nil
}
