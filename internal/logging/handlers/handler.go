package handlers

import (
	"context"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, platform, userWorkloads addon.LogsOptions) (manifests.Options, error) {
	opts := manifests.Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
	}

	if platform.SubscriptionChannel != "" {
		opts.SubscriptionChannel = platform.SubscriptionChannel
	} else {
		opts.SubscriptionChannel = userWorkloads.SubscriptionChannel
	}

	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addon.ClusterLogForwardersResource)
	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(ctx, key, clf, &client.GetOptions{}); err != nil {
		return opts, err
	}
	opts.ClusterLogForwarder = clf

	targetSecretName := make(map[addon.Endpoint]string)
	for _, output := range clf.Spec.Outputs {
		targetSecretName[addon.Endpoint(output.Name)] = output.Secret.Name
	}

	targetSecrets, err := addon.GetSecrets(ctx, k8s, clf.Namespace, mcAddon.Namespace, targetSecretName)
	if err != nil {
		return opts, err
	}
	opts.Secrets = targetSecrets

	return opts, nil
}
