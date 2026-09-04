package handlers

import (
	"context"
	"fmt"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func buildDefaultStackCollectionOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1beta1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	clf, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, loggingv1.GroupVersion.Group, addoncfg.ClusterLogForwardersResource, &loggingv1.ClusterLogForwarder{})
	if err != nil {
		return err
	}
	opts.DefaultStack.Collection.ClusterLogForwarder = clf

	mTLSSecret, err := common.GetSecret(ctx, k8s, clf.Namespace, mcAddon.Namespace, manifests.DefaultCollectionMTLSSecretName)
	if err != nil {
		return err
	}
	opts.DefaultStack.Collection.Secrets = []corev1.Secret{*mTLSSecret}

	opts.DefaultStack.LokiURL = fmt.Sprintf("https://mcoa-observability-observatorium-api.%s.svc:8080/api/logs/v1/%s/otlp/v1/logs", addoncfg.InstallNamespace, mcAddon.Namespace)

	return nil
}
