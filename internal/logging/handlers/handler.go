package handlers

import (
	"context"

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

func BuildOptions(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
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
	for _, config := range mcAddon.Status.ConfigReferences {
		switch config.ConfigGroupResource.Resource {
		case addon.ConfigMapResource:
			key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}
			if err := k8s.Get(context.Background(), key, authCM, &client.GetOptions{}); err != nil {
				return resources, err
			}

			if signal, ok := authCM.Labels[addon.SignalLabelKey]; !ok || signal != addon.Logging.String() {
				continue
			}
		}
	}

	secretsProvider, err := authentication.NewSecretsProvider(k8s, mcAddon.Namespace, addon.Logging, manifests.AuthDefaultConfig)
	if err != nil {
		return resources, err
	}

	ctx := context.Background()
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
