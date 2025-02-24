package manifests

type ObservabilityOperatorValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts Options) *ObservabilityOperatorValues {
	return &ObservabilityOperatorValues{
		Enabled: opts.Enabled,
	}
}
