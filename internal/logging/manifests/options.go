package manifests

import (
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

type Options struct {
	Secrets               map[addon.Endpoint]corev1.Secret
	ClusterLogForwarder   *loggingv1.ClusterLogForwarder
	AddOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
}
