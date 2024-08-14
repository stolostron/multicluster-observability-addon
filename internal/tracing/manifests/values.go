package manifests

import (
	"encoding/json"

	"k8s.io/klog/v2"
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

	if opts.Instrumentation != nil {
		klog.Info("Building Instrumentation tracing instance")
		values.InstrumentationEnabled = true
		b, err = json.Marshal(opts.Instrumentation.Spec)
		if err != nil {
			return values, err
		}
		values.InstrumenationSpec = string(b)
	}

	return values, nil
}
