// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package handlers

import (
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
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
