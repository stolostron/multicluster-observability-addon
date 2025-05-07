package resource

import (
	"fmt"
	"slices"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// PrometheusAgentBuilder applies configuration and invariants to an existing PrometheusAgent
type PrometheusAgentBuilder struct {
	Agent               *prometheusalpha1.PrometheusAgent
	IsUwl               bool
	SAName              string
	RemoteWriteEndpoint string
	PrometheusImage     string
	MatchLabels         map[string]string
}

// Build applies all configurations and invariants to the existing PrometheusAgent
func (p *PrometheusAgentBuilder) Build() *prometheusalpha1.PrometheusAgent {
	return p.setCommonFields().
		setPrometheusRemoteWriteConfig().
		setWatchedResources().
		setScrapeClasses().
		Agent
}

func (p *PrometheusAgentBuilder) setCommonFields() *PrometheusAgentBuilder {
	p.Agent.TypeMeta.Kind = prometheusalpha1.PrometheusAgentsKind
	p.Agent.TypeMeta.APIVersion = prometheusalpha1.SchemeGroupVersion.String()
	spec := &p.Agent.Spec.CommonPrometheusFields

	spec.ArbitraryFSAccessThroughSMs = prometheusv1.ArbitraryFSAccessThroughSMsConfig{
		Deny: true,
	}
	spec.Image = &p.PrometheusImage
	spec.Version = ""
	spec.ServiceAccountName = p.SAName
	spec.WALCompression = ptr.To(true)

	return p
}

func (p *PrometheusAgentBuilder) setPrometheusRemoteWriteConfig() *PrometheusAgentBuilder {
	spec := &p.Agent.Spec.CommonPrometheusFields
	spec.Secrets = append(spec.Secrets, config.HubCASecretName, config.ClientCertSecretName)

	// keep user remote write configs and enforce ours
	desiredRemoteWriteSpec := prometheusv1.RemoteWriteSpec{
		URL:           p.RemoteWriteEndpoint,
		Name:          ptr.To(config.RemoteWriteCfgName),
		RemoteTimeout: ptr.To(prometheusv1.Duration("30s")),
		TLSConfig: &prometheusv1.TLSConfig{
			CAFile:   p.formatSecretPath(config.HubCASecretName, "ca.crt"),
			CertFile: p.formatSecretPath(config.ClientCertSecretName, "tls.crt"),
			KeyFile:  p.formatSecretPath(config.ClientCertSecretName, "tls.key"),
		},
		// WriteRelabelConfigs is set individually for each managed cluster in order to enforce cluster identification labels
		QueueConfig: p.createQueueConfig(),
	}

	var found bool
	p.Agent.Spec.RemoteWrite = slices.DeleteFunc(p.Agent.Spec.RemoteWrite, func(e prometheusv1.RemoteWriteSpec) bool {
		if e.Name != desiredRemoteWriteSpec.Name {
			return false
		}
		if !found {
			found = true
			return false
		}
		return true
	})

	index := slices.IndexFunc(p.Agent.Spec.RemoteWrite, func(e prometheusv1.RemoteWriteSpec) bool { return e.Name == desiredRemoteWriteSpec.Name })
	if index >= 0 {
		p.Agent.Spec.RemoteWrite[index] = desiredRemoteWriteSpec
	} else {
		p.Agent.Spec.RemoteWrite = append(p.Agent.Spec.RemoteWrite, desiredRemoteWriteSpec)
	}

	return p
}

func (p *PrometheusAgentBuilder) createQueueConfig() *prometheusv1.QueueConfig {
	return &prometheusv1.QueueConfig{
		BatchSendDeadline: ptr.To(prometheusv1.Duration("15s")),
		Capacity:          12000,
		MaxShards:         3,
		MinShards:         1,
		MaxSamplesPerSend: 4000,
		MinBackoff:        ptr.To(prometheusv1.Duration("1s")),
		MaxBackoff:        ptr.To(prometheusv1.Duration("30s")),
		RetryOnRateLimit:  true,
	}
}

func (p *PrometheusAgentBuilder) setWatchedResources() *PrometheusAgentBuilder {
	p.Agent.Spec.ScrapeConfigSelector = &metav1.LabelSelector{
		MatchLabels: p.MatchLabels,
	}
	if p.IsUwl {
		// Listen to all namespaces
		p.Agent.Spec.ScrapeConfigNamespaceSelector = &metav1.LabelSelector{}
	}
	p.clearSelectors()
	return p
}

func (p *PrometheusAgentBuilder) setScrapeClasses() *PrometheusAgentBuilder {
	p.Agent.Spec.ConfigMaps = append(p.Agent.Spec.ConfigMaps, config.PrometheusCAConfigMapName)
	desiredScrapeClass := prometheusv1.ScrapeClass{
		Authorization: &prometheusv1.Authorization{
			CredentialsFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		},
		Name: "ocp-monitoring",
		TLSConfig: &prometheusv1.TLSConfig{
			CAFile: fmt.Sprintf("/etc/prometheus/configmaps/%s/service-ca.crt", config.PrometheusCAConfigMapName),
		},
	}

	// Ensure there is a single ocp-monitring class
	var found bool
	p.Agent.Spec.ScrapeClasses = slices.DeleteFunc(p.Agent.Spec.ScrapeClasses, func(e prometheusv1.ScrapeClass) bool {
		if e.Name != desiredScrapeClass.Name {
			return false
		}
		if !found {
			found = true
			return false
		}
		return true
	})

	index := slices.IndexFunc(p.Agent.Spec.ScrapeClasses, func(e prometheusv1.ScrapeClass) bool { return e.Name == desiredScrapeClass.Name })
	if index >= 0 {
		p.Agent.Spec.ScrapeClasses[index] = desiredScrapeClass
	} else {
		p.Agent.Spec.ScrapeClasses = append(p.Agent.Spec.ScrapeClasses, desiredScrapeClass)
	}

	return p
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
