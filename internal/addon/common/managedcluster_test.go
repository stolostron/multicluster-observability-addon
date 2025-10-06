package common

import (
	"testing"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

func TestGetManagedClusterID(t *testing.T) {
	cases := []struct {
		name              string
		cluster           *clusterv1.ManagedCluster
		expectedClusterID string
	}{
		{
			name: "with clusterID label",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
					Labels: map[string]string{
						addoncfg.ManagedClusterLabelClusterID: "cluster1-id-from-label",
					},
				},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  addoncfg.ClusterClaimClusterID,
							Value: "cluster1-id-from-claim",
						},
					},
				},
			},
			expectedClusterID: "cluster1-id-from-label",
		},
		{
			name: "with id.k8s.io claim",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster2",
				},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  addoncfg.ClusterClaimClusterID,
							Value: "cluster2-id-from-claim",
						},
					},
				},
			},
			expectedClusterID: "cluster2-id-from-claim",
		},
		{
			name: "with no label or claim",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster3",
				},
			},
			expectedClusterID: "cluster3",
		},
		{
			name:              "with empty cluster",
			cluster:           &clusterv1.ManagedCluster{},
			expectedClusterID: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterID := GetManagedClusterID(c.cluster)
			if clusterID != c.expectedClusterID {
				t.Errorf("expected cluster ID %s, got %s", c.expectedClusterID, clusterID)
			}
		})
	}
}
