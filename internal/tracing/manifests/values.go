package manifests

import (
	"encoding/json"
)

type TracingValues struct {
	Enabled                bool          `json:"enabled"`
	InstrumentationEnabled bool          `json:"instrumentationEnabled"`
	OTELColSpec            string        `json:"otelColSpec"`
	InstrumenationSpec     string        `json:"instrumentationSpec"`
	Secrets                []SecretValue `json:"secrets"`
}

type SecretValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func BuildValues(opts Options) (TracingValues, error) {
	values := TracingValues{
		Enabled: true,
	}

	secrets, err := buildSecrets(opts)
	if err != nil {
		return values, err
	}
	values.Secrets = secrets

	b, err := json.Marshal(opts.OpenTelemetryCollector.Spec)
	if err != nil {
		return values, err
	}

	values.OTELColSpec = string(b)

	if opts.Instrumentation != nil {
		values.InstrumentationEnabled = true
		b, err = json.Marshal(opts.Instrumentation.Spec)
		if err != nil {
			return values, err
		}
		values.InstrumenationSpec = string(b)
	}

	return values, nil
}
