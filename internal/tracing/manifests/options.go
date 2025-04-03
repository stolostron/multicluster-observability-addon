package manifests

import (
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

type Options struct {
	ClusterName            string
	Secrets                []corev1.Secret
	OpenTelemetryCollector *otelv1beta1.OpenTelemetryCollector
	Instrumentation        *otelv1alpha1.Instrumentation
	AddOnDeploymentConfig  *addonapiv1alpha1.AddOnDeploymentConfig
	UserWorkloads          addon.TracesOptions
}
