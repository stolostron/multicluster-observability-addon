package manifests

import (
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Unmanaged           Unmanaged
	Managed             Managed
	IsHubCluster        bool
	Platform            addon.LogsOptions
	UserWorkloads       addon.LogsOptions
	SubscriptionChannel string
}

type Unmanaged struct {
	Collection Collection
}

type Managed struct {
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
}

func (opts Options) UnmanagedCollectionEnabled() bool {
	return opts.Platform.CollectionEnabled || opts.UserWorkloads.CollectionEnabled
}

func (opts Options) ManagedStackEnabled() bool {
	return opts.Platform.StorageEnabled
}
