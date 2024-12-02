package metrics_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/handlers"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/manifests"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/resource"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHelmBuild_Metrics_All(t *testing.T) {
	testCases := map[string]struct {
		PlatformMetrics bool
		UserMetrics     bool
		Expects         func(*testing.T, []runtime.Object)
	}{
		"no metrics": {
			PlatformMetrics: false,
			UserMetrics:     false,
			Expects: func(t *testing.T, objects []runtime.Object) {
				assert.Len(t, objects, 0)
			},
		},
		"platform metrics": {
			PlatformMetrics: true,
			UserMetrics:     false,
			Expects: func(t *testing.T, objects []runtime.Object) {
				// ensure the agent is created
				agent := getResourceByLabelSelector[*prometheusalpha1.PrometheusAgent](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, agent, 1)
				assert.Equal(t, config.PlatformMetricsCollectorApp, agent[0].GetName())
				assert.NotEmpty(t, agent[0].Spec.CommonPrometheusFields.RemoteWrite[0].URL)
				// ensure that the haproxy config is created
				haProxyConfig := getResourceByLabelSelector[*corev1.ConfigMap](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, haProxyConfig, 1)
				// ensure that scrape config is created and matches the agent
				scrapeCfgs := getResourceByLabelSelector[*prometheusalpha1.ScrapeConfig](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, scrapeCfgs, 1)
				for k, v := range agent[0].Spec.ScrapeConfigSelector.MatchLabels {
					assert.Equal(t, v, scrapeCfgs[0].Labels[k])
				}
				assert.Equal(t, config.PrometheusControllerID, scrapeCfgs[0].Annotations["operator.prometheus.io/controller-id"])
				// ensure that recording rules are created
				recordingRules := getResourceByLabelSelector[*prometheusv1.PrometheusRule](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, recordingRules, 1)
				assert.Equal(t, "openshift-monitoring/prometheus-operator", recordingRules[0].Annotations["operator.prometheus.io/controller-id"])
				// ensure that the number of objects is correct
				// 4 (prom operator) + 6 (agent + haproxy config) + 2 secrets (mTLS to hub) + 1 cm (prom ca) + 1 rule + 1 scrape config = 15
				assert.Len(t, objects, 15)
				assert.Len(t, getResourceByLabelSelector[*corev1.Secret](objects, nil), 2) // 2 secrets (mTLS to hub)
			},
		},
		"user workload metrics": {
			PlatformMetrics: false,
			UserMetrics:     true,
			Expects: func(t *testing.T, objects []runtime.Object) {
				// ensure the agent is created
				agent := getResourceByLabelSelector[*prometheusalpha1.PrometheusAgent](objects, config.UserWorkloadPrometheusMatchLabels)
				assert.Len(t, agent, 1)
				assert.Equal(t, config.UserWorkloadMetricsCollectorApp, agent[0].GetName())
				assert.NotEmpty(t, agent[0].Spec.CommonPrometheusFields.RemoteWrite[0].URL)
				// ensure that the haproxy config is created
				haProxyConfig := getResourceByLabelSelector[*corev1.ConfigMap](objects, config.UserWorkloadPrometheusMatchLabels)
				assert.Len(t, haProxyConfig, 1)

				assert.Len(t, objects, 13)
				assert.Len(t, getResourceByLabelSelector[*corev1.Secret](objects, nil), 2) // 2 secrets (mTLS to hub)
			},
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	assert.NoError(t, prometheusalpha1.AddToScheme(scheme))
	assert.NoError(t, prometheusv1.AddToScheme(scheme))
	assert.NoError(t, clusterv1.AddToScheme(scheme))

	installNamespace := "open-cluster-management-addon-observability"
	hubNamespace := "open-cluster-management-observability"

	// Add platform resources
	defaultAgentResources := resource.DefaultPlaftformAgentResources(hubNamespace)

	// Add user workload resources
	defaultAgentResources = append(defaultAgentResources, resource.DefaultUserWorkloadAgentResources(hubNamespace)...)

	configReferences := []addonapiv1alpha1.ConfigReference{}
	for _, obj := range defaultAgentResources {
		resource := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind) + "s"
		configReferences = append(configReferences, addonapiv1alpha1.ConfigReference{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    obj.GetObjectKind().GroupVersionKind().Group,
				Resource: resource,
			},
			DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
				ConfigReferent: addonapiv1alpha1.ConfigReferent{
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
				},
			},
		})
	}

	clientObjects := []client.Object{}
	clientObjects = append(clientObjects, defaultAgentResources...)

	// Add secrets needed for the agent connection to the hub
	clientObjects = append(clientObjects, newSecret(config.HubCASecretName, hubNamespace))
	clientObjects = append(clientObjects, newSecret(config.ClientCertSecretName, hubNamespace))

	// Setup a managed cluster
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	clientObjects = append(clientObjects, managedCluster)

	// Images overrides configMap
	imagesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "images-list",
			Namespace: hubNamespace,
		},
		Data: map[string]string{
			"prometheus_operator":        "registry.redhat.io/rhacm2/acm-prometheus-rhel9@sha256:4234bab8666dad7917cfcf10fdaed87b60e549ef6f8fb23d1760881d922e03e9",
			"prometheus_config_reloader": "registry.redhat.io/rhacm2/acm-prometheus-config-reloader-rhel9@sha256:ab1632ec7aca478cf368e80ac9d98da3f2306a0cae8a4e9d29f95e149fd47ced",
			"kube_rbac_proxy":            "registry.redhat.io/rhacm2/kube-rbac-proxy-rhel9@sha256:c60a1d52359493a41b2b6f820d11716d67290e9b83dc18c16039dbc6f120e5f2",
		},
	}
	clientObjects = append(clientObjects, imagesCM)

	// Setup the fake k8s client
	client := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(clientObjects...).
		Build()

	// Setup the fake addon client
	addonClient := fakeaddon.NewSimpleClientset(newAddonDeploymentConfig())
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Register the addon for the managed cluster
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Spec.InstallNamespace = installNamespace
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{}
	managedClusterAddOn.Status.ConfigReferences = append(managedClusterAddOn.Status.ConfigReferences, configReferences...)

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Wire everything together to a fake addon instance
			agentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.MetricsChartDir).
				WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(client, tc.PlatformMetrics, tc.UserMetrics)).
				WithAgentRegistrationOption(&agent.RegistrationOption{}).
				WithScheme(scheme).
				BuildHelmAgentAddon()
			if err != nil {
				klog.Fatalf("failed to build agent %v", err)
			}

			// Render manifests and return them as k8s runtime objects
			objects, err := agentAddon.Manifests(managedCluster, managedClusterAddOn)
			assert.NoError(t, err)

			tc.Expects(t, objects)

			// Check common properties of the objects
			for _, obj := range objects {
				accessor, err := meta.Accessor(obj)
				assert.NoError(t, err)

				// if not a global object, check namespace
				if !slices.Contains([]string{"ClusterRole", "ClusterRoleBinding"}, obj.GetObjectKind().GroupVersionKind().Kind) {
					assert.Equal(t, installNamespace, accessor.GetNamespace(), fmt.Sprintf("Object: %s/%s", obj.GetObjectKind().GroupVersionKind(), accessor.GetName()))
				}
			}
		})
	}
}

func newAddonDeploymentConfig() *addonapiv1alpha1.AddOnDeploymentConfig {
	return &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management-observability",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "loggingSubscriptionChannel",
					Value: "stable-5.9",
				},
			},
		},
	}
}

func newSecret(name, ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}
}

func fakeGetValues(k8s client.Client, platformMetrics, userWorkloadMetrics bool) addonfactory.GetValuesFunc {
	return func(
		_ *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		optionsBuilder := handlers.OptionsBuilder{
			Client: k8s,
			ImagesConfigMap: types.NamespacedName{
				Name:      "images-list",
				Namespace: "open-cluster-management-observability",
			},
			RemoteWriteURL: "https://observatorium-api-open-cluster-management-observability.apps.sno-4xlarge-416-lqsr2.dev07.red-chesterfield.com/api/metrics/v1/default/api/v1/receive",
		}

		opts, err := optionsBuilder.Build(context.Background(), mcAddon, addon.MetricsOptions{CollectionEnabled: platformMetrics}, addon.MetricsOptions{CollectionEnabled: userWorkloadMetrics})
		if err != nil {
			return nil, err
		}

		helmValues, err := manifests.BuildValues(opts)
		if err != nil {
			return nil, err
		}

		return addonfactory.JsonStructToValues(helmValues)
	}
}

// getResourceByLabelSelector returns the first resource that matches the label selector.
// It works generically for any Kubernetes resource that implements client.Object.
func getResourceByLabelSelector[T client.Object](resources []runtime.Object, selector map[string]string) []T {
	labelSelector := labels.SelectorFromSet(selector)
	ret := []T{}

	for _, obj := range resources {
		if resource, ok := obj.(T); ok {
			if labelSelector.Matches(labels.Set(resource.GetLabels())) {
				ret = append(ret, resource)
			}
		}
	}

	return ret
}
