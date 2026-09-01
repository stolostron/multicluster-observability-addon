package thanos

import (
	"context"
	"maps"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var mcoaLabels = map[string]string{
	"app.kubernetes.io/part-of":    "multicluster-observability-addon",
	"app.kubernetes.io/managed-by": "multicluster-observability-addon-manager",
}

type ObjectBuilder struct {
	Client client.Client
	Logger logr.Logger
}

func (b *ObjectBuilder) Build(ctx context.Context, cluster *clusterv1.ManagedCluster, opts addon.Options) ([]runtime.Object, error) {
	if !common.IsHubCluster(cluster) {
		return nil, nil
	}

	if !opts.ThanosOperatorEnabled {
		return nil, nil
	}

	var storeImage string
	if b.Client != nil {
		images, err := config.GetImageOverrides(ctx, b.Client, opts.Registries, b.Logger)
		if err != nil {
			b.Logger.Error(err, "failed to get image overrides, thanos store will use operator default image")
		} else {
			storeImage = images.ThanosStore
		}
	}

	store := b.buildStore(opts, storeImage)
	receive := b.buildReceive(opts)

	return []runtime.Object{store, receive}, nil
}

func (b *ObjectBuilder) buildStore(opts addon.Options, storeImage string) *thanosv1alpha1.ThanosStore {
	shards := int32(config.DefaultStoreShards)

	store := &thanosv1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: thanosv1alpha1.GroupVersion.String(),
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa",
			Namespace: config.HubInstallNamespace,
			Labels:    storeLabels(),
		},
		Spec: thanosv1alpha1.ThanosStoreSpec{
			Replicas: 1,
			ObjectStorageConfig: thanosv1alpha1.ObjectStorageConfig{
				LocalObjectReference: corev1.LocalObjectReference{Name: config.ObjectStorageSecretName},
				Key:                  config.ObjectStorageSecretKey,
			},
			StorageConfiguration: thanosv1alpha1.StorageConfiguration{
				Size: thanosv1alpha1.StorageSize(config.DefaultStoreStorageSize),
			},
			ShardingStrategy: thanosv1alpha1.ShardingStrategy{
				Type:   thanosv1alpha1.Block,
				Shards: shards,
			},
		},
	}

	if storeImage != "" {
		store.Spec.Image = &storeImage
	}

	ApplyCommonThanosFields(&store.Spec.CommonFields, opts, config.ThanosStoreContainerID)

	return store
}

func (b *ObjectBuilder) buildReceive(opts addon.Options) *thanosv1alpha1.ThanosReceive {
	retention := thanosv1alpha1.Duration(config.DefaultReceiveRetention)

	receive := &thanosv1alpha1.ThanosReceive{
		TypeMeta: metav1.TypeMeta{
			APIVersion: thanosv1alpha1.GroupVersion.String(),
			Kind:       "ThanosReceive",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa",
			Namespace: config.HubInstallNamespace,
			Labels:    receiveLabels(),
		},
		Spec: thanosv1alpha1.ThanosReceiveSpec{
			Router: thanosv1alpha1.RouterSpec{
				Replicas:          config.DefaultReceiveRouterReplicas,
				ReplicationFactor: config.DefaultReceiveRouterReplication,
				ExternalLabels:    thanosv1alpha1.ExternalLabels{"receive": "true"},
			},
			Ingester: thanosv1alpha1.IngesterSpec{
				DefaultObjectStorageConfig: thanosv1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{Name: config.ObjectStorageSecretName},
					Key:                  config.ObjectStorageSecretKey,
				},
				Hashrings: []thanosv1alpha1.IngesterHashringSpec{
					{
						Name:     config.DefaultReceiveHashringName,
						Replicas: config.DefaultReceiveIngesterReplicas,
						TSDBConfig: thanosv1alpha1.TSDBConfig{
							Retention: retention,
						},
						StorageConfiguration: thanosv1alpha1.StorageConfiguration{
							Size: thanosv1alpha1.StorageSize(config.DefaultReceiveIngesterStorageSize),
						},
					},
				},
			},
		},
	}

	ApplyCommonThanosFields(&receive.Spec.Router.CommonFields, opts, config.ThanosReceiveRouterContainerID)
	ApplyCommonThanosFields(&receive.Spec.Ingester.Hashrings[0].CommonFields, opts, config.ThanosReceiveIngesterContainerID)

	return receive
}

func receiveLabels() map[string]string {
	labels := make(map[string]string, len(mcoaLabels)+2)
	maps.Copy(labels, mcoaLabels)
	labels["app.kubernetes.io/component"] = "receive"
	labels["app.kubernetes.io/name"] = config.ThanosOperatorAppName
	return labels
}

func storeLabels() map[string]string {
	labels := make(map[string]string, len(mcoaLabels)+2)
	maps.Copy(labels, mcoaLabels)
	labels["app.kubernetes.io/component"] = "store"
	labels["app.kubernetes.io/name"] = config.ThanosOperatorAppName
	return labels
}

// ApplyCommonThanosFields applies node selector, tolerations, resource requirements,
// and security context to Thanos component specs. When ADC does not provide overrides,
// sensible defaults matching the existing MCO Thanos deployment are applied.
func ApplyCommonThanosFields(fields *thanosv1alpha1.CommonFields, opts addon.Options, containerID string) {
	if len(opts.NodeSelector) > 0 {
		fields.NodeSelector = opts.NodeSelector
	} else {
		fields.NodeSelector = map[string]string{"kubernetes.io/os": "linux"}
	}

	if len(opts.Tolerations) > 0 {
		fields.Tolerations = opts.Tolerations
	}

	fields.ResourceRequirements = defaultResources(containerID)
	for _, resReq := range opts.ResourceReqs {
		if resReq.ContainerID == containerID {
			fields.ResourceRequirements = &resReq.Resources
		}
	}

	runAsNonRoot := true
	fields.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
}

func defaultResources(containerID string) *corev1.ResourceRequirements {
	switch containerID {
	case config.ThanosReceiveRouterContainerID:
		return &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(config.DefaultReceiveRouterCPURequest),
				corev1.ResourceMemory: resource.MustParse(config.DefaultReceiveRouterMemRequest),
			},
		}
	case config.ThanosReceiveIngesterContainerID:
		return &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(config.DefaultReceiveIngesterCPURequest),
				corev1.ResourceMemory: resource.MustParse(config.DefaultReceiveIngesterMemRequest),
			},
		}
	default:
		return &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(config.DefaultStoreCPURequest),
				corev1.ResourceMemory: resource.MustParse(config.DefaultStoreMemRequest),
			},
		}
	}
}
