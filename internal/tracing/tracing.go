package tracing

import (
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TracingValues struct {
	Enabled bool `json:"enabled"`
}

func GetValuesFunc(k8s client.Client, cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) (TracingValues, error) {
	values := TracingValues{
		Enabled: false,
	}

	// Get necessary values

	values.Enabled = true
	return values, nil
}
