package handlers

import (
	"context"
	"fmt"
	"strings"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests/otelcol"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	opentelemetryCollectorResource = "opentelemetrycollectors"
)

var (
	errNoExportersFound       = fmt.Errorf("no exporters found")
	errNoMountPathFound       = fmt.Errorf("mountpath not found in any secret")
	errNoVolumeMountForSecret = fmt.Errorf("no volumemount found for secret")
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
	klog.Info("OpenTelemetry Collector template found")

	targetSecretName, err := buildExportersSecrets(otelCol)
	if err != nil {
		return resources, nil
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

func buildExportersSecrets(otelCol *otelv1alpha1.OpenTelemetryCollector) (map[authentication.Target]string, error) {
	exporterSecrets := map[authentication.Target]string{}

	exporters, err := getExporters(otelCol)
	if err != nil {
		return exporterSecrets, err
	}

	if len(exporters) == 0 {
		return exporterSecrets, errNoExportersFound
	}

	for _, vol := range otelCol.Spec.Volumes {
		// We only care about volumes created from secrets
		if vol.Secret != nil {
			vm, err := getVolumeMount(otelCol, vol.Secret.SecretName)
			if err != nil {
				return exporterSecrets, err
			}
			exporter, err := searchVolumeMountInExporter(vm, exporters)
			if err != nil {
				klog.Warning(err)
				continue
			}
			klog.Info("exporter ", exporter, " uses secret ", vol.Secret.SecretName)
			exporterSecrets[authentication.Target(exporter)] = vol.Secret.SecretName
		}
	}
	return exporterSecrets, nil
}

// getVolumeMount gets the VolumeMount associated to a secret.
func getVolumeMount(otelCol *otelv1alpha1.OpenTelemetryCollector, secretName string) (v1.VolumeMount, error) {
	for _, vm := range otelCol.Spec.VolumeMounts {
		if vm.Name == secretName {
			return vm, nil
		}
	}
	return v1.VolumeMount{}, errNoVolumeMountForSecret
}

// searchVolumeMountInExporter checks if the VolumeMount is used in any exporter
func searchVolumeMountInExporter(vm v1.VolumeMount, exporters map[string]interface{}) (string, error) {
	for name, eMap := range exporters {
		if eMap == nil {
			continue
		}

		t, ok := eMap.(map[string]interface{})["tls"]
		if !ok {
			continue
		}
		tls := t.(map[string]interface{})
		if strings.HasPrefix(tls["cert_file"].(string), vm.MountPath) ||
			strings.HasPrefix(tls["key_file"].(string), vm.MountPath) ||
			strings.HasPrefix(tls["ca_file"].(string), vm.MountPath) {
			return name, nil
		}
	}
	return "", errNoMountPathFound
}

// getExporters gets the exporters from the OpenTelemetryCollector
func getExporters(otelCol *otelv1alpha1.OpenTelemetryCollector) (map[string]interface{}, error) {
	var exporters map[string]interface{}

	cfg, err := otelcol.ConfigFromString(otelCol.Spec.Config)
	if err != nil {
		return exporters, err
	}

	exporters, err = otelcol.GetExporters(cfg)
	return exporters, err
}
