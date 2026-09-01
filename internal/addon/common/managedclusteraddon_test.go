package common_test

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestApplyManagedClusterAddOnConfigs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))

	lokiCfg := addonapiv1beta1.AddOnConfig{
		ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
			Group:    "loki.grafana.com",
			Resource: addoncfg.LokiStacksResource,
		},
		ConfigReferent: addonapiv1beta1.ConfigReferent{
			Name:      "mcoa-default-global",
			Namespace: addoncfg.InstallNamespace,
		},
	}
	clfCfg := addonapiv1beta1.AddOnConfig{
		ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
			Group:    "observability.openshift.io",
			Resource: addoncfg.ClusterLogForwardersResource,
		},
		ConfigReferent: addonapiv1beta1.ConfigReferent{
			Name:      "mcoa-default-global",
			Namespace: addoncfg.InstallNamespace,
		},
	}
	staleLokiCfg := addonapiv1beta1.AddOnConfig{
		ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
			Group:    "loki.grafana.com",
			Resource: addoncfg.LokiStacksResource,
		},
		ConfigReferent: addonapiv1beta1.ConfigReferent{
			Name:      "old-lokistack",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	hubNS := addoncfg.HubNamespace
	newMCAO := func(configs []addonapiv1beta1.AddOnConfig) *addonapiv1beta1.ManagedClusterAddOn {
		return &addonapiv1beta1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name:      addoncfg.Name,
				Namespace: hubNS,
			},
			Spec: addonapiv1beta1.ManagedClusterAddOnSpec{
				Configs: configs,
			},
		}
	}

	tests := []struct {
		name            string
		existing        []client.Object
		desired         []addonapiv1beta1.AddOnConfig
		expectErr       bool
		expectNotFound  bool
		expectedConfigs []addonapiv1beta1.AddOnConfig
	}{
		{
			name:            "adds lokistack config and keeps unrelated configs",
			existing:        []client.Object{newMCAO([]addonapiv1beta1.AddOnConfig{clfCfg})},
			desired:         []addonapiv1beta1.AddOnConfig{lokiCfg},
			expectedConfigs: []addonapiv1beta1.AddOnConfig{clfCfg, lokiCfg},
		},
		{
			name:            "no-op when desired config already present",
			existing:        []client.Object{newMCAO([]addonapiv1beta1.AddOnConfig{clfCfg, lokiCfg})},
			desired:         []addonapiv1beta1.AddOnConfig{lokiCfg},
			expectedConfigs: []addonapiv1beta1.AddOnConfig{clfCfg, lokiCfg},
		},
		{
			name:            "replaces stale lokistack config",
			existing:        []client.Object{newMCAO([]addonapiv1beta1.AddOnConfig{clfCfg, staleLokiCfg})},
			desired:         []addonapiv1beta1.AddOnConfig{lokiCfg},
			expectedConfigs: []addonapiv1beta1.AddOnConfig{clfCfg, lokiCfg},
		},
		{
			name:            "removes lokistack configs when desired is empty",
			existing:        []client.Object{newMCAO([]addonapiv1beta1.AddOnConfig{clfCfg, lokiCfg})},
			desired:         nil,
			expectedConfigs: []addonapiv1beta1.AddOnConfig{clfCfg},
		},
		{
			name:           "not found with desired configs returns NotFound",
			existing:       nil,
			desired:        []addonapiv1beta1.AddOnConfig{lokiCfg},
			expectErr:      true,
			expectNotFound: true,
		},
		{
			name:      "not found with empty desired is a no-op",
			existing:  nil,
			desired:   nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.existing...).Build()
			err := common.ApplyManagedClusterAddOnConfigs(t.Context(), logr.Discard(), fakeClient, hubNS, tt.desired, "loki.grafana.com", addoncfg.LokiStacksResource)
			if tt.expectErr {
				require.Error(t, err)
				if tt.expectNotFound {
					require.True(t, apierrors.IsNotFound(err))
				}
				return
			}
			require.NoError(t, err)

			if len(tt.existing) == 0 {
				return
			}

			got := &addonapiv1beta1.ManagedClusterAddOn{}
			require.NoError(t, fakeClient.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: hubNS}, got))
			require.ElementsMatch(t, tt.expectedConfigs, got.Spec.Configs)
		})
	}
}
