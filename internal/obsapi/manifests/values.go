package manifests

const (
	legacyReceiveEndpoint = "http://observability-thanos-receive.open-cluster-management-observability.svc.cluster.local:19291"
	mcoaReceiveEndpoint   = "http://thanos-receive-router-mcoa.open-cluster-management-observability.svc.cluster.local:19291"

	legacyReadEndpoint = "http://observability-thanos-query-frontend.open-cluster-management-observability.svc.cluster.local:9090"
	mcoaReadEndpoint   = "http://thanos-query-frontend-mcoa.open-cluster-management-observability.svc.cluster.local:9090"
)

type ObsAPIResources struct {
	CPURequest    string `json:"cpuRequest"`
	MemoryRequest string `json:"memoryRequest"`
}

type ObsAPIValues struct {
	Enabled              bool            `json:"enabled"`
	MetricsWriteEndpoint string          `json:"metricsWriteEndpoint"`
	MetricsReadEndpoint  string          `json:"metricsReadEndpoint"`
	Replicas             int32           `json:"replicas"`
	Resources            ObsAPIResources `json:"resources"`
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

func BuildValues(isHubCluster, obsAPIEnabled, thanosOperatorEnabled bool, instanceSize string) *ObsAPIValues {
	if !isHubCluster || !obsAPIEnabled {
		return nil
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

	return &ObsAPIValues{
		Enabled:              true,
		MetricsWriteEndpoint: writeEndpoint,
		MetricsReadEndpoint:  readEndpoint,
		Replicas:             replicas,
		Resources: ObsAPIResources{
			CPURequest:    cpu,
			MemoryRequest: mem,
		},
	}
}
