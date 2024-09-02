package handlers

import (
	"context"
	"errors"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errMissingCLFRef = errors.New("missing ClusterLogForwarder reference on addon installation")

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

	keys := addon.GetObjectKeys(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, addon.ClusterLogForwardersResource)
	switch {
	case len(keys) == 0:
		return opts, errMissingCLFRef
	}
	clfs := []loggingv1.ClusterLogForwarder{}
	for _, key := range keys {
		clf := &loggingv1.ClusterLogForwarder{}
		if err := k8s.Get(ctx, key, clf, &client.GetOptions{}); err != nil {
			return opts, err
		}
		clfs = append(clfs, *clf)
	}
	opts.ClusterLogForwarders = clfs

	namespaceSecrets := map[string][]corev1.Secret{}
	namespaceSecret := map[client.ObjectKey]struct{}{}
	for _, clf := range clfs {
		// We will want secrets to land on the same namespace as the ClusterLogForwarder
		targetNamespace := "openshift-logging"
		if val, ok := clf.Annotations["mcoa/namespace"]; ok {
			targetNamespace = val
		}

		secretNames := []string{}
		for _, output := range clf.Spec.Outputs {
			// Don't add secrets that have already been added
			key := client.ObjectKey{Namespace: targetNamespace, Name: output.Secret.Name}
			if _, ok := namespaceSecret[key]; ok {
				continue
			}
			namespaceSecret[key] = struct{}{}
			secretNames = append(secretNames, output.Secret.Name)
		}

		secrets, err := addon.GetSecrets(ctx, k8s, clf.Namespace, mcAddon.Namespace, secretNames)
		if err != nil {
			return opts, err
		}
		namespaceSecrets[targetNamespace] = append(namespaceSecrets[targetNamespace], secrets...)
	}
	opts.Secrets = namespaceSecrets

	return opts, nil
}
