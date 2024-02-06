package manifests

import (
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
)

func buildOtelColSpec(resources Options) (*otelv1alpha1.OpenTelemetryCollectorSpec, error) {
	otelCol := resources.OpenTelemetryCollector

	return &otelCol.Spec, nil
}
