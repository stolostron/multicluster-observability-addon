package common_test

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

func TestValidateConfigNamespaces(t *testing.T) {
	for _, tt := range []struct {
		name            string
		addonNamespace  string
		configNamespace string
		wantErr         bool
	}{
		{name: "shared config", addonNamespace: "cluster-a", configNamespace: addoncfg.InstallNamespace},
		{name: "per-cluster config", addonNamespace: "cluster-a", configNamespace: "cluster-a"},
		{name: "hub logging config", addonNamespace: "cluster-a", configNamespace: "openshift-logging", wantErr: true},
		{name: "other cluster config", addonNamespace: "cluster-a", configNamespace: "cluster-b", wantErr: true},
		{name: "empty config namespace", addonNamespace: "cluster-a", wantErr: true},
		{name: "both namespaces empty", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mcAddon := &addonapiv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: tt.addonNamespace},
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1alpha1.ConfigReference{
						{},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group: "observability.openshift.io", Resource: addoncfg.ClusterLogForwardersResource,
							},
							ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: "instance", Namespace: tt.configNamespace},
						},
					},
				},
			}
			err := common.ValidateConfigNamespaces(mcAddon)
			if tt.wantErr {
				require.ErrorIs(t, err, common.ErrInvalidConfigNamespace)
				require.Contains(t, err.Error(), "clusterlogforwarders")
				require.Contains(t, err.Error(), "instance")
				return
			}
			require.NoError(t, err)
		})
	}

	require.NoError(t, common.ValidateConfigNamespaces(&addonapiv1alpha1.ManagedClusterAddOn{}))
	require.NoError(t, common.ValidateConfigNamespaces(&addonapiv1alpha1.ManagedClusterAddOn{
		Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1alpha1.ConfigReference{{}},
		},
	}))
}

func TestValidateConfigNamespacesUsesDesiredConfig(t *testing.T) {
	mcAddon := &addonapiv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "cluster-a"},
		Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group: "observability.openshift.io", Resource: addoncfg.ClusterLogForwardersResource,
					},
					// DesiredConfig takes precedence over the deprecated ConfigReferent fields when set.
					ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: "instance", Namespace: addoncfg.InstallNamespace},
					DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
						ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: "instance", Namespace: "openshift-logging"},
					},
				},
			},
		},
	}
	err := common.ValidateConfigNamespaces(mcAddon)
	require.ErrorIs(t, err, common.ErrInvalidConfigNamespace)
	require.Contains(t, err.Error(), "instance")
}

func TestGetObjectKeys(t *testing.T) {
	tests := []struct {
		name      string
		configRef []addonapiv1alpha1.ConfigReference
		group     string
		resource  string
		expected  int
	}{
		{
			name: "Matching config with name and namespace",
			configRef: []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{Group: "apps", Resource: "deployments"},
					ConfigReferent:      addonapiv1alpha1.ConfigReferent{Name: "test-deploy", Namespace: "test-ns"},
				},
			},
			group:    "apps",
			resource: "deployments",
			expected: 1,
		},
		{
			name: "Mismatched group",
			configRef: []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{Group: "apps", Resource: "deployments"},
					ConfigReferent:      addonapiv1alpha1.ConfigReferent{Name: "test-deploy", Namespace: "test-ns"},
				},
			},
			group:    "batch",
			resource: "deployments",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.GetObjectKeys(tt.configRef, tt.group, tt.resource)
			require.Len(t, result, tt.expected)
		})
	}
}
