package handlers

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationCAToInject           = "tracing.mcoa.openshift.io/ca"
	opentelemetryCollectorResource = "opentelemetrycollectors"
)

func BuildOptions(k8s client.Client, log logr.Logger, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
	resources := manifests.Options{
		AddOnDeploymentConfig: adoc,
		ClusterName:           mcAddon.Namespace,
	}

	log.V(1).Info("Retrieving OpenTelemetry Collector template")
	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, otelv1alpha1.GroupVersion.Group, opentelemetryCollectorResource)
	otelCol := &otelv1alpha1.OpenTelemetryCollector{}
	if err := k8s.Get(context.Background(), key, otelCol, &client.GetOptions{}); err != nil {
		return resources, err
	}
	resources.OpenTelemetryCollector = otelCol
	log.V(1).Info("OpenTelemetry Collector template found")

	var authCM *corev1.ConfigMap = nil
	var caSecret *corev1.Secret = nil

	for _, config := range mcAddon.Spec.Configs {
		l := log.WithValues(
			"config_name", config.Name,
			"config_namespace", config.Namespace,
		)
		key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}
		switch config.ConfigGroupResource.Resource {
		case addon.ConfigMapResource:
			cm := &corev1.ConfigMap{}
			l.V(1).Info("processing cm")
			if err := k8s.Get(context.Background(), key, cm, &client.GetOptions{}); err != nil {
				return resources, err
			}

			// Only care about cm's that configure tracing
			if signal, ok := cm.Labels[addon.SignalLabelKey]; !ok || signal != addon.Tracing.String() {
				l.V(1).Info("skipped configmap")
				continue
			}

			// If a cm doesn't have a target annotation then it's configuring authentication
			if _, ok := cm.Annotations[manifests.AnnotationTargetOutputName]; !ok {
				if authCM != nil {
					l.Info("auth configmap already set, will overwrite")
				}
				log.V(1).Info("auth configmap set")
				authCM = cm
				continue
			}

			resources.ConfigMaps = append(resources.ConfigMaps, *cm)
		case addon.SecretResource:
			secret := &corev1.Secret{}
			l.V(1).Info("processing secret")
			if err := k8s.Get(context.Background(), key, secret, &client.GetOptions{}); err != nil {
				return resources, err
			}

			// Only care about cm's that configure tracing
			if signal, ok := secret.Labels[addon.SignalLabelKey]; !ok || signal != addon.Tracing.String() {
				l.V(1).Info("skipped secret")
				continue
			}

			// If the secret has the ca annotation then it's the secret containing the ca
			if _, ok := secret.Annotations[AnnotationCAToInject]; ok {
				caSecret = secret
				continue
			}
		}
	}

	ctx := context.Background()
	authConfig := manifests.AuthDefaultConfig
	authConfig.MTLSConfig.CommonName = mcAddon.Namespace
	if caSecret == nil {
		log.Info("no CA was found")
	} else if len(caSecret.Data) > 0 {
		if ca, ok := caSecret.Data["ca.crt"]; ok {
			authConfig.MTLSConfig.CAToInject = string(ca)
		} else {
			return resources, kverrors.New("missing ca bundle in secret", "key", "ca.crt")
		}
	}

	if authCM != nil {
		secretsProvider, err := authentication.NewSecretsProvider(k8s, log, mcAddon.Namespace, addon.Tracing, authConfig)
		if err != nil {
			return resources, err
		}

		targetsSecret, err := secretsProvider.GenerateSecrets(ctx, authentication.BuildAuthenticationMap(authCM.Data))
		if err != nil {
			return resources, err
		}

		resources.Secrets, err = secretsProvider.FetchSecrets(ctx, targetsSecret, manifests.AnnotationTargetOutputName)
		if err != nil {
			return resources, err
		}
	}

	return resources, nil
}
