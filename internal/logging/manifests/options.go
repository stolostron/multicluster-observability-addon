package manifests

import (
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (opts Options) UnmanagedCollectionEnabled() bool {
	return opts.Platform.CollectionEnabled || opts.UserWorkloads.CollectionEnabled
}

func (opts Options) DefaultStackEnabled() bool {
	return opts.Platform.DefaultStack
}

func BuildDefaultStackOptions(platform, userWorkloads addon.LogsOptions, hubHostname string) Options {
	return Options{
		Platform:      platform,
		UserWorkloads: userWorkloads,
		IsHub:         true,
		HubHostname:   hubHostname,
		DefaultStack: DefaultStack{
			// Template value as CLF requires a LokiURL in its CEL expression.
			LokiURL: fmt.Sprintf("https://mcoa-observability-observatorium-api.%s.svc:8080/api/logs/v1/%s/otlp/v1/logs", addoncfg.InstallNamespace, "tenant"),
			Collection: Collection{
				Secrets: []corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      DefaultCollectionMTLSSecretName,
							Namespace: addoncfg.InstallNamespace,
						},
					},
				},
			},
			Storage: Storage{
				ObjStorageSecret: corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						// TODO(JoaoBraveCoding): Revisit this, I'm not sure if this makes sense as this should be something the user provides.
						Name:      DefaultStorageMTLSSecretName,
						Namespace: addoncfg.InstallNamespace,
					},
				},
				MTLSSecret: corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultStorageMTLSSecretName,
						Namespace: addoncfg.HubNamespace,
					},
				},
			},
		},
	}
}
