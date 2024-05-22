package handlers

import (
	"context"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests/otelcol"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	opentelemetryCollectorResource = "opentelemetrycollectors"
)

func BuildOptions(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
	resources := manifests.Options{
		AddOnDeploymentConfig: adoc,
		ClusterName:           mcAddon.Namespace,
	}

	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, otelv1alpha1.GroupVersion.Group, opentelemetryCollectorResource)
	otelCol := &otelv1alpha1.OpenTelemetryCollector{}
	if err := k8s.Get(context.Background(), key, otelCol, &client.GetOptions{}); err != nil {
		return resources, err
	}
	resources.OpenTelemetryCollector = otelCol
	cfg, err := otelcol.ConfigFromString(otelCol.Spec.Config)
	if err != nil {
		return resources, err
	}
	exporters, err := otelcol.GetExporters(cfg)
	if err != nil {
		return resources, err
	}

	targetSecretName := make(map[authentication.Target]string)
	for exporterName := range exporters {
		// TODO @iblancas help!
		targetSecretName[authentication.Target(exporterName)] = "TODO"
	}

	ctx := context.Background()
	secretsProvider := authentication.NewSecretsProvider(k8s, otelCol.Namespace, mcAddon.Namespace)
	targetsSecret, err := secretsProvider.GenerateSecrets(ctx, otelCol.Annotations, targetSecretName)
	if err != nil {
		return resources, err
	}

	resources.Secrets, err = secretsProvider.FetchSecrets(ctx, targetsSecret)
	if err != nil {
		return resources, err
	}

	return resources, nil
}
