package thanos

import (
	"fmt"
	"maps"

	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	memcachedPort         = 11211
	memcachedExporterPort = 9150
)

func (b *ObjectBuilder) buildMemcachedObjects(images config.ImageOverrides) []runtime.Object {
	memcachedImage := config.DefaultMemcachedImage
	if images.Memcached != "" {
		memcachedImage = images.Memcached
	}
	exporterImage := config.DefaultMemcachedExporterImage
	if images.MemcachedExporter != "" {
		exporterImage = images.MemcachedExporter
	}

	storeDeployment := buildMemcachedDeployment(config.MemcachedStoreName, memcachedStoreLabels(), memcachedImage, exporterImage)
	storeService := buildMemcachedService(config.MemcachedStoreName, memcachedStoreLabels())
	storeCacheSecret := buildCacheConfigSecret(
		config.StoreCacheConfigSecretName,
		config.StoreCacheConfigSecretKey,
		config.MemcachedStoreName,
	)

	qfeDeployment := buildMemcachedDeployment(config.MemcachedQueryFrontendName, memcachedQueryFrontendLabels(), memcachedImage, exporterImage)
	qfeService := buildMemcachedService(config.MemcachedQueryFrontendName, memcachedQueryFrontendLabels())
	qfeCacheSecret := buildCacheConfigSecret(
		config.QueryFECacheConfigSecretName,
		config.QueryFECacheConfigSecretKey,
		config.MemcachedQueryFrontendName,
	)

	return []runtime.Object{
		storeDeployment, storeService, storeCacheSecret,
		qfeDeployment, qfeService, qfeCacheSecret,
	}
}

func buildMemcachedDeployment(name string, labels map[string]string, memcachedImage, exporterImage string) *appsv1.Deployment {
	replicas := config.DefaultMemcachedReplicas
	memoryLimitArg := fmt.Sprintf("-m %d", config.DefaultMemcachedMemoryLimitMB)
	connLimitArg := fmt.Sprintf("-c %d", config.DefaultMemcachedConnectionLimit)
	maxItemArg := fmt.Sprintf("-I %s", config.DefaultMemcachedMaxItemSize)

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.HubInstallNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{"kubernetes.io/os": "linux"},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
					},
					Containers: []corev1.Container{
						{
							Name:  "memcached",
							Image: memcachedImage,
							Args:  []string{memoryLimitArg, maxItemArg, connLimitArg, "-v"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "client",
									ContainerPort: memcachedPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(config.DefaultMemcachedCPURequest),
									corev1.ResourceMemory: resource.MustParse(config.DefaultMemcachedMemRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(config.DefaultMemcachedMemLimit),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								ReadOnlyRootFilesystem: boolPtr(true),
							},
						},
						{
							Name:  "exporter",
							Image: exporterImage,
							Args: []string{
								fmt.Sprintf("--memcached.address=localhost:%d", memcachedPort),
								fmt.Sprintf("--web.listen-address=0.0.0.0:%d", memcachedExporterPort),
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "metrics",
									ContainerPort: memcachedExporterPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(config.DefaultMemcachedExporterCPURequest),
									corev1.ResourceMemory: resource.MustParse(config.DefaultMemcachedExporterMemRequest),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								ReadOnlyRootFilesystem: boolPtr(true),
							},
						},
					},
				},
			},
		},
	}
}

func buildMemcachedService(name string, labels map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.HubInstallNamespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector:  labels,
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:       "client",
					Port:       memcachedPort,
					TargetPort: intstr.FromInt(memcachedPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       memcachedExporterPort,
					TargetPort: intstr.FromInt(memcachedExporterPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

func buildCacheConfigSecret(secretName, secretKey, serviceName string) *corev1.Secret {
	cacheConfigYAML := fmt.Sprintf(`type: MEMCACHED
config:
  addresses:
  - dnssrv+_client._tcp.%s.%s.svc
  timeout: 500ms
  max_idle_connections: 100
  max_async_concurrency: 20
  max_async_buffer_size: 10000
  max_get_multi_concurrency: 100
  max_item_size: %s
  dns_provider_update_interval: 10s
`, serviceName, config.HubInstallNamespace, config.DefaultMemcachedMaxItemSizeCache)

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: config.HubInstallNamespace,
			Labels:    mcoaLabels,
		},
		StringData: map[string]string{
			secretKey: cacheConfigYAML,
		},
	}
}

func memcachedStoreLabels() map[string]string {
	labels := make(map[string]string, len(mcoaLabels)+2)
	maps.Copy(labels, mcoaLabels)
	labels["app.kubernetes.io/component"] = "store-index-cache"
	labels["app.kubernetes.io/name"] = "memcached"
	return labels
}

func memcachedQueryFrontendLabels() map[string]string {
	labels := make(map[string]string, len(mcoaLabels)+2)
	maps.Copy(labels, mcoaLabels)
	labels["app.kubernetes.io/component"] = "query-frontend-cache"
	labels["app.kubernetes.io/name"] = "memcached"
	return labels
}

func boolPtr(b bool) *bool {
	return &b
}
