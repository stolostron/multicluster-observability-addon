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

type ObsAPIResources struct {
	CPURequest    string `json:"cpuRequest"`
	MemoryRequest string `json:"memoryRequest"`
}

type MCOAGatewayValues struct {
	Enabled                        bool            `json:"enabled"`
	ThanosOperatorEnabled          bool            `json:"thanosOperatorEnabled"`
	AlertmanagerAccessorSecretName string          `json:"alertmanagerAccessorSecretName,omitempty"`
	AlertmanagerGatewayHost        string          `json:"alertmanagerGatewayHost,omitempty"`
	MetricsWriteEndpoint           string          `json:"metricsWriteEndpoint"`
	MetricsReadEndpoint            string          `json:"metricsReadEndpoint"`
	LogsEnabled                    bool            `json:"logsEnabled"`
	LogsWriteEndpoint              string          `json:"logsWriteEndpoint"`
	LogsReadEndpoint               string          `json:"logsReadEndpoint"`
	AlertmanagerEndpoints          []string        `json:"alertmanagerEndpoints,omitempty"`
	Replicas                       int32           `json:"replicas"`
	Resources                      ObsAPIResources `json:"resources"`
}

var obsAPICPURequest = map[string]string{
	"default": "20m",
	"small":   "250m",
	"medium":  "500m",
	"large":   "500m",
	"xlarge":  "1",
	"2xlarge": "1",
	"4xlarge": "1500m",
}

var obsAPIMemoryRequest = map[string]string{
	"default": "128Mi",
	"small":   "256Mi",
	"medium":  "512Mi",
	"large":   "1Gi",
	"xlarge":  "2Gi",
	"2xlarge": "2Gi",
	"4xlarge": "4Gi",
}

var obsAPIReplicas = map[string]int32{
	"default": 2,
	"small":   3,
	"medium":  3,
	"large":   6,
	"xlarge":  6,
	"2xlarge": 8,
	"4xlarge": 12,
}

func BuildValues(isHubCluster, mcoaGatewayEnabled, thanosOperatorEnabled bool, instanceSize string, logsEnabled bool) *MCOAGatewayValues {
	if !isHubCluster || !mcoaGatewayEnabled {
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

	if instanceSize == "" {
		instanceSize = "default"
	}

	cpu := obsAPICPURequest[instanceSize]
	if cpu == "" {
		cpu = obsAPICPURequest["default"]
	}
	mem := obsAPIMemoryRequest[instanceSize]
	if mem == "" {
		mem = obsAPIMemoryRequest["default"]
	}
	replicas := obsAPIReplicas[instanceSize]
	if replicas == 0 {
		replicas = obsAPIReplicas["default"]
	}
	return &MCOAGatewayValues{
		Enabled:              true,
		MetricsWriteEndpoint: writeEndpoint,
		MetricsReadEndpoint:  readEndpoint,
		Replicas:             replicas,
		Resources: ObsAPIResources{
			CPURequest:    cpu,
			MemoryRequest: mem,
		},
		LogsEnabled:           logsEnabled,
		LogsWriteEndpoint:     logsGatewayEndpoint,
		LogsReadEndpoint:      logsGatewayEndpoint,
		ThanosOperatorEnabled: thanosOperatorEnabled,
	}
}
