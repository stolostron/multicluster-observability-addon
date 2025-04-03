package manifests

type IncidentDetectionValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts Options) *IncidentDetectionValues {
	return &IncidentDetectionValues{
		Enabled: opts.Enabled,
	}
}
