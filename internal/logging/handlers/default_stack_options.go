package handlers

import (
	"context"
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// findOwnedResource finds a resource from ManagedClusterAddOn config references that is owned by ClusterManagementAddOn
func buildDefaultStackOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts *manifests.Options) error {
	if !opts.DefaultStackEnabled() {
		return nil
	}

	// Get CLF from ManagedClusterAddOn
	clf, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, loggingv1.GroupVersion.Group, addon.ClusterLogForwardersResource, &loggingv1.ClusterLogForwarder{})
	if err != nil {
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
		ls, err := common.GetResourceWithOwnerRef(ctx, k8s, mcAddon, lokiv1.GroupVersion.Group, addon.LokiStacksResource, &lokiv1.LokiStack{})
		if err != nil {
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

		// Extract tenants from the LokiStack object's authentication specs
		tenants := []string{}
		if ls.Spec.Tenants != nil && len(ls.Spec.Tenants.Authentication) > 0 {
			for _, auth := range ls.Spec.Tenants.Authentication {
				tenants = append(tenants, auth.TenantID)
			}
		}

		opts.DefaultStack.Storage.Tenants = tenants

		return nil
	}

	return nil
}
