// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package handlers

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
)

// Options contains the right-sizing configuration for helm values
type Options struct {
	NamespaceRightSizing      ComponentOptions
	VirtualizationRightSizing ComponentOptions
}

// ComponentOptions contains the configuration for a single right-sizing component
type ComponentOptions struct {
	Enabled         bool
	PrometheusRules []*monitoringv1.PrometheusRule
	ScrapeConfigs   []*cooprometheusv1alpha1.ScrapeConfig
}
