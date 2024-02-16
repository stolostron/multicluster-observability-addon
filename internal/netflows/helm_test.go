package netflows

import (
	"testing"

	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/netflows/handlers"
	"github.com/rhobs/multicluster-observability-addon/internal/netflows/manifests"

	nfv1beta2 "github.com/netobserv/network-observability-operator/apis/flowcollector/v1beta2"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
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
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	_ = nfv1beta2.AddToScheme(scheme.Scheme)
)

func fakeGetValues(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		addon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		opts, err := handlers.BuildOptions(k8s, addon, nil)
		if err != nil {
			return nil, err
		}

		nf, err := manifests.BuildValues(opts)
		if err != nil {
			return nil, err
		}

		return addonfactory.JsonStructToValues(nf)
	}
}

func Test_Netflows_AllConfigsTogether_AllResources(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedCluster      *clusterv1.ManagedCluster
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
		fc                    *nfv1beta2.FlowCollector
		authCM                *corev1.ConfigMap
		staticCred            *corev1.Secret

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
				Namespace: "open-cluster-management",
				Name:      "multicluster-observability-addon",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "flows.netobserv.io",
				Resource: "flowcollectors",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Name: "cluster",
			},
		},
	}

	// Setup configuration resources: ClusterLogForwarder, AddOnDeploymentConfig, Secrets, ConfigMaps
	fc = &nfv1beta2.FlowCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: nfv1beta2.FlowCollectorSpec{
			Namespace:       "netobserv",
			DeploymentModel: nfv1beta2.DeploymentModelDirect,
			Agent: nfv1beta2.FlowCollectorAgent{
				Type: nfv1beta2.AgentEBPF,
				EBPF: nfv1beta2.FlowCollectorEBPF{
					Sampling: ptr.To(int32(1)),
				},
			},
			Loki: nfv1beta2.FlowCollectorLoki{
				Enable: ptr.To(false),
			},
		},
	}

	staticCred = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "static-authentication",
			Namespace: "open-cluster-management",
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}

	authCM = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "logging-auth",
			Namespace: "open-cluster-management",
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "logging",
			},
		},
		Data: map[string]string{
			"app-logs":     "StaticAuthentication",
			"cluster-logs": "StaticAuthentication",
		},
	}

	addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "loggingSubscriptionChannel",
					Value: "stable-5.8",
				},
			},
		},
	}

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(fc, staticCred, authCM).
		Build()

	// Setup the fake addon client
	fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.NetflowChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	// Render manifests and return them as k8s runtime objects
	objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Len(t, objects, 4)

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *operatorsv1alpha1.Subscription:
			require.Equal(t, obj.Spec.Channel, "stable")
		case *nfv1beta2.FlowCollector:
			require.Equal(t, nfv1beta2.DeploymentModelDirect, obj.Spec.DeploymentModel)
			require.NotNil(t, obj.Spec.Loki.Enable)
			require.False(t, *obj.Spec.Loki.Enable)
			require.NotNil(t, obj.Spec.Agent.EBPF.Sampling)
			require.Equal(t, 1, int(*obj.Spec.Agent.EBPF.Sampling))
		}
	}
}
