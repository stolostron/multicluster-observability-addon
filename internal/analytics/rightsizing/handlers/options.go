package handlers

import (
	"encoding/json"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
)

// Options contains the right-sizing configuration for helm values
type Options struct {
	NamespaceRightSizing      ComponentOptions
	VirtualizationRightSizing ComponentOptions
	WorkloadPodRightSizing    ComponentOptions
	GPURightSizing            ComponentOptions
	ScrapeConfig              *cooprometheusv1alpha1.ScrapeConfig
	PredictionEnabled         bool
	PredictionProvider        string
	PredictionConfig          json.RawMessage
}

// ComponentOptions contains the configuration for a single right-sizing component
type ComponentOptions struct {
	Enabled         bool
	PrometheusRules []*monitoringv1.PrometheusRule
}
