package manifests

const (
	legacyReceiveEndpoint = "http://observability-thanos-receive.open-cluster-management-observability.svc.cluster.local:19291"
	mcoaReceiveEndpoint   = "http://thanos-receive-mcoa-router.open-cluster-management-observability.svc.cluster.local:19291"
)

type ObsAPIValues struct {
	Enabled              bool   `json:"enabled"`
	MetricsWriteEndpoint string `json:"metricsWriteEndpoint"`
}

func BuildValues(isHubCluster, obsAPIEnabled, thanosOperatorEnabled bool) *ObsAPIValues {
	if !isHubCluster || !obsAPIEnabled {
		return nil
	}

	endpoint := legacyReceiveEndpoint
	if thanosOperatorEnabled {
		endpoint = mcoaReceiveEndpoint
	}

	return &ObsAPIValues{
		Enabled:              true,
		MetricsWriteEndpoint: endpoint,
	}
}
