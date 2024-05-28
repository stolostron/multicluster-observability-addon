package otelcol

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureVolumeMounts(otelCol *v1beta1.OpenTelemetryCollector, secret corev1.Secret) {
	vm := corev1.VolumeMount{
		Name:      secret.Name,
		MountPath: fmt.Sprintf("/%s", secret.Name),
	}

	otelCol.Spec.VolumeMounts = append(otelCol.Spec.VolumeMounts, vm)
}
