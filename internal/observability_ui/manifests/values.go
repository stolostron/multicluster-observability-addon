package manifests

type UIValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts Options) *UIValues {
	return &UIValues{opts.Enabled}
}
