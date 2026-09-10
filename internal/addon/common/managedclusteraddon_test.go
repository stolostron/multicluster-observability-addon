package common_test

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetAddOnDeploymentConfig(t *testing.T) {
	tests := []struct {
		name         string
		mcAddon      *addonapiv1alpha1.ManagedClusterAddOn
		existingAODC *addonapiv1alpha1.AddOnDeploymentConfig
		expectedErr  error
	}{
		{
			name: "No AODC reference",
			mcAddon: &addonapiv1alpha1.ManagedClusterAddOn{
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: nil,
				},
			},
			expectedErr: common.ErrMissingAODCRef,
		},
		{
			name: "Multiple AODC references",
			mcAddon: &addonapiv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo"},
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1alpha1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addoncfg.AddonDeploymentConfigResource,
							},
							ConfigReferent: addonapiv1alpha1.ConfigReferent{
								Name:      "foo",
								Namespace: "foo",
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addoncfg.AddonDeploymentConfigResource,
							},
							ConfigReferent: addonapiv1alpha1.ConfigReferent{
								Name:      "bar",
								Namespace: "foo",
							},
						},
					},
				},
			},
			expectedErr: common.ErrMultipleAODCRef,
		},
		{
			name: "AODC reference found",
			mcAddon: &addonapiv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{Namespace: "foo"},
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1alpha1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addoncfg.AddonDeploymentConfigResource,
							},
							ConfigReferent: addonapiv1alpha1.ConfigReferent{
								Name:      "foo",
								Namespace: "foo",
							},
						},
					},
				},
			},
			existingAODC: &addonapiv1alpha1.AddOnDeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the existing AODC if provided
			objs := []client.Object{}
			if tt.existingAODC != nil {
				objs = append(objs, tt.existingAODC)
			}
			scheme := runtime.NewScheme()
			require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

			// Call the function
			ctx := t.Context()
			_, err := common.GetAddOnDeploymentConfig(ctx, fakeClient, tt.mcAddon)

			// require the results
			if tt.expectedErr != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedErr, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

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
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: "instance", Namespace: tt.configNamespace},
							},
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

func TestValidateConfigNamespacesUsesDeprecatedFields(t *testing.T) {
	mcAddon := &addonapiv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "cluster-a"},
		Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group: "observability.openshift.io", Resource: addoncfg.ClusterLogForwardersResource,
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: "instance", Namespace: "openshift-logging"},
				},
			},
		},
	}
	err := common.ValidateConfigNamespaces(mcAddon)
	require.ErrorIs(t, err, common.ErrInvalidConfigNamespace)
	require.Contains(t, err.Error(), "instance")
}

func TestGetAddOnDeploymentConfigRejectsNamespaceBeforeFetching(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))
	existingAODC := &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "instance", Namespace: "cluster-b"},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingAODC).Build()

	mcAddon := &addonapiv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: "cluster-a"},
		Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1alpha1.ConfigReference{{
				ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
					Group: addonutils.AddOnDeploymentConfigGVR.Group, Resource: addoncfg.AddonDeploymentConfigResource,
				},
				DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
					ConfigReferent: addonapiv1alpha1.ConfigReferent{Name: "instance", Namespace: "cluster-b"},
				},
			}},
		},
	}
	config, err := common.GetAddOnDeploymentConfig(t.Context(), fakeClient, mcAddon)
	require.ErrorIs(t, err, common.ErrInvalidConfigNamespace)
	require.Nil(t, config)
}
