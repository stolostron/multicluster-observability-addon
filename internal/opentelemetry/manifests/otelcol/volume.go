package otelcol

import (
	"github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureVolumes(spec *v1alpha1.OpenTelemetryCollectorSpec, secret corev1.Secret) {
	v := corev1.Volume{
		Name: secret.Name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret.Name,
			},
		},
	}

	spec.Volumes = append(spec.Volumes, v)
}
