package handlers

import (
	"context"
	"fmt"
	"strings"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func buildDefaultStackOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	// Get CLF from ManagedClusterAddOn
	keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addon.ClusterLogForwardersResource)
	if len(keys) == 0 {
		return errMissingDefaultCLFRef
	}
	clfKey := client.ObjectKey{}
	// TODO(JoaoBraveCoding): This needs to be changed to use ownerReferences
	for _, key := range keys {
		if strings.HasPrefix(key.Name, addon.DefaultStackPrefix) {
			clfKey = key
			break
		}
	}
	if clfKey.Name == "" {
		return errMissingDefaultCLFRef
	}

	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(ctx, clfKey, clf, &client.GetOptions{}); err != nil {
		return err
	}
	opts.DefaultStack.Collection.ClusterLogForwarder = clf

	mTLSSecret, err := common.GetSecret(ctx, k8s, clf.Namespace, mcAddon.Namespace, manifests.DefaultCollectionMTLSSecretName)
	if err != nil {
		// Even for not found we probably just want to return and wait for the next
		// reconciliation loop to try again.
		return err
	}
	opts.DefaultStack.Collection.Secrets = []corev1.Secret{*mTLSSecret}

	// Get the cluster hostname
	opts.DefaultStack.LokiURL = fmt.Sprintf("https://mcoa-managed-instance-openshift-logging.apps.%s/api/logs/v1/%s/otlp/v1/logs", opts.HubHostname, mcAddon.Namespace)

	if opts.IsHub {
		// Get LS from ManagedClusterAddOn
		keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, lokiv1.GroupVersion.Group, addon.LokiStacksResource)
		if len(keys) == 0 {
			return errMissingDefaultLSRef
		}
		lsKey := client.ObjectKey{}
		// TODO(JoaoBraveCoding): This needs to be changed to use ownerReferences
		for _, key := range keys {
			if strings.HasPrefix(key.Name, addon.DefaultStackPrefix) {
				lsKey = key
				break
			}
		}
		if lsKey.Name == "" {
			return errMissingDefaultLSRef
		}

		ls := &lokiv1.LokiStack{}
		if err := k8s.Get(ctx, lsKey, ls, &client.GetOptions{}); err != nil {
			return err
		}
		opts.DefaultStack.Storage.LokiStack = ls

		// Get objstorage secret
		objStorageSecret, err := common.GetSecret(ctx, k8s, ls.Namespace, mcAddon.Namespace, ls.Spec.Storage.Secret.Name)
		if err != nil {
			// Even for not found we probably just want to return and wait for the next
			// reconciliation loop to try again.
			return err
		}
		opts.DefaultStack.Storage.ObjStorageSecret = *objStorageSecret

		// Get mTLS secret
		mTLSSecret, err := common.GetSecret(ctx, k8s, ls.Namespace, mcAddon.Namespace, manifests.DefaultStorageMTLSSecretName)
		if err != nil {
			// Even for not found we probably just want to return and wait for the next
			// reconciliation loop to try again.
			return err
		}
		opts.DefaultStack.Storage.MTLSSecret = *mTLSSecret

		// TODO(JoaoBraveCoding): This might be rather heavy in big clusters,
		// this might be a good place to lower memory consumption.
		mcaoList := &addonapiv1alpha1.ManagedClusterAddOnList{}
		if err := k8s.List(ctx, mcaoList, &client.ListOptions{}); err != nil {
			return err
		}

		tenants := make([]string, 0, len(mcaoList.Items))
		for _, tenant := range mcaoList.Items {
			// TODO(JoaoBraveCoding): This is not the best way to match tenants due
			// to the addon-framework supporting tenantcy, but it will do for now
			if tenant.Name == addon.Name && tenant.Namespace != mcAddon.Namespace {
				tenants = append(tenants, tenant.Namespace)
			}
		}
		opts.DefaultStack.Storage.Tenants = tenants

		return nil
	}

	return nil
}
