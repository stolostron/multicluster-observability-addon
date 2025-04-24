package manifests

import (
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	ConfigMaps                 []corev1.ConfigMap
	Secrets                    []corev1.Secret
	ClusterLogForwarder        *loggingv1.ClusterLogForwarder
	Platform                   addon.LogsOptions
	UserWorkloads              addon.LogsOptions
	SubscriptionChannel        string
	ClusterLoggingSubscription *operatorv1alpha1.Subscription
}
