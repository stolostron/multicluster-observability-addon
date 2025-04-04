package handlers

import (
	"fmt"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrometheusAgentBuilder applies configuration and invariants to an existing PrometheusAgent
type PrometheusAgentBuilder struct {
	Agent               *prometheusalpha1.PrometheusAgent
	Name                string
	RemoteWriteEndpoint string
	ClusterName         string
	ClusterID           string
	EnvoyConfigMapName  string
	EnvoyProxyImage     string
	PrometheusImage     string
	MatchLabels         map[string]string
}

// Build applies all configurations and invariants to the existing PrometheusAgent
func (p *PrometheusAgentBuilder) Build() *prometheusalpha1.PrometheusAgent {
	return p.setCommonFields().
		setPrometheusRemoteWriteConfig().
		setWatchedResources().
		setEnvoyProxySidecar().
		Agent
}

func (p *PrometheusAgentBuilder) setCommonFields() *PrometheusAgentBuilder {
	spec := &p.Agent.Spec.CommonPrometheusFields

	spec.Replicas = toPtr(int32(1))
	spec.ArbitraryFSAccessThroughSMs = prometheusv1.ArbitraryFSAccessThroughSMsConfig{
		Deny: true,
	}
	spec.Image = &p.PrometheusImage
	spec.Version = ""
	spec.ServiceAccountName = p.Name
	spec.WALCompression = toPtr(true)

	return p
}

func (p *PrometheusAgentBuilder) setPrometheusRemoteWriteConfig() *PrometheusAgentBuilder {
	spec := &p.Agent.Spec.CommonPrometheusFields
	spec.Secrets = append(spec.Secrets, config.HubCASecretName, config.ClientCertSecretName)
	spec.RemoteWrite = []prometheusv1.RemoteWriteSpec{
		p.createRemoteWriteSpec(),
	}

	return p
}

func (p *PrometheusAgentBuilder) createRemoteWriteSpec() prometheusv1.RemoteWriteSpec {
	return prometheusv1.RemoteWriteSpec{
		URL:           p.RemoteWriteEndpoint,
		RemoteTimeout: prometheusv1.Duration("30s"),
		TLSConfig: &prometheusv1.TLSConfig{
			CAFile:   p.formatSecretPath(config.HubCASecretName, "ca.crt"),
			CertFile: p.formatSecretPath(config.ClientCertSecretName, "tls.crt"),
			KeyFile:  p.formatSecretPath(config.ClientCertSecretName, "tls.key"),
		},
		WriteRelabelConfigs: p.createWriteRelabelConfigs(),
		QueueConfig:         p.createQueueConfig(),
	}
}

func (p *PrometheusAgentBuilder) createWriteRelabelConfigs() []prometheusv1.RelabelConfig {
	return []prometheusv1.RelabelConfig{
		{
			Replacement: toPtr(p.ClusterName),
			TargetLabel: "cluster",
			Action:      "replace",
		},
		{
			Replacement: toPtr(p.ClusterID),
			TargetLabel: "clusterID",
			Action:      "replace",
		},
		{
			SourceLabels: []prometheusv1.LabelName{"exported_job"},
			TargetLabel:  "job",
			Action:       "replace",
		},
		{
			SourceLabels: []prometheusv1.LabelName{"exported_instance"},
			TargetLabel:  "instance",
			Action:       "replace",
		},
		{
			Regex:  "exported_job|exported_instance",
			Action: "labeldrop",
		},
	}
}

func (p *PrometheusAgentBuilder) createQueueConfig() *prometheusv1.QueueConfig {
	return &prometheusv1.QueueConfig{
		BatchSendDeadline: toPtr(prometheusv1.Duration("15s")),
		Capacity:          12000,
		MaxShards:         3,
		MinShards:         1,
		MaxSamplesPerSend: 4000,
		MinBackoff:        toPtr(prometheusv1.Duration("1s")),
		MaxBackoff:        toPtr(prometheusv1.Duration("30s")),
		RetryOnRateLimit:  true,
	}
}

func (p *PrometheusAgentBuilder) setWatchedResources() *PrometheusAgentBuilder {
	p.Agent.Spec.ScrapeConfigSelector = &metav1.LabelSelector{
		MatchLabels: p.MatchLabels,
	}
	if p.Name == config.UserWorkloadMetricsCollectorApp {
		// Listen to all namespaces
		p.Agent.Spec.ScrapeConfigNamespaceSelector = &metav1.LabelSelector{}
	}
	p.clearSelectors()
	return p
}

func (p *PrometheusAgentBuilder) setEnvoyProxySidecar() *PrometheusAgentBuilder {
	envoyVolumes := []corev1.Volume{
		{
			Name: "envoy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.EnvoyConfigMapName,
					},
				},
			},
		},
		{
			Name: "prom-server-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: config.PrometheusCAConfigMapName,
					},
				},
			},
		},
	}

	p.Agent.Spec.Volumes = append(p.Agent.Spec.Volumes, envoyVolumes...)
	p.Agent.Spec.Containers = append(
		p.Agent.Spec.Containers,
		p.createEnvoyContainer(),
	)
	return p
}

func (p *PrometheusAgentBuilder) createEnvoyContainer() corev1.Container {
	return corev1.Container{
		Name:  "envoy",
		Image: p.EnvoyProxyImage,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3m"),
				corev1.ResourceMemory: resource.MustParse("60Mi"),
			},
		},
		Command: []string{"/bin/sh", "-c"},
		Args: []string{
			"/usr/local/bin/envoy -c /etc/envoy/envoy.yaml",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "envoy-config",
				MountPath: "/etc/envoy",
			},
			{
				Name:      "prom-server-ca",
				MountPath: "/etc/certs",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             toPtr(true),
			Privileged:               toPtr(false),
			AllowPrivilegeEscalation: toPtr(false),
			ReadOnlyRootFilesystem:   toPtr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}
}

func (p *PrometheusAgentBuilder) formatSecretPath(secretName, fileName string) string {
	return fmt.Sprintf("/etc/prometheus/secrets/%s/%s", secretName, fileName)
}

func (b *PrometheusAgentBuilder) clearSelectors() {
	spec := &b.Agent.Spec.CommonPrometheusFields
	spec.ServiceMonitorNamespaceSelector = nil
	spec.ServiceMonitorSelector = nil
	spec.PodMonitorNamespaceSelector = nil
	spec.PodMonitorSelector = nil
	spec.ProbeNamespaceSelector = nil
	spec.ProbeSelector = nil
}

func toPtr[T any](v T) *T {
	return &v
}
