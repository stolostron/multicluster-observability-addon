package logging

import (
	"context"
	"encoding/json"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationTargetOutputName = "logging.openshift.io/target-output-name"
)

type LoggingValues struct {
	Enabled                    bool   `json:"enabled"`
	CLFSpec                    string `json:"clfSpec"`
	LoggingSubscriptionChannel string `json:"loggingSubscriptionChannel"`
}

func GetValuesFunc(k8s client.Client, cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (LoggingValues, error) {
	values := LoggingValues{
		Enabled: true,
	}

	channel, err := getLoggingSubscriptionChannel(adoc)
	if err != nil {
		return values, err
	}
	values.LoggingSubscriptionChannel = channel

	clfSpec, err := getClusterLogForwarderSpec(k8s, cluster, addon)
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

func getLoggingSubscriptionChannel(adoc *addonapiv1alpha1.AddOnDeploymentConfig) (string, error) {
	if adoc == nil || len(adoc.Spec.CustomizedVariables) == 0 {
		return defaultLoggingVersion, nil
	}

	for _, keyvalue := range adoc.Spec.CustomizedVariables {
		if keyvalue.Name == "loggingSubscriptionChannel" {
			return keyvalue.Value, nil
		}
	}
	return defaultLoggingVersion, nil
}

func getClusterLogForwarderSpec(k8s client.Client, cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) (*loggingv1.ClusterLogForwarderSpec, error) {
	var key client.ObjectKey
	for _, config := range addon.Status.ConfigReferences {
		if config.ConfigGroupResource.Group != loggingv1.GroupVersion.Group {
			continue
		}
		if config.ConfigGroupResource.Resource != "clusterlogforwarders" {
			continue
		}

		key.Name = config.Name
		key.Namespace = config.Namespace
	}

	clf := &loggingv1.ClusterLogForwarder{}
	if err := k8s.Get(context.TODO(), key, clf, &client.GetOptions{}); err != nil {
		return nil, err
	}

	for _, config := range addon.Status.ConfigReferences {
		if config.ConfigGroupResource.Group != "" {
			continue
		}

		switch config.ConfigGroupResource.Resource {
		case "configmaps":
			if err := getManagedClusterConfigMaps(k8s, &clf.Spec, config, cluster.Namespace); err != nil {
				return nil, err
			}
		case "secrets":
			if err := getManagedClusterSecrets(k8s, &clf.Spec, config, cluster.Namespace); err != nil {
				return nil, err
			}
		}
	}

	return &clf.Spec, nil
}

func getManagedClusterConfigMaps(k8s client.Client, spec *loggingv1.ClusterLogForwarderSpec, config addonapiv1alpha1.ConfigReference, clusterNs string) error {
	key := client.ObjectKey{Name: config.Name, Namespace: clusterNs}
	cm := &corev1.ConfigMap{}
	if err := k8s.Get(context.TODO(), key, cm, &client.GetOptions{}); err != nil {
		if errors.IsNotFound(err); err != nil {
			return nil
		}
		return err
	}

	clfOutputName, ok := cm.Annotations[AnnotationTargetOutputName]
	if !ok {
		return nil
	}

	outputURL := cm.Data["url"]

	for k, output := range spec.Outputs {
		if output.Name == clfOutputName {
			spec.Outputs[k].URL = outputURL
		}
	}

	return nil
}

func getManagedClusterSecrets(k8s client.Client, spec *loggingv1.ClusterLogForwarderSpec, config addonapiv1alpha1.ConfigReference, clusterNs string) error {
	key := client.ObjectKey{Name: config.Name, Namespace: clusterNs}
	secret := &corev1.Secret{}
	if err := k8s.Get(context.TODO(), key, secret, &client.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	clfOutputName, ok := secret.Annotations[AnnotationTargetOutputName]
	if !ok {
		return nil
	}

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
