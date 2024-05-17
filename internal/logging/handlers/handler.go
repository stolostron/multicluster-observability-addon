package handlers

import (
	"context"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
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

	targetSecret := make(map[authentication.Target]authentication.SecretKey)
	for _, output := range clf.Spec.Outputs {
		targetSecret[authentication.Target(output.Name)] = authentication.SecretKey{
			Name: output.Secret.Name,
		}
	}

	ctx := context.Background()
	authConfig := manifests.AuthDefaultConfig
	authConfig.TargetSecret = targetSecret
	authConfig.DefaultNamespace = clf.Namespace
	secretsProvider, err := authentication.NewSecretsProvider(k8s, mcAddon.Namespace, addon.Logging, authConfig)
	if err != nil {
		return resources, err
	}

	authMap, err := authentication.BuildAuthenticationFromAnnotations(clf.Annotations)
	if err != nil {
		return resources, err
	}

	targetsSecret, err := secretsProvider.GenerateSecrets(ctx, authMap)
	if err != nil {
		return resources, err
	}

	resources.Secrets, err = secretsProvider.FetchSecrets(ctx, targetsSecret)
	if err != nil {
		return resources, err
	}

	return resources, nil
}
