package config

const (
	// Thanos Operator
	ThanosOperatorAppName = "thanos-operator"
	ThanosAPIGroup        = "monitoring.thanos.io"
	ThanosCRName          = "mcoa"

	// Thanos CR resource names (plural)
	ThanosStoreResource   = "thanosstores"
	ThanosReceiveResource = "thanosreceives"
	ThanosQueryResource   = "thanosqueries"
	ThanosRulerResource   = "thanosrulers"
	ThanosCompactResource = "thanoscompacts"
	// TODO: replace with image from ACM image overrides ConfigMap once available.
	ThanosOperatorImage = "quay.io/thanos/thanos-operator:main-2026-04-09-a4dc024"

	// Thanos Store defaults
	DefaultStoreShards      = 3
	DefaultStoreStorageSize = "10Gi"
	DefaultStoreCPURequest  = "100m"
	DefaultStoreMemRequest  = "1Gi"
	ThanosStoreContainerID  = "statefulsets:mcoa:thanos"

	// Thanos Receive defaults
	DefaultReceiveRouterReplicas      = 1
	DefaultReceiveRouterReplication   = 3
	DefaultReceiveIngesterReplicas    = 3
	DefaultReceiveIngesterStorageSize = "10Gi"
	DefaultReceiveRetention           = "24h"
	DefaultReceiveTooFarInFuture      = "5m"
	DefaultReceiveRouterCPURequest    = "100m"
	DefaultReceiveRouterMemRequest    = "256Mi"
	DefaultReceiveIngesterCPURequest  = "300m"
	DefaultReceiveIngesterMemRequest  = "512Mi"
	ThanosReceiveRouterContainerID    = "deployments:mcoa:thanos-receive-router"
	ThanosReceiveIngesterContainerID  = "statefulsets:mcoa:thanos-receive-ingester"
	DefaultReceiveHashringName        = "default"

	// Thanos Query defaults
	DefaultQueryReplicas           = 2
	DefaultQueryCPURequest         = "300m"
	DefaultQueryMemRequest         = "1Gi"
	DefaultQueryFrontendReplicas   = 2
	DefaultQueryFrontendCPURequest = "100m"
	DefaultQueryFrontendMemRequest = "256Mi"
	ThanosQueryContainerID         = "deployments:mcoa:thanos-query"
	ThanosQueryFrontendContainerID = "deployments:mcoa:thanos-query-frontend"

	// Thanos Ruler defaults
	DefaultRulerReplicas        = 3
	DefaultRulerRetention       = "2h"
	DefaultRulerEvalInterval    = "1m"
	DefaultRulerStorageSize     = "1Gi"
	DefaultRulerCPURequest      = "100m"
	DefaultRulerMemRequest      = "256Mi"
	DefaultRulerAlertmanagerURL = "http://alertmanager.open-cluster-management-observability.svc:9093"
	ThanosRulerContainerID      = "statefulsets:mcoa:thanos-ruler"

	// Thanos Compact defaults
	DefaultCompactStorageSize    = "10Gi"
	DefaultCompactCPURequest     = "100m"
	DefaultCompactMemRequest     = "512Mi"
	DefaultCompactRetentionRaw   = "365d"
	DefaultCompactRetention5m    = "365d"
	DefaultCompactRetention1h    = "365d"
	DefaultCompactConcurrency    = int32(4)
	DefaultCompactDownsampleConc = int32(4)
	ThanosCompactContainerID     = "statefulsets:mcoa:thanos-compact"

	// Memcached defaults
	DefaultMemcachedImage              = "quay.io/ocm-observability/memcached:1.6.3-alpine"
	DefaultMemcachedExporterImage      = "quay.io/prometheus/memcached-exporter:v0.9.0"
	DefaultMemcachedReplicas           = int32(3)
	DefaultMemcachedMemoryLimitMB      = int32(1024)
	DefaultMemcachedConnectionLimit    = int32(1024)
	DefaultMemcachedMaxItemSize        = "1m"
	DefaultMemcachedMaxItemSizeCache   = "1MiB"
	DefaultMemcachedCPURequest         = "45m"
	DefaultMemcachedMemRequest         = "1124Mi"
	DefaultMemcachedMemLimit           = "1224Mi"
	DefaultMemcachedExporterCPURequest = "5m"
	DefaultMemcachedExporterMemRequest = "50Mi"
	MemcachedStoreName                 = "memcached-store"
	MemcachedQueryFrontendName         = "memcached-query-frontend"
	StoreCacheConfigSecretName         = "thanos-store-index-cache-config"
	StoreCacheConfigSecretKey          = "config.yaml"
	QueryFECacheConfigSecretName       = "thanos-query-frontend-cache-config"
	QueryFECacheConfigSecretKey        = "config.yaml"

	// Thanos object storage configuration
	ObjectStorageSecretName = "thanos-object-storage"
	ObjectStorageSecretKey  = "thanos.yaml"
)
