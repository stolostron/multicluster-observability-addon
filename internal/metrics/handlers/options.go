package handlers

import (
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Platform      Collector
	UserWorkloads Collector
	Secrets       []corev1.Secret
	ConfigMaps    []*corev1.ConfigMap
	ClusterName   string
	ClusterID     string
	Images        ImagesOptions
}

type Collector struct {
	PrometheusAgent *prometheusalpha1.PrometheusAgent
	ScrapeConfigs   []*prometheusalpha1.ScrapeConfig
	Rules           []*prometheusv1.PrometheusRule
}

type ImagesOptions struct {
	PrometheusOperator       string
	PrometheusConfigReloader string
	HAProxy                  string
	KubeRBACProxy            string
}
