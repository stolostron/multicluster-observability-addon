package handlers

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	nfv1beta2 "github.com/netobserv/network-observability-operator/apis/flowcollector/v1beta2"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/netflows/manifests"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	flowCollectorResource = "flowcollectors"
)

func BuildOptions(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
	resources := manifests.Options{
		AddOnDeploymentConfig: adoc,
	}

	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, nfv1beta2.GroupVersion.Group, flowCollectorResource)
	fc := &nfv1beta2.FlowCollector{}
	if err := k8s.Get(context.Background(), key, fc, &client.GetOptions{}); err != nil {
		return resources, err
	}
	resources.FlowCollector = fc

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

			// Only care about cm's that configure netflow
			if signal, ok := cm.Labels[addon.SignalLabelKey]; !ok || signal != addon.Netflow.String() {
				continue
			}

			// If a cm has the ca annotation then it's the configmap containing the ca
			if _, ok := cm.Annotations[manifests.AnnotationCAToInject]; ok {
				caCM = cm
				continue
			}

			// If a cm doesn't have a target annotation then it's configuring authentication
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

	secretsProvider, err := authentication.NewSecretsProvider(k8s, mcAddon.Namespace, addon.Logging, authConfig)
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
