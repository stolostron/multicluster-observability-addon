package logging

import (
	"context"
	"encoding/json"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LoggingValues struct {
	Enabled                    bool          `json:"enabled"`
	CLFSpec                    string        `json:"clfSpec"`
	LoggingSubscriptionChannel string        `json:"loggingSubscriptionChannel"`
	Secrets                    []SecretValue `json:"secrets"`
}
type SecretValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type Resources struct {
	Secrets               []corev1.Secret
	ClusterLogForwarder   *loggingv1.ClusterLogForwarder
	AddOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
}

func GetValuesFunc(k8s client.Client, _ *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (LoggingValues, error) {
	values := LoggingValues{
		Enabled: true,
	}

	resources, err := fetchLoggingResources(k8s, mcAddon, adoc)
	if err != nil {
		return values, err
	}

	values.LoggingSubscriptionChannel = buildSubscriptionChannel(resources)

	secrets, err := buildSecrets(resources)
	if err != nil {
		return values, err
	}
	values.Secrets = secrets

	clfSpec, err := buildClusterLogForwarderSpec(resources)
	if err != nil {
		return values, err
	}

	b, err := json.Marshal(clfSpec)
	if err != nil {
		return values, err
	}
	values.CLFSpec = string(b)

	return values, nil
}

func fetchLoggingResources(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (Resources, error) {
	resources := Resources{
		AddOnDeploymentConfig: adoc,
	}

	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, clusterLogForwarderResource)
	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(context.Background(), key, clf, &client.GetOptions{}); err != nil {
		return resources, err
	}
	resources.ClusterLogForwarder = clf

	secrets := []corev1.Secret{}
	for _, config := range mcAddon.Status.ConfigReferences {
		switch config.ConfigGroupResource.Resource {
		case addon.SecretResource:
			key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}
			secret := &corev1.Secret{}
			if err := k8s.Get(context.Background(), key, secret, &client.GetOptions{}); err != nil {
				if errors.IsNotFound(err) {
					continue // Here we should throw an error as it's a missing secret
				}
				return resources, err
			}

			if signal, ok := secret.Labels[addon.SignalLabelKey]; !ok || signal != addon.SignalLogging {
				continue
			}

			secrets = append(secrets, *secret)
		}
	}
	resources.Secrets = secrets

	return resources, nil
}
