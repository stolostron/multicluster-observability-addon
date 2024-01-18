package logging

import (
	"context"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func buildSubscriptionChannel(adoc *addonapiv1alpha1.AddOnDeploymentConfig) string {
	if adoc == nil || len(adoc.Spec.CustomizedVariables) == 0 {
		return defaultLoggingVersion
	}

	for _, keyvalue := range adoc.Spec.CustomizedVariables {
		if keyvalue.Name == subscriptionChannelValueKey {
			return keyvalue.Value
		}
	}
	return defaultLoggingVersion
}

func buildClusterLogForwarderSpec(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) (*loggingv1.ClusterLogForwarderSpec, error) {
	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, loggingv1.GroupVersion.Group, clusterLogForwarderResource)
	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(context.Background(), key, clf, &client.GetOptions{}); err != nil {
		return nil, err
	}

	for _, config := range mcAddon.Status.ConfigReferences {
		if config.ConfigGroupResource.Group != "" {
			continue
		}

		switch config.ConfigGroupResource.Resource {
		case addon.ConfigMapResource:
			if err := templateWithConfigMap(k8s, &clf.Spec, config); err != nil {
				return nil, err
			}
		case addon.SecretResource:
			if err := templateWithSecret(k8s, &clf.Spec, config); err != nil {
				return nil, err
			}
		}
	}

	return &clf.Spec, nil
}

func templateWithConfigMap(k8s client.Client, spec *loggingv1.ClusterLogForwarderSpec, config addonapiv1alpha1.ConfigReference) error {
	key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}
	cm := &corev1.ConfigMap{}
	if err := k8s.Get(context.Background(), key, cm, &client.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if signal, ok := cm.Labels[addon.SignalLabelKey]; !ok || signal != addon.SignalLogging {
		return nil
	}

	clfOutputName, ok := cm.Annotations[annotationTargetOutputName]
	if !ok {
		return nil
	}
	// TODO(JoaoBraveCoding) Validate that clfOutputName actually exists

	outputURL := cm.Data["url"]
	// TODO(JoaoBraveCoding) Validate that is a valid URL

	for k, output := range spec.Outputs {
		if output.Name == clfOutputName {
			spec.Outputs[k].URL = outputURL
		}
	}

	return nil
}

func templateWithSecret(k8s client.Client, spec *loggingv1.ClusterLogForwarderSpec, config addonapiv1alpha1.ConfigReference) error {
	key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}
	secret := &corev1.Secret{}
	if err := k8s.Get(context.Background(), key, secret, &client.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if signal, ok := secret.Labels[addon.SignalLabelKey]; !ok || signal != addon.SignalLogging {
		return nil
	}

	clfOutputName, ok := secret.Annotations[annotationTargetOutputName]
	if !ok {
		return nil
	}
	// TODO(JoaoBraveCoding) Validate that clfOutputName actually exists
	// TODO(JoaoBraveCoding) Validate secret

	for k, output := range spec.Outputs {
		if output.Name == clfOutputName {
			output.Secret = &loggingv1.OutputSecretSpec{
				Name: secret.Name,
			}
			spec.Outputs[k] = output
		}
	}

	return nil
}
