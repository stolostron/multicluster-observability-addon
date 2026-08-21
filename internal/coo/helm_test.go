package coo

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/handlers"
	"github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = addonapiv1beta1.Install(scheme.Scheme)
	_ = uiplugin.AddToScheme(scheme.Scheme)
	_ = persesv1.AddToScheme(scheme.Scheme)
)

func fakeGetValues(ctx context.Context, k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1beta1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		aodc := &addonapiv1beta1.AddOnDeploymentConfig{}
		keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addoncfg.AddonDeploymentConfigResource)
		if err := k8s.Get(ctx, keys[0], aodc, &client.GetOptions{}); err != nil {
			return nil, err
		}
		addonOpts, err := addon.BuildOptions(aodc)
		if err != nil {
			return nil, err
		}

		isHub := false
		if cluster != nil {
			if val, ok := cluster.Labels["local-cluster"]; ok {
				isHub = val == "true"
			}
		}

		installCOO, err := handlers.InstallOfCOOOnTheHubIsNeeded(ctx, k8s, logr.Discard(), isHub)
		if err != nil {
			return nil, err
		}

		cooValues := manifests.BuildValues(addonOpts, installCOO, isHub, false)

		return addonfactory.JsonStructToValues(cooValues)
	}
}

func newCOOAgentAddon(initObjects []client.Object, addOnDeploymentConfig *addonapiv1beta1.AddOnDeploymentConfig) agent.AgentAddon {
	initObjects = append(initObjects, addOnDeploymentConfig)
	fakeKubeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(initObjects...).
		Build()

	getter := mockAODCGetter{addOnDeploymentConfig}
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		getter,
		addonfactory.ToAddOnCustomizedVariableValues,
	)
	ctx := context.Background()

	oboAgentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.COOChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(ctx, fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}
	return oboAgentAddon
}

// Test_COOHelmRendering tests that the COO Helm chart renders only COO
// subscription resources. Hub-only Perses resources (dashboards, datasources,
// UIPlugin) are now reconciled directly by HubResourceReconciler.
func Test_COOHelmRendering(t *testing.T) {
	for _, tc := range []struct {
		name         string
		isHub        bool
		cv           []addonapiv1beta1.CustomizedVariable
		expectedFunc func(*testing.T, []runtime.Object)
	}{
		{
			name: "no config produces no manifests",
			expectedFunc: func(t *testing.T, objects []runtime.Object) {
				require.Empty(t, objects)
			},
		},
		{
			name:  "hub with right-sizing enabled renders no Perses resources",
			isHub: true,
			cv: []addonapiv1beta1.CustomizedVariable{
				{Name: addon.KeyRightSizingDelegated, Value: "true"},
				{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
				{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
			},
			expectedFunc: func(t *testing.T, objects []runtime.Object) {
				for _, o := range objects {
					assert.IsNotType(t, &persesv1.PersesDashboard{}, o, "dashboards should not come from Helm")
					assert.IsNotType(t, &persesv1.PersesDatasource{}, o, "datasources should not come from Helm")
					assert.IsNotType(t, &uiplugin.UIPlugin{}, o, "UIPlugin should not come from Helm")
				}
			},
		},
		{
			name:  "hub with metrics UI renders no Perses resources from Helm",
			isHub: true,
			cv: []addonapiv1beta1.CustomizedVariable{
				{Name: "platformMetricsCollection", Value: "prometheusagents.v1alpha1.monitoring.rhobs"},
				{Name: addon.KeyMetricsHubHostname, Value: "metrics.hub.com"},
				{Name: "platformMetricsUI", Value: "uiplugins.v1alpha1.observability.openshift.io"},
			},
			expectedFunc: func(t *testing.T, objects []runtime.Object) {
				for _, o := range objects {
					assert.IsNotType(t, &persesv1.PersesDashboard{}, o, "dashboards should not come from Helm")
					assert.IsNotType(t, &persesv1.PersesDatasource{}, o, "datasources should not come from Helm")
					assert.IsNotType(t, &uiplugin.UIPlugin{}, o, "UIPlugin should not come from Helm")
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mc := addontesting.NewManagedCluster("cluster-1")
			if tc.isHub {
				mc.Labels = map[string]string{"local-cluster": "true"}
			}

			mcao := addontesting.NewAddon("test", "cluster-1")
			mcao.Status.ConfigReferences = []addonapiv1beta1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
						Group:    "addon.open-cluster-management.io",
						Resource: "addondeploymentconfigs",
					},
					DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
						ConfigReferent: addonapiv1beta1.ConfigReferent{
							Namespace: "open-cluster-management-observability",
							Name:      "multicluster-observability-addon",
						},
						SpecHash: "fake-spec-hash",
					},
				},
			}

			addc := &addonapiv1beta1.AddOnDeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicluster-observability-addon",
					Namespace: "open-cluster-management-observability",
				},
				Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
					CustomizedVariables: tc.cv,
				},
			}

			cooAgentAddon := newCOOAgentAddon([]client.Object{mcao}, addc)
			objects, err := cooAgentAddon.Manifests(t.Context(), mc, mcao)
			require.NoError(t, err)
			tc.expectedFunc(t, objects)
		})
	}
}

type mockAODCGetter struct {
	aodc *addonapiv1beta1.AddOnDeploymentConfig
}

func (m mockAODCGetter) Get(ctx context.Context, namespace, name string) (*addonapiv1beta1.AddOnDeploymentConfig, error) {
	return m.aodc, nil
}
