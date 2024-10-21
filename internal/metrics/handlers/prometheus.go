package handlers

import (
	"fmt"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PrometheusAgentBuilder struct {
	Agent               *prometheusalpha1.PrometheusAgent // Base PrometheusAgent object to build upon
	Name                string                            // Resource and service account name
	RemoteWriteEndpoint string
	ClusterName         string
	ClusterID           string
	EnvoyConfigMapName  string
	EnvoyProxyImage     string
	PrometheusImage     string
	MatchLabels         map[string]string
}

// Build initializes and returns the fully constructed PrometheusAgent
func (p *PrometheusAgentBuilder) Build() *prometheusalpha1.PrometheusAgent {
	p.setCommonFields().
		setPrometheusRemoteWriteConfig().
		setWatchedResources().
		setEnvoyProxySidecar()
	return p.Agent
}

func (p *PrometheusAgentBuilder) setCommonFields() *PrometheusAgentBuilder {
	replicas := int32(1)
	p.Agent.Spec.CommonPrometheusFields.Replicas = &replicas
	p.Agent.Spec.CommonPrometheusFields.ArbitraryFSAccessThroughSMs = prometheusv1.ArbitraryFSAccessThroughSMsConfig{
		Deny: true,
	}
	// Set prometheus image
	p.Agent.Spec.CommonPrometheusFields.Image = &p.PrometheusImage
	p.Agent.Spec.CommonPrometheusFields.Version = ""
	p.Agent.Spec.CommonPrometheusFields.NodeSelector = map[string]string{
		"kubernetes.io/os": "linux",
	}
	p.Agent.Spec.CommonPrometheusFields.ServiceAccountName = p.Name
	p.Agent.Spec.CommonPrometheusFields.WALCompression = toPtr(true)
	// Add default scrape class with relabeling to add clusterID and cluster labels
	// p.Agent.Spec.CommonPrometheusFields.ScrapeClass = "openshift-monitoring"
	p.Agent.Spec.CommonPrometheusFields.ScrapeClasses = []prometheusv1.ScrapeClass{
		{
			Name:    "openshift-monitoring",
			Default: toPtr(true),
			MetricRelabelings: []prometheusv1.RelabelConfig{
				{
					Replacement: toPtr(p.ClusterName),
					TargetLabel: "testing",
					Action:      "replace",
				},
				// {
				// 	Replacement: toPtr(p.ClusterID),
				// 	TargetLabel: "clusterID",
				// 	Action:      "replace",
				// },
				// // TODO: remove
				// {
				// 	Replacement: toPtr("mcoa"),
				// 	TargetLabel: "collector",
				// 	Action:      "replace",
				// },
			},
		},
	}

	return p
}

func (p *PrometheusAgentBuilder) setPrometheusRemoteWriteConfig() *PrometheusAgentBuilder {
	secrets := []string{config.HubCASecretName, config.ClientCertSecretName}
	p.Agent.Spec.CommonPrometheusFields.Secrets = append(p.Agent.Spec.CommonPrometheusFields.Secrets, secrets...)

	p.Agent.Spec.CommonPrometheusFields.RemoteWrite = []prometheusv1.RemoteWriteSpec{
		{
			URL:           p.RemoteWriteEndpoint,
			RemoteTimeout: prometheusv1.Duration("30s"),
			TLSConfig: &prometheusv1.TLSConfig{
				CAFile:   p.formatSecretPath(config.HubCASecretName, "ca.crt"),
				CertFile: p.formatSecretPath(config.ClientCertSecretName, "tls.crt"),
				KeyFile:  p.formatSecretPath(config.ClientCertSecretName, "tls.key"),
			},
			WriteRelabelConfigs: []prometheusv1.RelabelConfig{
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
			},
			QueueConfig: &prometheusv1.QueueConfig{
				BatchSendDeadline: toPtr(prometheusv1.Duration("15s")),
				Capacity:          12000,
				MaxShards:         3,
				MinShards:         1,
				MaxSamplesPerSend: 4000,
				MinBackoff:        toPtr(prometheusv1.Duration("1s")),
				MaxBackoff:        toPtr(prometheusv1.Duration("30s")),
				RetryOnRateLimit:  true,
			},
		},
	}
	return p
}

func (p *PrometheusAgentBuilder) setWatchedResources() *PrometheusAgentBuilder {
	p.Agent.Spec.CommonPrometheusFields.ScrapeConfigSelector = &metav1.LabelSelector{
		MatchLabels: p.MatchLabels,
	}
	p.clearSelectors()
	return p
}

func (p *PrometheusAgentBuilder) setEnvoyProxySidecar() *PrometheusAgentBuilder {
	envoyProxyContainer := corev1.Container{
		Name:  "envoy",
		Image: p.EnvoyProxyImage,
		// ReadinessProbe: &corev1.Probe{
		// 	ProbeHandler: corev1.ProbeHandler{
		// 		HTTPGet: &corev1.HTTPGetAction{
		// 			Path: "/healthz",
		// 			Port: intstr.FromString("healthz"),
		// 		},
		// 	},
		// 	InitialDelaySeconds: 2,
		// 	PeriodSeconds:       5,
		// },
		// LivenessProbe: &corev1.Probe{
		// 	ProbeHandler: corev1.ProbeHandler{
		// 		HTTPGet: &corev1.HTTPGetAction{
		// 			Path: "/healthz",
		// 			Port: intstr.FromString("healthz"),
		// 		},
		// 	},
		// 	InitialDelaySeconds: 5,
		// 	PeriodSeconds:       10,
		// },

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

	p.Agent.Spec.Volumes = append(p.Agent.Spec.Volumes, []corev1.Volume{
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
	}...)

	p.Agent.Spec.CommonPrometheusFields.Containers = append(p.Agent.Spec.CommonPrometheusFields.Containers, envoyProxyContainer)
	return p
}

// Helper function to format the secret path
func (p *PrometheusAgentBuilder) formatSecretPath(secretName, fileName string) string {
	return fmt.Sprintf("/etc/prometheus/secrets/%s/%s", secretName, fileName)
}

// Clears all unnecessary selectors from the Prometheus spec
func (p *PrometheusAgentBuilder) clearSelectors() {
	p.Agent.Spec.CommonPrometheusFields.ServiceMonitorNamespaceSelector = nil
	p.Agent.Spec.CommonPrometheusFields.ServiceMonitorSelector = nil
	p.Agent.Spec.CommonPrometheusFields.PodMonitorNamespaceSelector = nil
	p.Agent.Spec.CommonPrometheusFields.PodMonitorSelector = nil
	p.Agent.Spec.CommonPrometheusFields.ProbeNamespaceSelector = nil
	p.Agent.Spec.CommonPrometheusFields.ProbeSelector = nil
}

func toPtr[T any](v T) *T {
	return &v
}
