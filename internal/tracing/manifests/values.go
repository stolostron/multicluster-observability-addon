package manifests

import (
	"encoding/json"
)

type TracingValues struct {
	Enabled     bool          `json:"enabled"`
	OTELColSpec string        `json:"otelColSpec"`
	Secrets     []SecretValue `json:"secrets"`
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

	return values, nil
}
