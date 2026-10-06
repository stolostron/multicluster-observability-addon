package common

import (
	"slices"
	"strconv"

	clusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	clusterlifecycleconstants "github.com/stolostron/cluster-lifecycle-api/constants"
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

func IsHubCluster(cluster *clusterv1.ManagedCluster) bool {
	return cluster.Labels[clusterlifecycleconstants.SelfManagedClusterLabelKey] == "true"
}

func IsOpenShiftVendor(cluster *clusterv1.ManagedCluster) bool {
	if cluster == nil {
		return false
	}

	// For e2e testing non OCP cases more easily, we use a special annotation to override the cluster vendor
	if override := VendorIsOverridden(cluster); override != "" {
		return override == string(clusterinfov1beta1.KubeVendorOpenShift)
	}
	if cluster.Labels[clusterinfov1beta1.LabelKubeVendor] == string(clusterinfov1beta1.KubeVendorOpenShift) {
		return true
	}

	if _, ok := cluster.Labels[clusterinfov1beta1.OCPVersion]; ok {
		return true
	}

	idx := slices.IndexFunc(cluster.Status.ClusterClaims, func(c clusterv1.ManagedClusterClaim) bool {
		return c.Name == "id.openshift.io"
	})
	return idx != -1
}

// IsCOOExternallyInstalledOnSpoke reports whether COO was installed by someone other than
// MCOA. Used by the COO Subscription install decision: MCOA keeps its own Subscription
// ("mcoa") but backs off when COO was installed externally.
func IsCOOExternallyInstalledOnSpoke(cluster *clusterv1.ManagedCluster) (externallyInstalled bool, hasReport bool) {
	status := getClusterClaim(cluster, addoncfg.CooStatusClaimName)
	switch status {
	case "not-installed", "mcoa":
		return false, true
	case "external":
		return true, true
	default:
		return false, false
	}
}

// IsCOOInstalledOnSpoke reports whether COO is installed on a spoke, regardless of who
// installed it (MCOA or an external party). Returns (true, true) when COO is installed,
// (false, true) when not installed, and (false, false) when the endpoint operator hasn't
// reported yet.
func IsCOOInstalledOnSpoke(cluster *clusterv1.ManagedCluster) (installed bool, hasReport bool) {
	status := getClusterClaim(cluster, addoncfg.CooStatusClaimName)
	switch status {
	case "not-installed":
		return false, true
	case "mcoa", "external":
		return true, true
	default:
		return false, false
	}
}

func getClusterClaim(cluster *clusterv1.ManagedCluster, name string) string {
	idx := slices.IndexFunc(cluster.Status.ClusterClaims, func(c clusterv1.ManagedClusterClaim) bool {
		return c.Name == name
	})
	if idx == -1 {
		return ""
	}
	return cluster.Status.ClusterClaims[idx].Value
}

// IsOCPVersionAtLeast checks if the managed cluster's major OCP version is at least the given value.
// Returns false for non-OpenShift clusters or when the version label is missing.
func IsOCPVersionAtLeast(cluster *clusterv1.ManagedCluster, major int) bool {
	if cluster == nil {
		return false
	}
	majorStr, ok := cluster.Labels[clusterinfov1beta1.OCPVersionMajor]
	if !ok {
		return false
	}
	majorVal, err := strconv.Atoi(majorStr)
	if err != nil {
		return false
	}
	return majorVal >= major
}

func VendorIsOverridden(cluster *clusterv1.ManagedCluster) string {
	vendorOverride := cluster.Annotations[addoncfg.VendorOverrideAnnotationKey]
	if vendorOverride != "" {
		return vendorOverride
	}

	return ""
}
