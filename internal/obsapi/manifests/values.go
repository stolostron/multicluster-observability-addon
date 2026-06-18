package manifests

type ObsAPIValues struct {
	Enabled     bool `json:"enabled"`
	LogsEnabled bool `json:"logsEnabled"`
}

func BuildValues(isHubCluster, obsAPIEnabled, logsEnabled bool) *ObsAPIValues {
	enabled := obsAPIEnabled || logsEnabled
	if !isHubCluster || !enabled {
		return nil
	}

	return &ObsAPIValues{
		Enabled:     true,
		LogsEnabled: logsEnabled,
	}
}
