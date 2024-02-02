package manifests

import (
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

type Options struct {
	Secrets               []corev1.Secret
	ConfigMaps            []corev1.ConfigMap
	ClusterLogForwarder   *loggingv1.ClusterLogForwarder
	AddOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
}
