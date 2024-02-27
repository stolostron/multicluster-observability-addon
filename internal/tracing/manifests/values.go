package manifests

import (
	"encoding/json"

	"github.com/go-logr/logr"
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

func BuildValues(log logr.Logger, opts Options) (TracingValues, error) {
	values := TracingValues{
		Enabled: true,
	}

	secrets, err := buildSecrets(opts)
	if err != nil {
		return values, err
	}
	values.Secrets = secrets

	log.V(1).Info("Building OTEL Collector instance")
	otelColSpec, err := buildOtelColSpec(opts)
	if err != nil {
		return values, err
	}

	b, err := json.Marshal(otelColSpec)
	if err != nil {
		return values, err
	}

	values.OTELColSpec = string(b)

	return values, nil
}
