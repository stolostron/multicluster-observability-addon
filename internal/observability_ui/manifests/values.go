package manifests

type UIValues struct {
	Enabled bool            `json:"enabled"`
	Logs    LogsUIValues    `json:"logging"`
	Metrics MetricsUIValues `json:"metrics"`
}

type LogsUIValues struct {
	Enabled bool `json:"enabled"`
}

type MetricsUIValues struct {
	Enabled bool         `json:"enabled"`
	ACM     ACMValues    `json:"acm"`
	Perses  PersesValues `json:"perses"`
}

type ACMValues struct {
	Enabled bool `json:"enabled"`
}

type PersesValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts Options) *UIValues {
	values := &UIValues{
		Enabled: opts.Enabled,
		Logs:    LogsUIValues{Enabled: opts.LogsUI.Enabled},
		Metrics: MetricsUIValues{
			Enabled: opts.MetricsUI.Enabled,
			ACM:     ACMValues{Enabled: opts.MetricsUI.ACM.Enabled},
			Perses:  PersesValues{Enabled: opts.MetricsUI.Perses.Enabled},
		},
	}
	return values
}
