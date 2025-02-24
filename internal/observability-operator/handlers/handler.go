package handlers

import (
	"context"

	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/observability-operator/manifests"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, oboOptions addon.ObservabilityOperatorOptions) manifests.Options {
	return manifests.Options{
		Enabled: oboOptions.Enabled,
	}
}
