package resource

import (
	"fmt"
	"maps"
	"slices"

	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

// NewDefaultPrometheusAgent generates the default prometheusAgent resource containing sensible
// defaults that can be overridden by the user.
func NewDefaultPrometheusAgent(ns, name string, isUWL bool, placementRef addonv1alpha1.PlacementRef) *cooprometheusv1alpha1.PrometheusAgent {
	agent := &cooprometheusv1alpha1.PrometheusAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       cooprometheusv1alpha1.PrometheusAgentsKind,
			APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				Replicas: ptr.To(int32(1)),
				LogLevel: "warn",
				NodeSelector: map[string]string{
					"kubernetes.io/os": "linux",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("3m"),
						corev1.ResourceMemory: resource.MustParse("150Mi"),
					},
				},
				ScrapeInterval: "300s",
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: ptr.To(true),
				},
				ScrapeTimeout: cooprometheusv1.Duration("30s"),
				PortName:      "web", // set this value to the default to avoid triggering update when comparing the spec
			},
		},
	}

	agent.Name = name
	agent.Namespace = ns
	if agent.Labels == nil {
		agent.Labels = map[string]string{}
	}

	if isUWL {
		// Listen to all namespaces by default. Can be overridden by the user.
		agent.Spec.ScrapeConfigNamespaceSelector = &metav1.LabelSelector{}
	}

	maps.Copy(agent.Labels, makeConfigResourceLabels(isUWL, placementRef))

	return agent
}

func makeConfigResourceLabels(isUWL bool, placementRef addonv1alpha1.PlacementRef) map[string]string {
	appName := config.PlatformMetricsCollectorApp
	if isUWL {
		appName = config.UserWorkloadMetricsCollectorApp
	}
	return map[string]string{
		addoncfg.ManagedByK8sLabelKey:          addoncfg.Name,
		addoncfg.ComponentK8sLabelKey:          appName,
		addoncfg.PlacementRefNameLabelKey:      placementRef.Name,
		addoncfg.PlacementRefNamespaceLabelKey: placementRef.Namespace,
	}
}

// PrometheusAgentSSA applies configuration and invariants to an existing PrometheusAgent
// It is used to enforce mandatory fields with server-side apply.
type PrometheusAgentSSA struct {
	ExistingAgent       *cooprometheusv1alpha1.PrometheusAgent
	IsUwl               bool
	PrometheusImage     string
	KubeRBACProxyImage  string
	Labels              map[string]string
	RemoteWriteEndpoint string

	desiredAgent *cooprometheusv1alpha1.PrometheusAgent
}

// Build generate the prometheusAgent resource containing only fields that must be enforced using server-side apply.
func (p *PrometheusAgentSSA) Build() *cooprometheusv1alpha1.PrometheusAgent {
	// Ensure TypeMeta is always set, even if the existing agent doesn't have it
	typeMeta := p.ExistingAgent.TypeMeta
	if typeMeta.Kind == "" {
		typeMeta = metav1.TypeMeta{
			Kind:       cooprometheusv1alpha1.PrometheusAgentsKind,
			APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.String(),
		}
	}

	p.desiredAgent = &cooprometheusv1alpha1.PrometheusAgent{
		TypeMeta: typeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.ExistingAgent.Name,
			Namespace: p.ExistingAgent.Namespace,
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				ArbitraryFSAccessThroughSMs: cooprometheusv1.ArbitraryFSAccessThroughSMsConfig{
					Deny: true,
				},
				Image: ptr.To(""), // Make sure the image is not overridden
				PodMetadata: &cooprometheusv1.EmbeddedObjectMetadata{
					Labels: map[string]string{
						"app.kubernetes.io/part-of": config.AddonName,
					},
				},
				Version:            "",
				ServiceAccountName: config.PlatformMetricsCollectorApp,
				ServiceName:        ptr.To(config.PlatformMetricsCollectorApp),
				WALCompression:     ptr.To(true),
			},
		},
	}

	if p.IsUwl {
		p.desiredAgent.Spec.ServiceAccountName = config.UserWorkloadMetricsCollectorApp
		p.desiredAgent.Spec.ServiceName = ptr.To(config.UserWorkloadMetricsCollectorApp)
	}

	if len(p.PrometheusImage) > 0 {
		p.desiredAgent.Spec.Image = &p.PrometheusImage
	}

	if len(p.Labels) > 0 {
		p.desiredAgent.Labels = maps.Clone(p.ExistingAgent.Labels)
		if p.desiredAgent.Labels == nil {
			p.desiredAgent.Labels = map[string]string{}
		}
		maps.Copy(p.desiredAgent.Labels, p.Labels)
	}

	if p.desiredAgent.Labels == nil {
		p.desiredAgent.Labels = map[string]string{}
	}
	p.desiredAgent.Labels[addoncfg.BackupLabelKey] = addoncfg.BackupLabelValue

	p.setPrometheusRemoteWriteConfig()
	p.setWatchedResources()
	p.setScrapeClasses()
	p.setKubeRBACProxySidecar()

	return p.desiredAgent
}

func (p *PrometheusAgentSSA) setPrometheusRemoteWriteConfig() {
	// Add remote write secrets and keep user defined ones, keeping original order
	p.desiredAgent.Spec.Secrets = slices.Clone(p.ExistingAgent.Spec.Secrets)
	neededSecrets := []string{config.HubCASecretName, config.ClientCertSecretName}
	for _, secret := range neededSecrets {
		if !slices.Contains(p.ExistingAgent.Spec.Secrets, secret) {
			p.desiredAgent.Spec.Secrets = append(p.desiredAgent.Spec.Secrets, secret)
		}
	}

	// keep user remote write configs and enforce ours
	desiredRemoteWriteSpec := cooprometheusv1.RemoteWriteSpec{
		URL:           p.RemoteWriteEndpoint,
		Name:          ptr.To(config.RemoteWriteCfgName),
		RemoteTimeout: ptr.To(cooprometheusv1.Duration("30s")),
		TLSConfig: &cooprometheusv1.TLSConfig{
			CAFile:   p.formatSecretPath(config.HubCASecretName, "ca.crt"),
			CertFile: p.formatSecretPath(config.ClientCertSecretName, "tls.crt"),
			KeyFile:  p.formatSecretPath(config.ClientCertSecretName, "tls.key"),
		},
		// WriteRelabelConfigs is set individually for each managed cluster in order to enforce cluster identification labels
	}

	// Ensure there is a single instance of our config
	var found *cooprometheusv1.RemoteWriteSpec
	p.desiredAgent.Spec.RemoteWrite = slices.Clone(p.ExistingAgent.Spec.RemoteWrite)
	p.desiredAgent.Spec.RemoteWrite = slices.DeleteFunc(p.desiredAgent.Spec.RemoteWrite, func(e cooprometheusv1.RemoteWriteSpec) bool {
		if e.Name == nil || *e.Name != *desiredRemoteWriteSpec.Name {
			return false
		}
		if found == nil {
			found = &e
			return false
		}
		return true
	})

	// Keep some of the existing parameters, allowing the user to override them
	if found != nil {
		if found.QueueConfig != nil {
			desiredRemoteWriteSpec.QueueConfig = found.QueueConfig
		}
		if found.RemoteTimeout != nil {
			desiredRemoteWriteSpec.RemoteTimeout = found.RemoteTimeout
		}
	}

	// Insert or replace the config
	index := slices.IndexFunc(p.desiredAgent.Spec.RemoteWrite, func(e cooprometheusv1.RemoteWriteSpec) bool {
		return e.Name != nil && *e.Name == *desiredRemoteWriteSpec.Name
	})
	if index >= 0 {
		p.desiredAgent.Spec.RemoteWrite[index] = desiredRemoteWriteSpec
	} else {
		p.desiredAgent.Spec.RemoteWrite = append(p.desiredAgent.Spec.RemoteWrite, desiredRemoteWriteSpec)
	}
}

func (p *PrometheusAgentSSA) setWatchedResources() {
	if p.IsUwl {
		p.desiredAgent.Spec.ScrapeConfigSelector = &metav1.LabelSelector{
			MatchLabels: config.UserWorkloadPrometheusMatchLabels,
		}
	} else {
		p.desiredAgent.Spec.ScrapeConfigSelector = &metav1.LabelSelector{
			MatchLabels: config.PlatformPrometheusMatchLabels,
		}
		p.desiredAgent.Spec.ScrapeConfigNamespaceSelector = nil
	}
}

func (p *PrometheusAgentSSA) setScrapeClasses() {
	// The addon dynamically injects scrape classes based on the managed cluster type.
	// To avoid conflicts and confusion, we enforce that users can only customize
	// the MetricRelabelings for the addon's scrape classes on the hub PrometheusAgent CR.
	// Other fields like TLSConfig or Authorization are managed by the addon's values logic.
	p.desiredAgent.Spec.ScrapeClasses = slices.Clone(p.ExistingAgent.Spec.ScrapeClasses)
	for i, sc := range p.desiredAgent.Spec.ScrapeClasses {
		if sc.Name == config.ScrapeClassCfgName || sc.Name == config.NonOCPScrapeClassName {
			p.desiredAgent.Spec.ScrapeClasses[i] = cooprometheusv1.ScrapeClass{
				Name:              sc.Name,
				MetricRelabelings: sc.MetricRelabelings,
			}
		}
	}

	// We also remove the associated configmap if manually added by the user,
	// as it's only injected for OCP clusters by the values logic.
	p.desiredAgent.Spec.ConfigMaps = slices.Clone(p.ExistingAgent.Spec.ConfigMaps)
	p.desiredAgent.Spec.ConfigMaps = slices.DeleteFunc(p.desiredAgent.Spec.ConfigMaps, func(e string) bool {
		return e == config.PrometheusCAConfigMapName
	})
}

func (p *PrometheusAgentSSA) setKubeRBACProxySidecar() {
	tlsSecret := config.PlatformRBACProxyTLSSecret
	if p.IsUwl {
		tlsSecret = config.UserWorkloadRBACProxyTLSSecret
	}
	newVolumes := []corev1.Volume{
		{
			Name: "kube-rbac-proxy-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: tlsSecret,
				},
			},
		},
	}

	p.desiredAgent.Spec.Volumes = append(p.desiredAgent.Spec.Volumes, newVolumes...)
	p.desiredAgent.Spec.Containers = append(
		p.desiredAgent.Spec.Containers,
		p.createKubeRbacProxyContainer(),
	)
}

func (p PrometheusAgentSSA) createKubeRbacProxyContainer() corev1.Container {
	return corev1.Container{
		Name:  "kube-rbac-proxy",
		Image: p.KubeRBACProxyImage,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1m"),
				corev1.ResourceMemory: resource.MustParse("15Mi"),
			},
		},
		Args: []string{
			fmt.Sprintf("--secure-listen-address=0.0.0.0:%d", config.RBACProxyPort),
			"--upstream=http://127.0.0.1:9090",
			"--tls-cert-file=/etc/tls/private/tls.crt",
			"--tls-private-key-file=/etc/tls/private/tls.key",
			"--logtostderr=true",
			"--allow-paths=/metrics",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "kube-rbac-proxy-tls",
				MountPath: "/etc/tls/private",
				ReadOnly:  true,
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: int32(config.RBACProxyPort),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			Privileged:               ptr.To(false),
			AllowPrivilegeEscalation: ptr.To(false),
			ReadOnlyRootFilesystem:   ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}
}

func (p *PrometheusAgentSSA) formatSecretPath(secretName, fileName string) string {
	return fmt.Sprintf("/etc/prometheus/secrets/%s/%s", secretName, fileName)
}
