package coo

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/handlers"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = addonapiv1alpha1.AddToScheme(scheme.Scheme)
	_ = uiplugin.AddToScheme(scheme.Scheme)
)

func fakeGetValues(ctx context.Context, k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
		keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addon.AddonDeploymentConfigResource)
		if err := k8s.Get(ctx, keys[0], aodc, &client.GetOptions{}); err != nil {
			return nil, err
		}
		addonOpts, err := addon.BuildOptions(aodc)
		if err != nil {
			return nil, err
		}

		// Check if this is a hub cluster by looking for the local-cluster label
		isHub := false
		if cluster != nil {
			if val, ok := cluster.Labels["local-cluster"]; ok {
				isHub = val == "true"
			}
		}

		installCOO, err := handlers.InstallCOO(ctx, k8s, logr.Discard(), isHub)
		if err != nil {
			return nil, err
		}

		cooValues := manifests.BuildValues(addonOpts, installCOO, isHub)

		return addonfactory.JsonStructToValues(cooValues)
	}
}

func newCOOAgentAddon(initObjects []client.Object, addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig) agent.AgentAddon {
	initObjects = append(initObjects, addOnDeploymentConfig)
	// Setup the fake k8s client
	fakeKubeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(initObjects...).
		Build()

	fakeAddonClient := fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)
	ctx := context.Background()

	// Wire everything together to a fake addon instance
	oboAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.COOChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(ctx, fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}
	return oboAgentAddon
}

func Test_IncidentDetection_AllConfigsTogether_AllResources(t *testing.T) {
	for _, tc := range []struct {
		name         string
		isHub        bool
		cv           []addonapiv1alpha1.CustomizedVariable
		expectedFunc func(*testing.T, []runtime.Object)
	}{
		{
			name: "no config",
			expectedFunc: func(t *testing.T, objects []runtime.Object) {
				require.Equal(t, 0, len(objects))
			},
		},
		{
			name: "incident detection",
			cv: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "platformIncidentDetection",
					Value: "uiplugins.v1alpha1.observability.openshift.io",
				},
			},
			expectedFunc: func(t *testing.T, objects []runtime.Object) {
				require.Equal(t, 4, len(objects))
				expectedUIPluginSpec := uiplugin.UIPluginSpec{
					Type: "Monitoring",
					Monitoring: &uiplugin.MonitoringConfig{
						Incidents: &uiplugin.IncidentsReference{
							Enabled: true,
						},
					},
				}

				for _, o := range objects {
					switch o := o.(type) {
					case *uiplugin.UIPlugin:
						require.Equal(t, "monitoring", o.Name)
						require.Equal(t, expectedUIPluginSpec, o.Spec)
					}
				}
			},
		},
		{
			name:  "incident detection",
			isHub: true,
			cv: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "platformMetricsCollection",
					Value: "prometheusagents.v1alpha1.monitoring.coreos.com",
				},
				{
					Name:  "observabilityUIMetrics",
					Value: "uiplugins.v1alpha1.observability.openshift.io",
				},
			},
			expectedFunc: func(t *testing.T, objects []runtime.Object) {
				require.Equal(t, 4, len(objects))
				expectedUIPluginSpec := uiplugin.UIPluginSpec{
					Type: "Monitoring",
					Monitoring: &uiplugin.MonitoringConfig{
						ACM: &uiplugin.AdvancedClusterManagementReference{
							Enabled: true,
							Alertmanager: uiplugin.AlertmanagerReference{
								Url: "https://alertmanager.open-cluster-management-observability.svc:9095",
							},
							ThanosQuerier: uiplugin.ThanosQuerierReference{
								Url: "https://rbac-query-proxy.open-cluster-management-observability.svc:8443",
							},
						},
						Perses: &uiplugin.PersesReference{
							Enabled: true,
						},
					},
				}

				for _, o := range objects {
					switch o := o.(type) {
					case *uiplugin.UIPlugin:
						require.Equal(t, "monitoring", o.Name)
						require.Equal(t, expectedUIPluginSpec, o.Spec)
					}
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Setup a managed cluster
			mc := addontesting.NewManagedCluster("cluster-1")
			if tc.isHub {
				mc.Labels = map[string]string{
					"local-cluster": "true",
				}
			}

			// Register the addon for the managed cluster
			mcao := addontesting.NewAddon("test", "cluster-1")
			mcao.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "addon.open-cluster-management.io",
						Resource: "addondeploymentconfigs",
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Namespace: "open-cluster-management-observability",
						Name:      "multicluster-observability-addon",
					},
					DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
						ConfigReferent: addonapiv1alpha1.ConfigReferent{
							Namespace: "open-cluster-management-observability",
							Name:      "multicluster-observability-addon",
						},
						SpecHash: "fake-spec-hash",
					},
				},
			}

			addc := &addonapiv1alpha1.AddOnDeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicluster-observability-addon",
					Namespace: "open-cluster-management-observability",
				},
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					CustomizedVariables: tc.cv,
				},
			}

			// Create the COOAgentAddon
			cooAgentAddon := newCOOAgentAddon([]client.Object{mcao}, addc)

			// Render manifests and return them as k8s runtime objects
			objects, err := cooAgentAddon.Manifests(mc, mcao)
			require.NoError(t, err)
			tc.expectedFunc(t, objects)
		})
	}
}
