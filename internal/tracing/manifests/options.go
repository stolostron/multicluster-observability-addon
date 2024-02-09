package manifests

import (
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

type Options struct {
	Secrets                []corev1.Secret
	ConfigMaps             []corev1.ConfigMap
	OpenTelemetryCollector *otelv1alpha1.OpenTelemetryCollector
	AddOnDeploymentConfig  *addonapiv1alpha1.AddOnDeploymentConfig
}
