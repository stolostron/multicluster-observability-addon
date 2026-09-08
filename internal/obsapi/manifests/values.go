package manifests

const (
	legacyReceiveEndpoint = "http://observability-thanos-receive.open-cluster-management-observability.svc.cluster.local:19291"
	mcoaReceiveEndpoint   = "http://thanos-receive-router-mcoa.open-cluster-management-observability.svc.cluster.local:19291"

	legacyReadEndpoint = "http://observability-thanos-query-frontend.open-cluster-management-observability.svc.cluster.local:9090"
	mcoaReadEndpoint   = "http://thanos-query-frontend-mcoa.open-cluster-management-observability.svc.cluster.local:9090"
)

type ObsAPIValues struct {
	Enabled              bool   `json:"enabled"`
	MetricsWriteEndpoint string `json:"metricsWriteEndpoint"`
	MetricsReadEndpoint  string `json:"metricsReadEndpoint"`
}

func BuildValues(isHubCluster, obsAPIEnabled, thanosOperatorEnabled bool) *ObsAPIValues {
	if !isHubCluster || !obsAPIEnabled {
		return nil
	}

	writeEndpoint := legacyReceiveEndpoint
	readEndpoint := legacyReadEndpoint
	if thanosOperatorEnabled {
		writeEndpoint = mcoaReceiveEndpoint
		readEndpoint = mcoaReadEndpoint
	}

	return &ObsAPIValues{
		Enabled:              true,
		MetricsWriteEndpoint: writeEndpoint,
		MetricsReadEndpoint:  readEndpoint,
	}
}
