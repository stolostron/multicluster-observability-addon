// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package handlers

import (
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsHypershiftEnabled(managedCluster *clusterv1.ManagedCluster) bool {
	isLocalCluster, ok := managedCluster.Labels[config.LocalManagedClusterLabel]
	if !ok || isLocalCluster != "true" {
		return false
	}

	hypershiftAddonStatus, ok := managedCluster.Labels[config.HypershiftAddonStateLabel]
	if !ok {
		return false
	}

	if hypershiftAddonStatus == "disabled" {
		return false
	}

	return true
}

func (o *OptionsBuilder) buildHypershiftResources(opts *Options, clusterID string, configResources []client.Object) {
	etcdScrapeConfigs := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](configResources, config.EtcdHcpUserWorkloadPrometheusMatchLabels)
	apiserverScrapeConfigs := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](configResources, config.ApiserverHcpUserWorkloadPrometheusMatchLabels)
	etcdRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.EtcdHcpUserWorkloadPrometheusMatchLabels)
	apiserverRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.ApiserverHcpUserWorkloadPrometheusMatchLabels)

	if len(etcdScrapeConfigs) == 0 && len(apiserverScrapeConfigs) == 0 {
		o.Logger.V(1).Info("no HCP scrapeConfigs found in configuration resources, skipping hypershift resources")
		return
	}

	federationFilter := []cooprometheusv1.RelabelConfig{
		{
			SourceLabels: []cooprometheusv1.LabelName{config.ClusterIDMetricLabel}, // hosted clusterID is set
			Regex:        ".+",
			Action:       "keep",
		},
		{
			SourceLabels: []cooprometheusv1.LabelName{config.ManagementClusterIDMetricLabel}, // scraped by this management cluster
			Regex:        clusterID,
			Action:       "keep",
		},
	}

	hcpScrapeConfigs := append(etcdScrapeConfigs, apiserverScrapeConfigs...)
	for _, sc := range hcpScrapeConfigs {
		sc.Spec.MetricRelabelConfigs = append(sc.Spec.MetricRelabelConfigs, federationFilter...)
	}

	opts.UserWorkloads.ScrapeConfigs = append(opts.UserWorkloads.ScrapeConfigs, hcpScrapeConfigs...)
	opts.UserWorkloads.Rules = append(opts.UserWorkloads.Rules, etcdRules...)
	opts.UserWorkloads.Rules = append(opts.UserWorkloads.Rules, apiserverRules...)
}
