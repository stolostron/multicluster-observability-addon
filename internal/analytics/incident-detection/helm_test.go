package incidentdetection

import (
	"context"
	"testing"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/handlers"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/incident-detection/manifests"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
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
		_ *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
		keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addoncfg.AddonDeploymentConfigResource)
		if err := k8s.Get(ctx, keys[0], aodc, &client.GetOptions{}); err != nil {
			return nil, err
		}
		addonOpts, err := addon.BuildOptions(aodc)
		if err != nil {
			return nil, err
		}

		opts := handlers.BuildOptions(ctx, k8s, mcAddon, addonOpts.Platform.AnalyticsOptions.IncidentDetection)
		analyticsValues := &analytics.AnalyticsValues{
			IncidentDetectionValues: manifests.BuildValues(opts),
		}

		return addonfactory.JsonStructToValues(analyticsValues)
	}
}

func Test_IncidentDetection_AllConfigsTogether_AllResources(t *testing.T) {
	const oboNamespace = "openshift-cluster-observability-operator"
	var (
		// Addon envinronment and registration
		managedCluster      *clusterv1.ManagedCluster
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig

		// Test clients
		fakeKubeClient  client.Client
		fakeAddonClient *fakeaddon.Clientset
	)

	// Setup a managed cluster
	managedCluster = addontesting.NewManagedCluster("cluster-1")

	// Register the addon for the managed cluster
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
			DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
				ConfigReferent: addonapiv1alpha1.ConfigReferent{
					Namespace: "open-cluster-management-observability",
					Name:      "multicluster-observability-addon",
				},
				SpecHash: "fake-spec-hash",
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
					Name:  "observabilityOperator",
					Value: "monitoring.rhobs",
				},
			},
		},
	}

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(addOnDeploymentConfig).
		Build()

	fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)
	ctx := context.Background()

	// Wire everything together to a fake addon instance
	oboAgentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.IncidentDetectionChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(ctx, fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	require.NoError(t, err)

	// Render manifests and return them as k8s runtime objects
	objects, err := oboAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Equal(t, 1, len(objects))

	for _, o := range objects {
		switch o := o.(type) {
		case *operatorsv1alpha1.Subscription:
			require.Equal(t, "cluster-observability-operator", o.Name)
			require.Equal(t, oboNamespace, o.Namespace)
			require.Equal(t, "stable", o.Spec.Channel)
			_, ok := o.Labels["operators.coreos.com/cluster-observability-operator.openshift-cluster-observability"]
			require.True(t, ok)
		case *operatorsv1.OperatorGroup:
			require.Equal(t, oboNamespace, o.Name)
			require.Equal(t, oboNamespace, o.Namespace)
		case *corev1.Namespace:
			require.Equal(t, oboNamespace, o.Name)
		case *uiplugin.UIPlugin:
			require.Equal(t, "monitoring", o.Name)
		}
	}
}
