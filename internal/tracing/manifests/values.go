package manifests

import (
	"encoding/json"

	"k8s.io/klog/v2"
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

	klog.Info("Building OTEL Collector instance")
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
