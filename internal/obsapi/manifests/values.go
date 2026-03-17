package manifests

type ObsAPIValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(isHubCluster, obsAPIEnabled bool) *ObsAPIValues {
	if !isHubCluster || !obsAPIEnabled {
		return nil
	}

	return &ObsAPIValues{
		Enabled: true,
	}
}
