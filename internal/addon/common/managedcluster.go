package common

import (
	"slices"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

// GetManagedClusterID returns the cluster ID with following priotity order:
// 1. The `clusterID` label on the `ManagedCluster` resource.
// 2. The `id.k8s.io` cluster claim on the `ManagedCluster` resource.
// 3. The name of the `ManagedCluster` resource.
func GetManagedClusterID(cluster *clusterv1.ManagedCluster) string {
	if val, ok := cluster.Labels[addoncfg.ManagedClusterLabelClusterID]; ok {
		return val
	}

	idx := slices.IndexFunc(cluster.Status.ClusterClaims, func(c clusterv1.ManagedClusterClaim) bool {
		return c.Name == addoncfg.ClusterClaimClusterID
	})
	if idx != -1 {
		return cluster.Status.ClusterClaims[idx].Value
	}

	return cluster.Name
}
