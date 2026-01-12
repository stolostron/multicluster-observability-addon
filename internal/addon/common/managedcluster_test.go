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

func TestIsOpenShiftVendor(t *testing.T) {
	cases := []struct {
		name     string
		cluster  *clusterv1.ManagedCluster
		expected bool
	}{
		{
			name: "with vendor label",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"vendor": "OpenShift",
					},
				},
			},
			expected: true,
		},
		{
			name: "with openshiftVersion label",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"openshiftVersion": "4.15.0",
					},
				},
			},
			expected: true,
		},
		{
			name: "with id.openshift.io claim",
			cluster: &clusterv1.ManagedCluster{
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  "id.openshift.io",
							Value: "cluster-id",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "with other vendor",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"vendor": "Other",
					},
				},
			},
			expected: false,
		},
		{
			name:     "with empty cluster",
			cluster:  &clusterv1.ManagedCluster{},
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isOpenShift := IsOpenShiftVendor(c.cluster)
			if isOpenShift != c.expected {
				t.Errorf("expected IsOpenShiftVendor to be %v, got %v", c.expected, isOpenShift)
			}
		})
	}
}
