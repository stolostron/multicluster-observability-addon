// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package handlers

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
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
