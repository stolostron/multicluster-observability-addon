package config

const (
	PrometheusControllerID          = "acm-observability"
	PlatformMetricsCollectorApp     = "acm-platform-metrics-collector"
	UserWorkloadMetricsCollectorApp = "acm-user-workload-metrics-collector"
	HubCASecretName                 = "observability-managed-cluster-certs"
	ClientCertSecretName            = "observability-controller-open-cluster-management.io-observability-signer-client-cert"
	PrometheusCAConfigMapName       = "prometheus-server-ca"
	EnvoyImage                      = "registry.redhat.io/openshift-service-mesh/proxyv2-rhel9@sha256:153130dd485f60c9b1e120d51b8228fc3100afa9a7f500c3caa13ccd41520e99"
	EnvoyAdminPort                  = 9901
)

var (
	PlatformPrometheusMatchLabels = map[string]string{
		"app": PlatformMetricsCollectorApp,
	}
	UserWorkloadPrometheusMatchLabels = map[string]string{
		"app": UserWorkloadMetricsCollectorApp,
	}
)
