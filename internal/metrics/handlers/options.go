package handlers

import (
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Platform      Collector
	UserWorkloads Collector
	Secrets       []*corev1.Secret
	ClusterName   string
	ClusterID     string
	Images        mconfig.ImageOverrides
	UI            UIOptions
}

type Collector struct {
	ConfigMaps      []*corev1.ConfigMap
	PrometheusAgent *prometheusalpha1.PrometheusAgent
	ScrapeConfigs   []*prometheusalpha1.ScrapeConfig
	Rules           []*prometheusv1.PrometheusRule
	ServiceMonitors []*prometheusv1.ServiceMonitor // For deploying HCPs service monitor (user workloads)
}

type UIOptions struct {
	Enabled bool
}
