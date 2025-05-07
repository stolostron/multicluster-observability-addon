package resource

import (
	"fmt"
	"maps"
	"slices"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// PrometheusAgentBuilder applies configuration and invariants to an existing PrometheusAgent
// It is used to enforce mandatory fields with server-side apply
type PrometheusAgentBuilder struct {
	ExistingAgent       *prometheusalpha1.PrometheusAgent
	IsUwl               bool
	SAName              string
	RemoteWriteEndpoint string
	PrometheusImage     string
	// MatchLabels         map[string]string
	Labels map[string]string

	desiredAgent *prometheusalpha1.PrometheusAgent
}

// Build applies all configurations and invariants to the existing PrometheusAgent
func (p *PrometheusAgentBuilder) Build() *prometheusalpha1.PrometheusAgent {
	p.desiredAgent = &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.ExistingAgent.Name,
			Namespace: p.ExistingAgent.Namespace,
		},
	}
	return p.setCommonFields().
		setPrometheusRemoteWriteConfig().
		setWatchedResources().
		setScrapeClasses().
		desiredAgent
}

func (p *PrometheusAgentBuilder) setCommonFields() *PrometheusAgentBuilder {
	p.desiredAgent.TypeMeta.Kind = prometheusalpha1.PrometheusAgentsKind
	p.desiredAgent.TypeMeta.APIVersion = prometheusalpha1.SchemeGroupVersion.String()
	if len(p.Labels) > 0 {
		p.desiredAgent.Labels = maps.Clone(p.ExistingAgent.Labels)
		if p.desiredAgent.Labels == nil {
			p.desiredAgent.Labels = map[string]string{}
		}
		maps.Copy(p.desiredAgent.Labels, p.Labels)
	}
	spec := &p.desiredAgent.Spec.CommonPrometheusFields

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
	spec := &p.desiredAgent.Spec.CommonPrometheusFields
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
	p.desiredAgent.Spec.RemoteWrite = slices.DeleteFunc(p.desiredAgent.Spec.RemoteWrite, func(e prometheusv1.RemoteWriteSpec) bool {
		if e.Name != desiredRemoteWriteSpec.Name {
			return false
		}
		if !found {
			found = true
			return false
		}
		return true
	})

	index := slices.IndexFunc(p.desiredAgent.Spec.RemoteWrite, func(e prometheusv1.RemoteWriteSpec) bool { return e.Name == desiredRemoteWriteSpec.Name })
	if index >= 0 {
		p.desiredAgent.Spec.RemoteWrite[index] = desiredRemoteWriteSpec
	} else {
		p.desiredAgent.Spec.RemoteWrite = append(p.desiredAgent.Spec.RemoteWrite, desiredRemoteWriteSpec)
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
	if p.IsUwl {
		p.desiredAgent.Spec.ScrapeConfigSelector = &metav1.LabelSelector{
			MatchLabels: config.UserWorkloadPrometheusMatchLabels,
		}
		// Listen to all namespaces
		p.desiredAgent.Spec.ScrapeConfigNamespaceSelector = &metav1.LabelSelector{}
	} else {
		p.desiredAgent.Spec.ScrapeConfigSelector = &metav1.LabelSelector{
			MatchLabels: config.PlatformPrometheusMatchLabels,
		}
	}
	p.clearSelectors()
	return p
}

func (p *PrometheusAgentBuilder) setScrapeClasses() *PrometheusAgentBuilder {
	p.desiredAgent.Spec.ConfigMaps = append(p.desiredAgent.Spec.ConfigMaps, config.PrometheusCAConfigMapName)
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
	p.desiredAgent.Spec.ScrapeClasses = slices.DeleteFunc(p.desiredAgent.Spec.ScrapeClasses, func(e prometheusv1.ScrapeClass) bool {
		if e.Name != desiredScrapeClass.Name {
			return false
		}
		if !found {
			found = true
			return false
		}
		return true
	})

	index := slices.IndexFunc(p.desiredAgent.Spec.ScrapeClasses, func(e prometheusv1.ScrapeClass) bool { return e.Name == desiredScrapeClass.Name })
	if index >= 0 {
		p.desiredAgent.Spec.ScrapeClasses[index] = desiredScrapeClass
	} else {
		p.desiredAgent.Spec.ScrapeClasses = append(p.desiredAgent.Spec.ScrapeClasses, desiredScrapeClass)
	}

	return p
}

func (p *PrometheusAgentBuilder) formatSecretPath(secretName, fileName string) string {
	return fmt.Sprintf("/etc/prometheus/secrets/%s/%s", secretName, fileName)
}

func (b *PrometheusAgentBuilder) clearSelectors() {
	spec := &b.desiredAgent.Spec.CommonPrometheusFields
	spec.ServiceMonitorNamespaceSelector = nil
	spec.ServiceMonitorSelector = nil
	spec.PodMonitorNamespaceSelector = nil
	spec.PodMonitorSelector = nil
	spec.ProbeNamespaceSelector = nil
	spec.ProbeSelector = nil
}
