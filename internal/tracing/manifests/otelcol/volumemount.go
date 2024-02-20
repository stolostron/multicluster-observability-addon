package otelcol

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureVolumeMounts(spec *v1alpha1.OpenTelemetryCollectorSpec, secret corev1.Secret) {
	vm := corev1.VolumeMount{
		Name:      secret.Name,
		MountPath: fmt.Sprintf("/%s", secret.Name),
	}

	spec.VolumeMounts = append(spec.VolumeMounts, vm)
}
