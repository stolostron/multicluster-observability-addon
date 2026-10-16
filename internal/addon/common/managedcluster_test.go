package common

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
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

func TestIsOCPVersionAtLeast(t *testing.T) {
	cases := []struct {
		name     string
		cluster  *clusterv1.ManagedCluster
		major    int
		expected bool
	}{
		{
			name: "OCP 5 meets minimum 5",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"openshiftVersion-major": "5"},
				},
			},
			major:    5,
			expected: true,
		},
		{
			name: "OCP 4 does not meet minimum 5",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"openshiftVersion-major": "4"},
				},
			},
			major:    5,
			expected: false,
		},
		{
			name: "OCP 6 meets minimum 5",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"openshiftVersion-major": "6"},
				},
			},
			major:    5,
			expected: true,
		},
		{
			name:     "missing label returns false",
			cluster:  &clusterv1.ManagedCluster{},
			major:    5,
			expected: false,
		},
		{
			name:     "nil cluster returns false",
			cluster:  nil,
			major:    5,
			expected: false,
		},
		{
			name: "non-numeric label returns false",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"openshiftVersion-major": "abc"},
				},
			},
			major:    5,
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := IsOCPVersionAtLeast(c.cluster, c.major)
			if result != c.expected {
				t.Errorf("expected IsOCPVersionAtLeast to be %v, got %v", c.expected, result)
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
			name: "with vendor override annotation (OpenShift)",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						addoncfg.VendorOverrideAnnotationKey: "OpenShift",
					},
				},
			},
			expected: true,
		},
		{
			name: "with vendor override annotation (Other)",
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						addoncfg.VendorOverrideAnnotationKey: "Other",
					},
					Labels: map[string]string{
						"vendor": "OpenShift",
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
