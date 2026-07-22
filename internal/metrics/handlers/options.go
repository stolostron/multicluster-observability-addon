package handlers

import (
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

type Options struct {
	PlatformAlertsEnabled     bool
	UserWorkloadAlertsEnabled bool
	Platform                  Collector
	UserWorkloads             Collector
	Secrets                   []*corev1.Secret
	ConfigMaps                []*corev1.ConfigMap
	HubEndpoint               string
	ClusterName               string
	HubClusterID              string
	ClusterID                 string
	IsOpenShiftVendor         bool
	InstallNamespace          string
	Images                    mconfig.ImageOverrides
	IsHub                     bool
	COOIsSubscribed           bool
	Tolerations               []corev1.Toleration
	NodeSelector              map[string]string
	ResourceReqs              []addonv1beta1.ContainerResourceRequirements
	NodeExporter              addon.NodeExporterOptions
	// CRDEstablishedAnnotation is injected into the Prometheus Operator Deployment to trigger a
	// restart when optional CRDs (PrometheusAgent, ScrapeConfig) become available. This
	// prevents synchronization issues by ensuring the operator can watch these resources upon startup.
	CRDEstablishedAnnotation string
	ProxyConfig              addon.ProxyConfig

	// Thanos holds the hub-only Thanos component specifications.
	// These are only populated when IsHub is true and platform metrics collection is enabled.
	Thanos ThanosOptions

	MonitoringStackPatches []MonitoringStackPatch

	PrometheusServerRemoteWrite []cooprometheusv1.RemoteWriteSpec
}

type MonitoringStackPatch struct {
	Namespace       string
	Name            string
	RemoteWriteSpec *cooprometheusv1.RemoteWriteSpec
}

// ThanosOptions holds the Thanos operator CR specs for hub deployment.
type ThanosOptions struct {
	Receive *thanosv1alpha1.ThanosReceive
	Query   *thanosv1alpha1.ThanosQuery
	Compact *thanosv1alpha1.ThanosCompact
	Store   *thanosv1alpha1.ThanosStore
	Ruler   *thanosv1alpha1.ThanosRuler
}

type Collector struct {
	PrometheusAgent *cooprometheusv1alpha1.PrometheusAgent
	ScrapeConfigs   []*cooprometheusv1alpha1.ScrapeConfig
	Rules           []*prometheusv1.PrometheusRule
	COORules        []*cooprometheusv1.PrometheusRule
	ServiceMonitors []*prometheusv1.ServiceMonitor // For deploying HCPs service monitor (user workloads)
}

type UIOptions struct {
	Enabled bool
}

// IsPlatformEnabled checks if platform monitoring is configured.
func (o *Options) IsPlatformEnabled() bool {
	return o.Platform.PrometheusAgent != nil
}

// IsUserWorkloadsEnabled checks if user workload monitoring is configured.
func (o *Options) IsUserWorkloadsEnabled() bool {
	return o.UserWorkloads.PrometheusAgent != nil
}
