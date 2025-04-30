package handlers

import (
	"fmt"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// PrometheusAgentBuilder applies configuration and invariants to an existing PrometheusAgent
type PrometheusAgentBuilder struct {
	Agent                    *prometheusalpha1.PrometheusAgent
	Name                     string
	RemoteWriteEndpoint      string
	ClusterName              string
	ClusterID                string
	PrometheusImage          string
	MatchLabels              map[string]string
	IsHypershiftLocalCluster bool
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
	spec := &p.Agent.Spec.CommonPrometheusFields

	spec.Replicas = ptr.To(int32(1))
	spec.ArbitraryFSAccessThroughSMs = prometheusv1.ArbitraryFSAccessThroughSMsConfig{
		Deny: true,
	}
	spec.Image = &p.PrometheusImage
	spec.Version = ""
	spec.ServiceAccountName = p.Name
	spec.WALCompression = ptr.To(true)

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
		RemoteTimeout: ptr.To(prometheusv1.Duration("30s")),
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
	ret := make([]prometheusv1.RelabelConfig, 0, 7)
	if p.IsHypershiftLocalCluster {
		// Don't overwrite the clusterID label as some are set to the hosted cluster ID (for hosted etcd and apiserver)
		// These rules ensure that the correct management cluster labels are set if the clusterID label differs from the current cluster one.
		// If the clusterID it the current cluster one, nothing is done.
		var isNotHcpTmpLabel prometheusv1.LabelName = "__tmp_is_not_hcp"
		ret = append(ret,
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        "^$", // Is empty
				TargetLabel:  config.ClusterNameMetricLabel,
				Action:       "replace",
				Replacement:  &p.ClusterName,
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        "^$", // Is empty
				TargetLabel:  config.ClusterIDMetricLabel,
				Action:       "replace",
				Replacement:  &p.ClusterID,
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        p.ClusterID,
				TargetLabel:  string(isNotHcpTmpLabel),
				Action:       "replace",
				Replacement:  ptr.To("true"),
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{isNotHcpTmpLabel},
				Regex:        "^$", // Is not the current clusterID and is not empty
				TargetLabel:  config.ManagementClusterIDMetricLabel,
				Action:       "replace",
				Replacement:  &p.ClusterID,
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{isNotHcpTmpLabel},
				Regex:        "^$", // Is not the current clusterID and is not empty
				TargetLabel:  config.ManagementClusterNameMetricLabel,
				Action:       "replace",
				Replacement:  &p.ClusterName,
			},
		)
	} else {
		// If not hypershift hub, enforce the clusterID and Name on all metrics
		ret = append(ret,
			prometheusv1.RelabelConfig{
				Replacement: ptr.To(p.ClusterName),
				TargetLabel: config.ClusterNameMetricLabel,
				Action:      "replace",
			},
			prometheusv1.RelabelConfig{
				Replacement: ptr.To(p.ClusterID),
				TargetLabel: config.ClusterIDMetricLabel,
				Action:      "replace",
			})
	}

	return append(ret,
		prometheusv1.RelabelConfig{
			SourceLabels: []prometheusv1.LabelName{"exported_job"},
			TargetLabel:  "job",
			Action:       "replace",
		},
		prometheusv1.RelabelConfig{
			SourceLabels: []prometheusv1.LabelName{"exported_instance"},
			TargetLabel:  "instance",
			Action:       "replace",
		},
		prometheusv1.RelabelConfig{
			Regex:  "exported_job|exported_instance",
			Action: "labeldrop",
		})
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
	if p.Name == config.UserWorkloadMetricsCollectorApp {
		// Listen to all namespaces
		p.Agent.Spec.ScrapeConfigNamespaceSelector = &metav1.LabelSelector{}
	}
	p.clearSelectors()
	return p
}

func (p *PrometheusAgentBuilder) setScrapeClasses() *PrometheusAgentBuilder {
	p.Agent.Spec.ConfigMaps = append(p.Agent.Spec.ConfigMaps, config.PrometheusCAConfigMapName)

	p.Agent.Spec.ScrapeClasses = []prometheusv1.ScrapeClass{
		{
			Authorization: &prometheusv1.Authorization{
				CredentialsFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
			},
			Name: "ocp-monitoring",
			TLSConfig: &prometheusv1.TLSConfig{
				CAFile: fmt.Sprintf("/etc/prometheus/configmaps/%s/service-ca.crt", config.PrometheusCAConfigMapName),
			},
		},
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
