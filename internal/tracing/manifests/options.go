package manifests

import (
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

type Options struct {
	ClusterName            string
	Secrets                map[addon.Endpoint]corev1.Secret
	ConfigMaps             []corev1.ConfigMap
	OpenTelemetryCollector *otelv1beta1.OpenTelemetryCollector
	AddOnDeploymentConfig  *addonapiv1alpha1.AddOnDeploymentConfig
	UserWorkloads          addon.TracesOptions
}
