package manifests

type AnalyticsValues struct {
	IncidentDetectionValues IncidentDetectionValues `json:"incidentDetection"`
}

type IncidentDetectionValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts Options) *AnalyticsValues {
	return &AnalyticsValues{
		IncidentDetectionValues: IncidentDetectionValues{
			Enabled: opts.Enabled,
		},
	}
}
