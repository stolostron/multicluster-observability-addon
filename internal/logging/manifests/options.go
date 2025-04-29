package manifests

import (
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Unmanaged                  Unmanaged
	DefaultStack               DefaultStack
	IsHub                      bool
	HubHostname                string
	Platform                   addon.LogsOptions
	UserWorkloads              addon.LogsOptions
	SubscriptionChannel        string
	ClusterLoggingSubscription *operatorv1alpha1.Subscription
}

type Unmanaged struct {
	Collection Collection
}

type DefaultStack struct {
	LokiURL    string
	Collection Collection
	Storage    Storage
}

type Collection struct {
	ConfigMaps          []corev1.ConfigMap
	Secrets             []corev1.Secret
	ClusterLogForwarder *loggingv1.ClusterLogForwarder
}

type Storage struct {
	Tenants          []string
	ObjStorageSecret corev1.Secret
	MTLSSecret       corev1.Secret
	LokiStack        *lokiv1.LokiStack
}

// UnmanagedCollectionEnabled returns true if the unmanaged collection is enabled.
func (opts Options) UnmanagedCollectionEnabled() bool {
	return opts.Platform.CollectionEnabled || opts.UserWorkloads.CollectionEnabled
}

func (opts Options) DefaultStackEnabled() bool {
	return opts.Platform.DefaultStack
}
