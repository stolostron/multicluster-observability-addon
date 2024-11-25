package helm

import (
	"context"
	"testing"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = addonapiv1alpha1.AddToScheme(scheme.Scheme)
	_ = apiextensionsv1.AddToScheme(scheme.Scheme)
)

func Test_Mcoa_Disable_Charts(t *testing.T) {
	var (
		managedCluster        *clusterv1.ManagedCluster
		managedClusterAddOn   *addonapiv1alpha1.ManagedClusterAddOn
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
	)

	managedCluster = addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")

	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      "multicluster-observability-addon",
			},
		},
	}

	addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{},
		},
	}

	fakeKubeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(addOnDeploymentConfig).
		Build()

	agentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.McoaChartDir).
		WithGetValuesFuncs(GetValuesFunc(context.TODO(), fakeKubeClient, "myhub.foo.com")).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	objects, err := agentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Empty(t, objects)
}

func Test_Mcoa_Disable_Chart_Hub(t *testing.T) {
	var (
		managedCluster        *clusterv1.ManagedCluster
		managedClusterAddOn   *addonapiv1alpha1.ManagedClusterAddOn
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
	)

	managedCluster = addontesting.NewManagedCluster("cluster-1")
	managedCluster.Labels = map[string]string{
		"local-cluster": "true",
	}
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")

	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      "multicluster-observability-addon",
			},
		},
	}

	addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformLogsCollection,
					Value: string(addon.ClusterLogForwarderV1),
				},
			},
		},
	}

	fakeKubeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(addOnDeploymentConfig).
		Build()

	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.McoaChartDir).
		WithGetValuesFuncs(GetValuesFunc(context.TODO(), fakeKubeClient, "myhub.foo.com")).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Len(t, objects, 2)
}

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
			expectedErr: errMissingAODCRef,
		},
		{
			name: "Multiple AODC references",
			mcAddon: &addonapiv1alpha1.ManagedClusterAddOn{
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1alpha1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addon.AddonDeploymentConfigResource,
							},
							ConfigReferent: addonapiv1alpha1.ConfigReferent{
								Name:      "foo",
								Namespace: "foo",
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addon.AddonDeploymentConfigResource,
							},
							ConfigReferent: addonapiv1alpha1.ConfigReferent{
								Name:      "bar",
								Namespace: "bar",
							},
						},
					},
				},
			},
			expectedErr: errMultipleAODCRef,
		},
		{
			name: "AODC reference found",
			mcAddon: &addonapiv1alpha1.ManagedClusterAddOn{
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1alpha1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    addonutils.AddOnDeploymentConfigGVR.Group,
								Resource: addon.AddonDeploymentConfigResource,
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
			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build()

			// Call the function
			ctx := context.TODO()
			_, err := getAddOnDeploymentConfig(ctx, fakeClient, tt.mcAddon)

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
