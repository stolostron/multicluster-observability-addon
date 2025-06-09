package tracing

import (
	"context"
	"testing"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/tracing/handlers"
	"github.com/stolostron/multicluster-observability-addon/internal/tracing/manifests"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	_ = otelv1beta1.AddToScheme(scheme.Scheme)
	_ = otelv1alpha1.AddToScheme(scheme.Scheme)
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
)

func fakeGetValues(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		opts, err := handlers.BuildOptions(context.TODO(), k8s, mcAddon, addon.TracesOptions{InstrumentationEnabled: true})
		if err != nil {
			return nil, err
		}

		tracing, err := manifests.BuildValues(opts)
		if err != nil {
			return nil, err
		}

		return addonfactory.JsonStructToValues(tracing)
	}
}

func Test_Tracing_AllConfigsTogether_AllResources(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedCluster      *clusterv1.ManagedCluster
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
		authCM                *corev1.ConfigMap

		// Test clients
		fakeKubeClient  client.Client
		fakeAddonClient *fakeaddon.Clientset
	)

	// Setup a managed cluster
	managedCluster = addontesting.NewManagedCluster("cluster-1")

	// Register the addon for the managed cluster
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Spec.Configs = []addonapiv1alpha1.AddOnConfig{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "configmaps",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      "tracing-auth",
			},
		},
	}
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
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "opentelemetry.io",
				Resource: "opentelemetrycollectors",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      "mcoa-instance",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "opentelemetry.io",
				Resource: "instrumentations",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management-observability",
				Name:      "mcoa-instance",
			},
		},
	}

	// Setup configuration resources: OpenTelemetryCollector, AddOnDeploymentConfig
	otelCol := otelv1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-instance",
			Namespace: "open-cluster-management-observability",
		},
		Spec: otelv1beta1.OpenTelemetryCollectorSpec{
			Config: otelv1beta1.Config{
				Exporters: otelv1beta1.AnyConfig{
					Object: map[string]interface{}{
						"otlp": map[string]interface{}{
							"protocols": map[string]interface{}{
								"otlp":     map[string]interface{}{},
								"otlphttp": map[string]interface{}{},
							},
						},
					},
				},
			},
		},
	}

	instr := otelv1alpha1.Instrumentation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcoa-instance",
			Namespace: "open-cluster-management-observability",
		},
		Spec: otelv1alpha1.InstrumentationSpec{
			Exporter: otelv1alpha1.Exporter{
				Endpoint: "http://test.com",
			},
		},
	}

	addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{},
	}

	authCM = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-auth",
			Namespace: "open-cluster-management-observability",
		},
		Data: map[string]string{
			"otlphttp": "mTLS",
		},
	}

	// This secret will be generated by cert-manager. We need to create it here
	// to mock the behavior from cert-manager
	generatedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-otlphttp-auth",
			Namespace: "cluster-1",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("data"),
			"ca.crt":  []byte("data"),
			"tls.key": []byte("data"),
		},
	}

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(&otelCol, &instr, authCM, generatedSecret).
		Build()

	// Setup the fake addon client
	fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	tracingAgentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.TracingChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	require.NoError(t, err)

	// Render manifests and return them as k8s runtime objects
	objects, err := tracingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Equal(t, 6, len(objects))

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *otelv1beta1.OpenTelemetryCollector:
			// Check name and namespace to make sure that if we change the helm
			// manifests that we don't break the addon probes
			require.Equal(t, addoncfg.SpokeOTELColName, obj.Name)
			require.Equal(t, addoncfg.SpokeOTELColNamespace, obj.Namespace)
			require.NotEmpty(t, obj.Spec.Config)
		case *otelv1alpha1.Instrumentation:
			require.Equal(t, addoncfg.SpokeInstrumentationName, obj.Name)
			require.Equal(t, addoncfg.SpokeOTELColNamespace, obj.Namespace)
		case *corev1.Secret:
			if obj.Name == "tracing-otlphttp-auth" {
				require.Equal(t, generatedSecret.Data, obj.Data)
			}
		}
	}
}
