package common_test

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetAddOnDeploymentConfig(t *testing.T) {
	tests := []struct {
		name         string
		mcAddon      *addonapiv1beta1.ManagedClusterAddOn
		existingAODC *addonapiv1beta1.AddOnDeploymentConfig
		expectedErr  error
	}{
		{
			name: "No AODC reference",
			mcAddon: &addonapiv1beta1.ManagedClusterAddOn{
				Status: addonapiv1beta1.ManagedClusterAddOnStatus{
					ConfigReferences: nil,
				},
			},
			expectedErr: common.ErrMissingAODCRef,
		},
		{
			name: "Multiple AODC references",
			mcAddon: &addonapiv1beta1.ManagedClusterAddOn{
				Status: addonapiv1beta1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1beta1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addoncfg.AddonDeploymentConfigResource,
							},
							DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
								ConfigReferent: addonapiv1beta1.ConfigReferent{
									Name:      "foo",
									Namespace: "foo",
								},
							},
						},
						{
							ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addoncfg.AddonDeploymentConfigResource,
							},
							DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
								ConfigReferent: addonapiv1beta1.ConfigReferent{
									Name:      "bar",
									Namespace: "bar",
								},
							},
						},
					},
				},
			},
			expectedErr: common.ErrMultipleAODCRef,
		},
		{
			name: "AODC reference found",
			mcAddon: &addonapiv1beta1.ManagedClusterAddOn{
				Status: addonapiv1beta1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1beta1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addoncfg.AddonDeploymentConfigResource,
							},
							DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
								ConfigReferent: addonapiv1beta1.ConfigReferent{
									Name:      "foo",
									Namespace: "foo",
								},
							},
						},
					},
				},
			},
			existingAODC: &addonapiv1beta1.AddOnDeploymentConfig{
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
			//nolint:staticcheck // client.Apply is deprecated, but alternative requires ApplyConfigurations which we don't have
			fakeAddonClient := fakeaddon.NewSimpleClientset()
			if tt.existingAODC != nil {
				//nolint:staticcheck // client.Apply is deprecated, but alternative requires ApplyConfigurations which we don't have
				fakeAddonClient = fakeaddon.NewSimpleClientset(tt.existingAODC)
			}
			scheme := runtime.NewScheme()
			require.NoError(t, addonapiv1beta1.Install(scheme))
			getter := addonutils.NewAddOnDeploymentConfigGetter(fakeAddonClient)

			// Call the function
			ctx := t.Context()
			_, err := common.GetAddOnDeploymentConfig(ctx, getter, tt.mcAddon)

			// require the results
			if tt.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetObjectKeys(t *testing.T) {
	tests := []struct {
		name      string
		configRef []addonapiv1beta1.ConfigReference
		group     string
		resource  string
		expected  []client.ObjectKey
	}{
		{
			name: "Matching config with name and namespace",
			configRef: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "apps",
						Resource: "deployments",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      "test-deploy",
							Namespace: "test-ns",
						},
					},
				},
			},
			group:    "apps",
			resource: "deployments",
			expected: []client.ObjectKey{
				{
					Name:      "test-deploy",
					Namespace: "test-ns",
				},
			},
		},
		{
			name: "Mismatched group",
			configRef: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "apps",
						Resource: "deployments",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      "test-deploy",
							Namespace: "test-ns",
						},
					},
				},
			},
			group:    "batch",
			resource: "deployments",
			expected: nil,
		},
		{
			name: "Mismatched resource",
			configRef: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "apps",
						Resource: "deployments",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Name:      "test-deploy",
							Namespace: "test-ns",
						},
					},
				},
			},
			group:    "apps",
			resource: "statefulsets",
			expected: nil,
		},
		{
			name: "Nil DesiredConfig",
			configRef: []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "apps",
						Resource: "deployments",
					},
					DesiredConfig: nil,
				},
			},
			group:    "apps",
			resource: "deployments",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.GetObjectKeys(tt.configRef, tt.group, tt.resource)
			require.Equal(t, tt.expected, result)
		})
	}
}
