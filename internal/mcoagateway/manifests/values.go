package manifests

const (
	legacyReceiveEndpoint = "http://observability-thanos-receive.open-cluster-management-observability.svc.cluster.local:19291"
	mcoaReceiveEndpoint   = "http://thanos-receive-router-mcoa.open-cluster-management-observability.svc.cluster.local:19291"

	legacyReadEndpoint = "http://observability-thanos-query-frontend.open-cluster-management-observability.svc.cluster.local:9090"
	mcoaReadEndpoint   = "http://thanos-query-frontend-mcoa.open-cluster-management-observability.svc.cluster.local:9090"

	// logsGatewayEndpoint points at the gateway of the real, Managed LokiStack
	// (see internal/logging/manifests/var.go: DefaultStorageLSName, LoggingNamespace)
	// that MCOA ships via ManifestWork. It is NOT the hub-side "mcoa-default-*"
	// LokiStack, which is intentionally left Unmanaged and never gets a gateway.
	logsGatewayEndpoint = "https://mcoa-logging-managed-storage-gateway-http.openshift-logging.svc:8080"
)

type MCOAGatewayValues struct {
	Enabled                        bool     `json:"enabled"`
	ThanosOperatorEnabled          bool     `json:"thanosOperatorEnabled"`
	AlertmanagerAccessorSecretName string   `json:"alertmanagerAccessorSecretName,omitempty"`
	AlertmanagerGatewayHost        string   `json:"alertmanagerGatewayHost,omitempty"`
	MetricsWriteEndpoint           string   `json:"metricsWriteEndpoint"`
	MetricsReadEndpoint            string   `json:"metricsReadEndpoint"`
	LogsEnabled                    bool     `json:"logsEnabled"`
	LogsWriteEndpoint              string   `json:"logsWriteEndpoint"`
	LogsReadEndpoint               string   `json:"logsReadEndpoint"`
	AlertmanagerEndpoints          []string `json:"alertmanagerEndpoints,omitempty"`
}

func BuildValues(isHubCluster, mcoaGatewayEnabled, thanosOperatorEnabled bool, logsEnabled bool) *MCOAGatewayValues {
	if !mcoaGatewayEnabled {
		return nil
	}
	if !isHubCluster {
		return &MCOAGatewayValues{}
	}

	writeEndpoint := legacyReceiveEndpoint
	readEndpoint := legacyReadEndpoint
	if thanosOperatorEnabled {
		writeEndpoint = mcoaReceiveEndpoint
		readEndpoint = mcoaReadEndpoint
	}

	return &MCOAGatewayValues{
		Enabled:               true,
		ThanosOperatorEnabled: thanosOperatorEnabled,
		MetricsWriteEndpoint:  writeEndpoint,
		MetricsReadEndpoint:   readEndpoint,
		LogsEnabled:           logsEnabled,
		LogsWriteEndpoint:     logsGatewayEndpoint,
		LogsReadEndpoint:      logsGatewayEndpoint,
	}
}
