package config

const (
	// Thanos Operator
	ThanosOperatorAppName = "thanos-operator"
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

	// Thanos object storage configuration
	ObjectStorageSecretName = "thanos-object-storage"
	ObjectStorageSecretKey  = "thanos.yaml"
)
