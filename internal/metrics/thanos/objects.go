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

	return []runtime.Object{store}, nil
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

	fields.ResourceRequirements = defaultStoreResources()
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

func defaultStoreResources() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(config.DefaultStoreCPURequest),
			corev1.ResourceMemory: resource.MustParse(config.DefaultStoreMemRequest),
		},
	}
}
