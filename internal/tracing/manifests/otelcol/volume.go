package otelcol

import (
	"github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureVolumes(otelCol *v1beta1.OpenTelemetryCollector, secret corev1.Secret) {
	v := corev1.Volume{
		Name: secret.Name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret.Name,
			},
		},
	}

	otelCol.Spec.Volumes = append(otelCol.Spec.Volumes, v)
}
