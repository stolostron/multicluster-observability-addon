package config

const (
	PrometheusControllerID          = "acm-observability"
	PlatformMetricsCollectorApp     = "acm-platform-metrics-collector"
	UserWorkloadMetricsCollectorApp = "acm-user-workload-metrics-collector"
	HubCASecretName                 = "observability-managed-cluster-certs"
	ClientCertSecretName            = "observability-controller-open-cluster-management.io-observability-signer-client-cert"
	PrometheusCAConfigMapName       = "prometheus-server-ca"
)
