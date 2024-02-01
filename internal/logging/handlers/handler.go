package handlers

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterLogForwarderResource = "clusterlogforwarders"
)

func BuildOptions(k8s client.Client, log logr.Logger, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
	resources := manifests.Options{
		AddOnDeploymentConfig: adoc,
	}

	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, clusterLogForwarderResource)
	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(context.Background(), key, clf, &client.GetOptions{}); err != nil {
		return resources, err
	}
	resources.ClusterLogForwarder = clf

	authCM := &corev1.ConfigMap{}
	caCM := &corev1.ConfigMap{}
	for _, config := range mcAddon.Spec.Configs {
		switch config.ConfigGroupResource.Resource {
		case addon.ConfigMapResource:
			cm := &corev1.ConfigMap{}
			key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}
			if err := k8s.Get(context.Background(), key, cm, &client.GetOptions{}); err != nil {
				return resources, err
			}

			// Only care about cm's that configure logging
			if signal, ok := cm.Labels[addon.SignalLabelKey]; !ok || signal != addon.Logging.String() {
				continue
			}

			// If a cm has the ca annotation then it's the configmap containing the ca
			if _, ok := cm.Annotations[manifests.AnnotationCAToInject]; ok {
				caCM = cm
				continue
			}

			// If a cm doesn't have a target label then it's configuring authentication
			if _, ok := cm.Annotations[manifests.AnnotationTargetOutputName]; !ok {
				authCM = cm
				continue
			}

			resources.ConfigMaps = append(resources.ConfigMaps, *cm)
		}
	}

	ctx := context.Background()
	authConfig := manifests.AuthDefaultConfig
	authConfig.MTLSConfig.CommonName = mcAddon.Namespace
	if len(caCM.Data) > 0 {
		if ca, ok := caCM.Data["service-ca.crt"]; ok {
			authConfig.MTLSConfig.CAToInject = ca
		} else {
			return resources, kverrors.New("missing ca bundle in configmap", "key", "service-ca.crt")
		}
	}

	secretsProvider, err := authentication.NewSecretsProvider(k8s, log, mcAddon.Namespace, addon.Logging, authConfig)
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

	return resources, nil
}
