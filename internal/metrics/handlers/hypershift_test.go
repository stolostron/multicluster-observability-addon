// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package handlers

import (
	"testing"

	"github.com/go-logr/logr"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIsHypershiftEnabled(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name: "local cluster with hypershift addon available",
			labels: map[string]string{
				config.LocalManagedClusterLabel:  "true",
				config.HypershiftAddonStateLabel: "available",
			},
			expected: true,
		},
		{
			name: "not local cluster",
			labels: map[string]string{
				config.HypershiftAddonStateLabel: "available",
			},
			expected: false,
		},
		{
			name: "hypershift addon disabled",
			labels: map[string]string{
				config.LocalManagedClusterLabel:  "true",
				config.HypershiftAddonStateLabel: "disabled",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: tc.labels},
			}
			assert.Equal(t, tc.expected, IsHypershiftEnabled(mc))
		})
	}
}

func TestBuildHypershiftResources(t *testing.T) {
	t.Parallel()

	newScrapeConfig := func(name string, labels map[string]string) *cooprometheusv1alpha1.ScrapeConfig {
		return &cooprometheusv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		}
	}
	newRule := func(name string, labels map[string]string) *prometheusv1.PrometheusRule {
		return &prometheusv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		}
	}

	configResources := []client.Object{
		newScrapeConfig("etcd-sc", config.EtcdHcpUserWorkloadPrometheusMatchLabels),
		newScrapeConfig("apiserver-sc", config.ApiserverHcpUserWorkloadPrometheusMatchLabels),
		newScrapeConfig("uwl-sc", config.UserWorkloadPrometheusMatchLabels), // unrelated, must be ignored
		newRule("etcd-rule", config.EtcdHcpUserWorkloadPrometheusMatchLabels),
		newRule("apiserver-rule", config.ApiserverHcpUserWorkloadPrometheusMatchLabels),
	}

	o := &OptionsBuilder{Logger: logr.Discard()}
	opts := &Options{}
	o.buildHypershiftResources(opts, "spoke-id", configResources)

	// Only HCP scrapeConfigs are delivered.
	require.Len(t, opts.UserWorkloads.ScrapeConfigs, 2)
	names := []string{opts.UserWorkloads.ScrapeConfigs[0].Name, opts.UserWorkloads.ScrapeConfigs[1].Name}
	assert.ElementsMatch(t, []string{"etcd-sc", "apiserver-sc"}, names)

	// Both HCP rules are delivered.
	require.Len(t, opts.UserWorkloads.Rules, 2)

	// The federation filter is baked into each HCP scrapeConfig.
	for _, sc := range opts.UserWorkloads.ScrapeConfigs {
		require.Len(t, sc.Spec.MetricRelabelConfigs, 2, "scrapeConfig %s", sc.Name)
		assert.Equal(t, "keep", sc.Spec.MetricRelabelConfigs[0].Action)
		assert.Equal(t, config.ClusterIDMetricLabel, string(sc.Spec.MetricRelabelConfigs[0].SourceLabels[0]))
		assert.Equal(t, ".+", sc.Spec.MetricRelabelConfigs[0].Regex)
		assert.Equal(t, "keep", sc.Spec.MetricRelabelConfigs[1].Action)
		assert.Equal(t, config.ManagementClusterIDMetricLabel, string(sc.Spec.MetricRelabelConfigs[1].SourceLabels[0]))
		assert.Equal(t, "spoke-id", sc.Spec.MetricRelabelConfigs[1].Regex)
	}
}
