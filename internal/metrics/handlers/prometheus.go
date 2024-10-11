package handlers

import (
	"fmt"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	hubCASecretName      = "observability-managed-cluster-certs"
	clientCertSecretName = "observability-controller-open-cluster-management.io-observability-signer-client-cert"
	prometheusCaCm       = "prometheus-server-ca"
)

type PrometheusAgentBuilder struct {
	Agent                *prometheusalpha1.PrometheusAgent // Base PrometheusAgent object to build upon
	Name                 string                            // Resource and service account name
	RemoteWriteEndpoint  string
	ClusterName          string
	ClusterID            string
	HAProxyConfigMapName string
	HAProxyImage         string
	PrometheusImage      string
	MatchLabels          map[string]string
}

// Build initializes and returns the fully constructed PrometheusAgent
func (p *PrometheusAgentBuilder) Build() *prometheusalpha1.PrometheusAgent {
	p.setCommonFields().
		setPrometheusRemoteWriteConfig().
		setWatchedResources().
		setExternalLabels().
		setHAProxySidecar()
	return p.Agent
}

func (p *PrometheusAgentBuilder) setCommonFields() *PrometheusAgentBuilder {
	replicas := int32(1)
	p.Agent.Spec.CommonPrometheusFields.Replicas = &replicas
	p.Agent.Spec.CommonPrometheusFields.OverrideHonorLabels = true
	p.Agent.Spec.CommonPrometheusFields.ArbitraryFSAccessThroughSMs = prometheusv1.ArbitraryFSAccessThroughSMsConfig{
		Deny: true,
	}
	// Set prometheus image
	p.Agent.Spec.CommonPrometheusFields.Image = &p.PrometheusImage
	p.Agent.Spec.CommonPrometheusFields.Version = ""

	return p
}

func (p *PrometheusAgentBuilder) setPrometheusRemoteWriteConfig() *PrometheusAgentBuilder {
	secrets := []string{hubCASecretName, clientCertSecretName}
	p.Agent.Spec.CommonPrometheusFields.Secrets = append(p.Agent.Spec.CommonPrometheusFields.Secrets, secrets...)

	p.Agent.Spec.CommonPrometheusFields.RemoteWrite = []prometheusv1.RemoteWriteSpec{
		{
			URL: p.RemoteWriteEndpoint,
			TLSConfig: &prometheusv1.TLSConfig{
				CAFile:   p.formatSecretPath(hubCASecretName, "ca.crt"),
				CertFile: p.formatSecretPath(clientCertSecretName, "tls.crt"),
				KeyFile:  p.formatSecretPath(clientCertSecretName, "tls.key"),
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

func (p *PrometheusAgentBuilder) setExternalLabels() *PrometheusAgentBuilder {
	if p.Agent.Spec.CommonPrometheusFields.ExternalLabels == nil {
		p.Agent.Spec.CommonPrometheusFields.ExternalLabels = make(map[string]string)
	}
	p.Agent.Spec.CommonPrometheusFields.ExternalLabels["clusterID"] = p.ClusterID
	p.Agent.Spec.CommonPrometheusFields.ExternalLabels["cluster"] = p.ClusterName
	return p
}

func (p *PrometheusAgentBuilder) setHAProxySidecar() *PrometheusAgentBuilder {
	haProxyContainer := corev1.Container{
		Name:  "haproxy",
		Image: p.HAProxyImage,
		Ports: []corev1.ContainerPort{
			{
				Name:          "healthz",
				ContainerPort: 8081,
			},
			{
				Name:          "metrics",
				ContainerPort: 8082,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString("healthz"),
				},
			},
			InitialDelaySeconds: 2,
			PeriodSeconds:       5,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString("healthz"),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},

		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		Command: []string{"/bin/sh", "-c"},
		Args: []string{
			"export BEARER_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) && haproxy -f /etc/haproxy/haproxy.cfg",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "haproxy-config",
				MountPath: "/etc/haproxy",
			},
			{
				Name:      "prom-server-ca",
				MountPath: "/etc/haproxy/certs",
			},
		},
	}

	p.Agent.Spec.Volumes = append(p.Agent.Spec.Volumes, []corev1.Volume{
		{
			Name: "haproxy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.HAProxyConfigMapName,
					},
				},
			},
		},
		{
			Name: "prom-server-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: prometheusCaCm,
					},
				},
			},
		},
	}...)

	p.Agent.Spec.CommonPrometheusFields.Containers = append(p.Agent.Spec.CommonPrometheusFields.Containers, haProxyContainer)
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
