package handlers

import (
	"context"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	opentelemetryCollectorResource = "opentelemetrycollectors"
)

func BuildOptions(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
	resources := manifests.Options{
		AddOnDeploymentConfig: adoc,
	}

	klog.Info("Retrieving OpenTelemetry Collector template")
	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, otelv1alpha1.GroupVersion.Group, opentelemetryCollectorResource)
	otelCol := &otelv1alpha1.OpenTelemetryCollector{}
	if err := k8s.Get(context.Background(), key, otelCol, &client.GetOptions{}); err != nil {
		return resources, err
	}
	resources.OpenTelemetryCollector = otelCol
	klog.Info("OpenTelemetry Collector template found")

	return resources, nil
}
