package manifests

import (
	"encoding/json"

	"k8s.io/klog"
)

type TracingValues struct {
	Enabled     bool   `json:"enabled"`
	OTELColSpec string `json:"otelColSpec"`
}

func BuildValues(opts Options) (TracingValues, error) {
	values := TracingValues{
		Enabled: true,
	}

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
