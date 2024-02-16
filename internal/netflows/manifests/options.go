package manifests

import (
	nfv1beta2 "github.com/netobserv/network-observability-operator/apis/flowcollector/v1beta2"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

type Options struct {
	Secrets               []corev1.Secret
	ConfigMaps            []corev1.ConfigMap
	FlowCollector         *nfv1beta2.FlowCollector
	AddOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
}
