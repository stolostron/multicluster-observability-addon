package tracing

import (
	"context"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func buildOtelColSpec(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) (*otelv1alpha1.OpenTelemetryCollectorSpec, error) {
	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, otelv1alpha1.GroupVersion.Group, otelColResource)
	otelCol := &otelv1alpha1.OpenTelemetryCollector{}
	if err := k8s.Get(context.Background(), key, otelCol, &client.GetOptions{}); err != nil {
		return nil, err
	}

	return &otelCol.Spec, nil
}
