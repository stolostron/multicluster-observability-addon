package handlers

import (
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Platform        Collector
	UserWorkloads   Collector
	Secrets         []*corev1.Secret
	ClusterName     string
	ClusterID       string
	ClusterVendor   string
	Images          mconfig.ImageOverrides
	IsHub           bool
	COOIsSubscribed bool
	// CRDEstablishedAnnotation is injected into the Prometheus Operator Deployment to trigger a
	// restart when optional CRDs (PrometheusAgent, ScrapeConfig) become available. This
	// prevents synchronization issues by ensuring the operator can watch these resources upon startup.
	CRDEstablishedAnnotation string
}

type Collector struct {
	ConfigMaps      []*corev1.ConfigMap
	PrometheusAgent *cooprometheusv1alpha1.PrometheusAgent
	ScrapeConfigs   []*cooprometheusv1alpha1.ScrapeConfig
	Rules           []*prometheusv1.PrometheusRule
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

// IsOCPCluster checks if the cluster vendor is OCP.
func (o *Options) IsOCPCluster() bool {
	return o.ClusterVendor == mconfig.ManagedClusterLabelVendorOCPValue
}
