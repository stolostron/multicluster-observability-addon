package manifests

import (
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Unmanaged           Unmanaged
	ManagedStack        ManagedStack
	IsHubCluster        bool
	HubHostname         string
	Platform            addon.LogsOptions
	UserWorkloads       addon.LogsOptions
	SubscriptionChannel string
}

type Unmanaged struct {
	Collection Collection
}

type ManagedStack struct {
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

// UnmanagedCollectionEnabled returns true if the unmanaged collection is enabled.
// Note we have disabled unmanaged collection for hub cluster on purpose due to
// have never been designed in the first version of MCOA. This can change but it
// should be done in its own PR.
func (opts Options) UnmanagedCollectionEnabled() bool {
	return (opts.Platform.CollectionEnabled || opts.UserWorkloads.CollectionEnabled) && !opts.IsHubCluster
}

func (opts Options) ManagedStackEnabled() bool {
	return opts.Platform.ManagedStack
}
