package metrics_test

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	clusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/handlers"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/manifests"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestHelmBuild_Metrics_All(t *testing.T) {
	testCases := map[string]struct {
		PlatformMetrics  bool
		UserMetrics      bool
		COOIsInstalled   bool
		IsOCP            bool
		InstallNamespace string
		Expects          func(*testing.T, []client.Object)
	}{
		"no metrics": {
			PlatformMetrics: false,
			UserMetrics:     false,
			IsOCP:           true,
			Expects: func(t *testing.T, objects []client.Object) {
				assert.Len(t, objects, 0)
			},
		},
		"platform metrics, no coo": {
			PlatformMetrics: true,
			UserMetrics:     false,
			COOIsInstalled:  false,
			IsOCP:           true,
			Expects: func(t *testing.T, objects []client.Object) {
				// ensure the agent is created
				agent := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.PrometheusAgent](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, agent, 1)
				assert.Equal(t, config.PlatformMetricsCollectorApp, agent[0].GetName())
				assert.NotEmpty(t, agent[0].Spec.CommonPrometheusFields.RemoteWrite[0].URL)
				// ensure that scrape config is created and matches the agent
				scrapeCfgs := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, scrapeCfgs, 2)
				assert.Equal(t, config.PrometheusControllerID, scrapeCfgs[0].Annotations["operator.prometheus.io/controller-id"])
				assert.GreaterOrEqual(t, len(agent[0].Spec.ScrapeConfigSelector.MatchLabels), 0)
				scrapeConfigsSelector := labels.SelectorFromSet(labels.Set(agent[0].Spec.ScrapeConfigSelector.MatchLabels))
				assert.True(t, scrapeConfigsSelector.Matches(labels.Set(scrapeCfgs[0].Labels)))
				// ensure that recording rules are created
				recordingRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, recordingRules, 2)
				assert.Equal(t, "openshift-monitoring/prometheus-operator", recordingRules[0].Annotations["operator.prometheus.io/controller-id"])
				// Ensure the COO Prometheus operator is generated
				cooOperator := common.FilterResourcesByLabelSelector[*appsv1.Deployment](objects, nil)
				assert.Len(t, cooOperator, 1)
				// ensure that the number of objects is correct
				// 4 (prom operator) + 5 (agent) + 2 secrets (mTLS to hub) + 1 cm (prom ca) + 2 rule + 2 scrape config = 16
				expectedCount := 33
				if len(objects) != expectedCount {
					t.Fatalf("expected %d objects, but got %d:\n%s", expectedCount, len(objects), formatObjects(objects))
				}
				assert.Len(t, common.FilterResourcesByLabelSelector[*corev1.Secret](objects, nil), 4) // 6 secrets (mTLS to hub) + alertmananger secrets (accessor+ca in platform)
			},
		},
		"platform metrics, coo is installed": {
			PlatformMetrics: true,
			UserMetrics:     false,
			COOIsInstalled:  true,
			IsOCP:           true,
			Expects: func(t *testing.T, objects []client.Object) {
				// ensure the agent is created and gets the label expected by the OLM installed COO operator
				agent := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.PrometheusAgent](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, agent, 1)
				assert.Equal(t, "observability-operator", agent[0].Labels["app.kubernetes.io/managed-by"])
				assert.Empty(t, agent[0].Annotations["operator.prometheus.io/controller-id"])
				// ensure that the number of objects is correct
				expectedCount := 25
				if len(objects) != expectedCount {
					t.Fatalf("expected %d objects, but got %d:\n%s", expectedCount, len(objects), formatObjects(objects))
				}
			},
		},
		"user workload metrics": {
			PlatformMetrics: false,
			UserMetrics:     true,
			IsOCP:           true,
			Expects: func(t *testing.T, objects []client.Object) {
				// ensure the agent is created
				agent := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.PrometheusAgent](objects, config.UserWorkloadPrometheusMatchLabels)
				assert.Len(t, agent, 1)
				assert.Equal(t, config.UserWorkloadMetricsCollectorApp, agent[0].GetName())
				assert.NotEmpty(t, agent[0].Spec.CommonPrometheusFields.RemoteWrite[0].URL)
				// ensure that scrape config is created and matches the agent
				scrapeCfgs := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](objects, config.UserWorkloadPrometheusMatchLabels)
				assert.Len(t, scrapeCfgs, 2)
				assert.Equal(t, config.PrometheusControllerID, scrapeCfgs[0].Annotations["operator.prometheus.io/controller-id"])
				assert.GreaterOrEqual(t, len(agent[0].Spec.ScrapeConfigSelector.MatchLabels), 0)
				scrapeConfigsSelector := labels.SelectorFromSet(labels.Set(agent[0].Spec.ScrapeConfigSelector.MatchLabels))
				assert.True(t, scrapeConfigsSelector.Matches(labels.Set(scrapeCfgs[0].Labels)))
				// ensure that recording rules are created
				recordingRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](objects, config.UserWorkloadPrometheusMatchLabels)
				assert.Len(t, recordingRules, 2)
				assert.Equal(t, "openshift-user-workload-monitoring/prometheus-operator", recordingRules[0].Annotations["operator.prometheus.io/controller-id"])
				expectedCount := 33
				if len(objects) != expectedCount {
					t.Fatalf("expected %d objects, but got %d:\n%s", expectedCount, len(objects), formatObjects(objects))
				}
				assert.Len(t, common.FilterResourcesByLabelSelector[*corev1.Secret](objects, nil), 4) // 2 secrets (mTLS to hub) + alertmananger secrets (accessor+ca in uwl)
			},
		},
		"user workload, coo is installed": {
			PlatformMetrics: false,
			UserMetrics:     true,
			COOIsInstalled:  true,
			IsOCP:           true,
			Expects: func(t *testing.T, objects []client.Object) {
				// ensure the agent is created and gets the label expected by the OLM installed COO operator
				agent := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.PrometheusAgent](objects, config.UserWorkloadPrometheusMatchLabels)
				assert.Len(t, agent, 1)
				assert.Equal(t, "observability-operator", agent[0].Labels["app.kubernetes.io/managed-by"])
				assert.Empty(t, agent[0].Annotations["operator.prometheus.io/controller-id"])
				// ensure that the number of objects is correct
				expectedCount := 25
				if len(objects) != expectedCount {
					t.Fatalf("expected %d objects, but got %d:\n%s", expectedCount, len(objects), formatObjects(objects))
				}
			},
		},
		"is non ocp": {
			PlatformMetrics: true,
			UserMetrics:     false,
			COOIsInstalled:  false,
			IsOCP:           false,
			Expects: func(t *testing.T, objects []client.Object) {
				// ensure the agent is created
				agent := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.PrometheusAgent](objects, config.PlatformPrometheusMatchLabels)
				assert.Len(t, agent, 1)
				assert.Equal(t, config.PlatformMetricsCollectorApp, agent[0].GetName())
				assert.NotEmpty(t, agent[0].Spec.CommonPrometheusFields.RemoteWrite[0].URL)
				// ensure that the number of objects is correct
				expectedCount := 69
				if len(objects) != expectedCount {
					t.Fatalf("expected %d objects, but got %d:\n%s", expectedCount, len(objects), formatObjects(objects))
				}
			},
		},
		"custom namespace": {
			PlatformMetrics:  true,
			UserMetrics:      false,
			COOIsInstalled:   false,
			IsOCP:            false,
			InstallNamespace: "custom",
			Expects: func(t *testing.T, objects []client.Object) {
				// ensure the namespace is created
				ns := common.FilterResourcesByLabelSelector[*corev1.Namespace](objects, nil)
				assert.Len(t, ns, 1)
				assert.Equal(t, "custom", ns[0].Name)
				// ensure that the number of objects is correct
				expectedCount := 70
				if len(objects) != expectedCount {
					t.Fatalf("expected %d objects, but got %d:\n%s", expectedCount, len(objects), formatObjects(objects))
				}
			},
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	assert.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	assert.NoError(t, prometheusv1.AddToScheme(scheme))
	assert.NoError(t, cooprometheusv1.AddToScheme(scheme))
	assert.NoError(t, clusterv1.AddToScheme(scheme))
	assert.NoError(t, addonapiv1alpha1.AddToScheme(scheme))
	assert.NoError(t, workv1.AddToScheme(scheme))

	hubNamespace := "open-cluster-management-observability"

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if tc.InstallNamespace == "" {
				tc.InstallNamespace = addonfactory.AddonDefaultInstallNamespace
			}
			// Add platform resources
			defaultAgentResources := []client.Object{}
			platformScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
				TypeMeta: metav1.TypeMeta{
					Kind:       cooprometheusv1alpha1.ScrapeConfigsKind,
					APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "platform",
					Namespace: hubNamespace,
					Labels:    config.PlatformPrometheusMatchLabels,
				},
				Spec: cooprometheusv1alpha1.ScrapeConfigSpec{},
			}
			platformScrapeConfigAdditional := platformScrapeConfig.DeepCopy() // Checks that the helm loop is well set
			platformScrapeConfigAdditional.Name = platformScrapeConfigAdditional.Name + "- additional"
			defaultAgentResources = append(defaultAgentResources, platformScrapeConfig, platformScrapeConfigAdditional)
			platformRules := &prometheusv1.PrometheusRule{
				TypeMeta: metav1.TypeMeta{
					Kind:       prometheusv1.PrometheusRuleKind,
					APIVersion: prometheusv1.SchemeGroupVersion.Identifier(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "platform",
					Namespace: hubNamespace,
					Labels:    config.PlatformPrometheusMatchLabels,
				},
				Spec: prometheusv1.PrometheusRuleSpec{},
			}
			platformRulesAdditional := platformRules.DeepCopy() // Checks that the helm loop is well set
			platformRulesAdditional.Name = platformRulesAdditional.Name + "-additional"
			defaultAgentResources = append(defaultAgentResources, platformRules, platformRulesAdditional)

			// Add user workload resources
			configReferences := []addonapiv1alpha1.ConfigReference{}
			for _, obj := range defaultAgentResources {
				configReferences = append(configReferences, newConfigReference(obj))
			}

			clientObjects := []client.Object{}
			clientObjects = append(clientObjects, defaultAgentResources...)

			// Add uwl scrape configs
			uwlScrapeConfig := platformScrapeConfig.DeepCopy()
			uwlScrapeConfig.Name = "uwl-scrape-config"
			uwlScrapeConfig.Labels = config.UserWorkloadPrometheusMatchLabels
			uwlScrapeConfigAdditional := uwlScrapeConfig.DeepCopy() // Checks that the helm loop is well set
			uwlScrapeConfigAdditional.Name = "uwl-scrape-config-additional"
			configReferences = append(configReferences, newConfigReference(uwlScrapeConfig), newConfigReference(uwlScrapeConfigAdditional))
			clientObjects = append(clientObjects, uwlScrapeConfig, uwlScrapeConfigAdditional)

			// Add uwl rules
			uwlRules := platformRules.DeepCopy()
			uwlRules.Name = "uwl-rules"
			uwlRules.Labels = config.UserWorkloadPrometheusMatchLabels
			uwlRulesAdditional := uwlRules.DeepCopy()
			uwlRulesAdditional.Name = "uwl-rules-additional"
			configReferences = append(configReferences, newConfigReference(uwlRules), newConfigReference(uwlRulesAdditional))
			clientObjects = append(clientObjects, uwlRules, uwlRulesAdditional)

			// Add secrets needed for the agent connection to the hub
			clientObjects = append(clientObjects, newSecret(config.HubCASecretName, hubNamespace))
			clientObjects = append(clientObjects, newSecret(config.ClientCertSecretName, hubNamespace))

			// Add alermanager secrets
			clientObjects = append(clientObjects, newSecret(config.AlertmanagerAccessorSecretName, hubNamespace))
			routerCertsSecret := newSecret(config.RouterDefaultCertsConfigMapObjKey.Name, config.RouterDefaultCertsConfigMapObjKey.Namespace)
			routerCertsSecret.Data["tls.crt"] = []byte("toto")
			clientObjects = append(clientObjects, routerCertsSecret)

			// Setup a managed cluster
			managedCluster := addontesting.NewManagedCluster("cluster-1")
			managedCluster.Labels = map[string]string{}
			if tc.IsOCP {
				managedCluster.Labels[clusterinfov1beta1.LabelKubeVendor] = string(clusterinfov1beta1.KubeVendorOpenShift)
			}
			clientObjects = append(clientObjects, managedCluster)

			// Setup the ClusterManagementAddon (needed to generate default resources)
			cmao := newCMOA()
			clientObjects = append(clientObjects, cmao)

			// Images overrides configMap
			imagesCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ImagesConfigMapObjKey.Name,
					Namespace: config.ImagesConfigMapObjKey.Namespace,
				},
				Data: map[string]string{
					"obo_prometheus_rhel9_operator": "quay.io/prometheus/obo-operator",
					"prometheus_config_reloader":    "quay.io/prometheus/config-reloader",
					"kube_rbac_proxy":               "quay.io/kube/rbac-proxy",
					"kube_state_metrics":            "quay.io/kube/kube-state-metrics",
					"node_exporter":                 "quay.io/kube/node-exporter",
					"prometheus":                    "quay.io/prometheus/prometheus",
				},
			}
			clientObjects = append(clientObjects, imagesCM)
			clientObjects = append(clientObjects, newManifestWork("cluster-1", tc.COOIsInstalled))

			// Setup the fake k8s client
			client := fakeclient.NewClientBuilder().
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return clientww.Patch(ctx, obj, client.Merge, opts...)
					},
				}).
				WithScheme(scheme).
				WithObjects(clientObjects...).
				Build()

			// Setup the fake addon client
			addonClient := fakeaddon.NewSimpleClientset(newAddonDeploymentConfig())
			addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
				addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
				addonfactory.ToAddOnCustomizedVariableValues,
			)

			// generate default agent resources
			defaultStack := resource.DefaultStackResources{
				Client:       client,
				CMAO:         cmao,
				AddonOptions: newAddonOptions(true, true),
				Logger:       klog.Background(),
			}

			dc, err := defaultStack.Reconcile(context.Background())
			require.NoError(t, err)
			err = common.EnsureAddonConfig(context.Background(), klog.Background(), client, dc)
			require.NoError(t, err)

			promAgents := cooprometheusv1alpha1.PrometheusAgentList{}
			err = client.List(context.Background(), &promAgents)
			require.NoError(t, err)
			require.Len(t, promAgents.Items, 2)
			configReferences = append(configReferences, newConfigReference(&promAgents.Items[0]), newConfigReference(&promAgents.Items[1]))

			// Register the addon for the managed cluster
			managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
			managedClusterAddOn.Spec.InstallNamespace = tc.InstallNamespace
			managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{}
			managedClusterAddOn.Status.ConfigReferences = append(managedClusterAddOn.Status.ConfigReferences, configReferences...)

			// Wire everything together to a fake addon instance
			agentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.MetricsChartDir).
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
			clientObjs := runtimeToClientObjects(t, objects)

			tc.Expects(t, clientObjs)

			// Check common properties of the objects
			for _, obj := range objects {
				accessor, err := meta.Accessor(obj)
				assert.NoError(t, err)

				// if not a global object, check namespace
				// secrets are possible to install in multiple namespaces (such as openshift-monitoring)
				// and are therefore also ignored.
				if !slices.Contains([]string{"ClusterRole", "ClusterRoleBinding", "CustomResourceDefinition", "Secret", "Namespace"}, obj.GetObjectKind().GroupVersionKind().Kind) {
					assert.Equal(t, tc.InstallNamespace, accessor.GetNamespace(), fmt.Sprintf("Object: %s/%s", obj.GetObjectKind().GroupVersionKind(), accessor.GetName()))
				}
			}
		})
	}
}

func TestHelmBuild_Metrics_HCP(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	assert.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	assert.NoError(t, prometheusv1.AddToScheme(scheme))
	assert.NoError(t, cooprometheusv1.AddToScheme(scheme))
	assert.NoError(t, clusterv1.AddToScheme(scheme))
	assert.NoError(t, hyperv1.AddToScheme(scheme))
	assert.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	installNamespace := "open-cluster-management-addon-observability"
	hubNamespace := "open-cluster-management-observability"

	// Add user workload resources
	defaultAgentResources := []client.Object{}

	configReferences := []addonapiv1alpha1.ConfigReference{}
	for _, obj := range defaultAgentResources {
		configReferences = append(configReferences, newConfigReference(obj))
	}

	clientObjects := []client.Object{}
	clientObjects = append(clientObjects, defaultAgentResources...)

	// Add hcp scrape configs and rules
	etcdHcpScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       cooprometheusv1alpha1.ScrapeConfigsKind,
			APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-base",
			Namespace: hubNamespace,
			Labels:    config.EtcdHcpUserWorkloadPrometheusMatchLabels,
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					`{__name__="etcd_metric"}`,
				},
			},
		},
	}
	apiserverHcpScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       cooprometheusv1alpha1.ScrapeConfigsKind,
			APIVersion: cooprometheusv1alpha1.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "apiserver-base",
			Namespace: hubNamespace,
			Labels:    config.ApiserverHcpUserWorkloadPrometheusMatchLabels,
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			Params: map[string][]string{
				"match[]": {
					`{__name__="apiserver_metric"}`,
				},
			},
		},
	}
	etcdHcpRule := &prometheusv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       prometheusv1.PrometheusRuleKind,
			APIVersion: prometheusv1.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-base",
			Namespace: hubNamespace,
			Labels:    config.EtcdHcpUserWorkloadPrometheusMatchLabels,
		},
		Spec: prometheusv1.PrometheusRuleSpec{
			Groups: []prometheusv1.RuleGroup{
				{
					Rules: []prometheusv1.Rule{
						{
							Expr: intstr.FromString("sum(etcd_rule_dependent_metric)"),
						},
					},
				},
			},
		},
	}
	apiserverHcpRule := &prometheusv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind:       prometheusv1.PrometheusRuleKind,
			APIVersion: prometheusv1.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "apiserver-base",
			Namespace: hubNamespace,
			Labels:    config.ApiserverHcpUserWorkloadPrometheusMatchLabels,
		},
		Spec: prometheusv1.PrometheusRuleSpec{
			Groups: []prometheusv1.RuleGroup{
				{
					Rules: []prometheusv1.Rule{
						{
							Expr: intstr.FromString("apiserver_rule_dependent_metric"),
						},
					},
				},
			},
		},
	}

	configReferences = append(configReferences, newConfigReference(etcdHcpScrapeConfig), newConfigReference(apiserverHcpScrapeConfig), newConfigReference(etcdHcpRule), newConfigReference(apiserverHcpRule))
	clientObjects = append(clientObjects, etcdHcpScrapeConfig, apiserverHcpScrapeConfig, etcdHcpRule, apiserverHcpRule)

	// Add hypershift dependencies
	hostedCluster := &hyperv1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "clusters",
		},
		Spec: hyperv1.HostedClusterSpec{
			ClusterID: "cluster-id-a",
		},
	}
	etcdSM := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd",
			Namespace: "clusters-a",
		},
		Spec: prometheusv1.ServiceMonitorSpec{
			Endpoints: []prometheusv1.Endpoint{
				{
					Port: "metrics",
				},
			},
		},
	}
	apiserverSM := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: "clusters-a",
		},
		Spec: prometheusv1.ServiceMonitorSpec{
			Endpoints: []prometheusv1.Endpoint{
				{
					Port: "client",
				},
			},
		},
	}
	clientObjects = append(clientObjects, hostedCluster, etcdSM, apiserverSM)

	// Add secrets needed for the agent connection to the hub
	clientObjects = append(clientObjects, newSecret(config.HubCASecretName, hubNamespace))
	clientObjects = append(clientObjects, newSecret(config.ClientCertSecretName, hubNamespace))

	// Add alermanager secrets
	clientObjects = append(clientObjects, newSecret(config.AlertmanagerAccessorSecretName, hubNamespace))
	routerCertsSecret := newSecret(config.RouterDefaultCertsConfigMapObjKey.Name, config.RouterDefaultCertsConfigMapObjKey.Namespace)
	routerCertsSecret.Data["tls.crt"] = []byte("toto")
	clientObjects = append(clientObjects, routerCertsSecret)

	// Setup a the local cluster as managed cluster
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedCluster.Labels = map[string]string{
		config.LocalManagedClusterLabel:  "true",
		config.HypershiftAddonStateLabel: "available",
	}
	clientObjects = append(clientObjects, managedCluster)

	// Images overrides configMap
	imagesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "images-list",
			Namespace: hubNamespace,
		},
		Data: map[string]string{
			"obo_prometheus_rhel9_operator": "quay.io/prometheus/obo-operator",
			"prometheus_config_reloader":    "quay.io/prometheus/config-reloader",
			"kube_rbac_proxy":               "quay.io/kube/rbac-proxy",
			"kube_state_metrics":            "quay.io/kube/kube-state-metrics",
			"node_exporter":                 "quay.io/kube/node-exporter",
			"prometheus":                    "quay.io/prometheus/prometheus",
		},
	}
	clientObjects = append(clientObjects, imagesCM)

	cmao := newCMOA()
	clientObjects = append(clientObjects, cmao)

	// Setup the fake k8s client
	client := fakeclient.NewClientBuilder().
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				return clientww.Patch(ctx, obj, client.Merge, opts...)
			},
		}).
		WithScheme(scheme).
		WithObjects(clientObjects...).
		Build()

	// Setup the fake addon client
	addonClient := fakeaddon.NewSimpleClientset(newAddonDeploymentConfig())
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// generate default agent resources
	defaultStack := resource.DefaultStackResources{
		Client:       client,
		CMAO:         cmao,
		AddonOptions: newAddonOptions(true, true),
		Logger:       klog.Background(),
	}
	dc, err := defaultStack.Reconcile(context.Background())
	require.NoError(t, err)
	err = common.EnsureAddonConfig(context.Background(), klog.Background(), client, dc)
	require.NoError(t, err)

	promAgents := cooprometheusv1alpha1.PrometheusAgentList{}
	err = client.List(context.Background(), &promAgents)
	require.NoError(t, err)
	require.Len(t, promAgents.Items, 2)
	configReferences = append(configReferences, newConfigReference(&promAgents.Items[0]), newConfigReference(&promAgents.Items[1]))

	// Register the addon for the managed cluster
	managedClusterAddOn := addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Spec.InstallNamespace = installNamespace
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{}
	managedClusterAddOn.Status.ConfigReferences = append(managedClusterAddOn.Status.ConfigReferences, configReferences...)

	// Wire everything together to a fake addon instance
	agentAddon, err := addonfactory.NewAgentAddonFactory(addoncfg.Name, addon.FS, addoncfg.MetricsChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(client, false, true)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	// Render manifests and return them as k8s runtime objects
	objects, err := agentAddon.Manifests(managedCluster, managedClusterAddOn)
	assert.NoError(t, err)
	clientObjs := runtimeToClientObjects(t, objects)

	recordingRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](clientObjs, nil)
	assert.Len(t, recordingRules, 2)
	scrapeConfigs := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](clientObjs, nil)
	assert.Len(t, scrapeConfigs, 2)
	serviceMonitors := common.FilterResourcesByLabelSelector[*prometheusv1.ServiceMonitor](clientObjs, nil)
	assert.Len(t, serviceMonitors, 3) // 2 for hcps and 1 for meta monitoring
	// keep only hcps serviceMonitors
	serviceMonitors = slices.DeleteFunc(serviceMonitors, func(e *prometheusv1.ServiceMonitor) bool { return e.Namespace != "clusters-a" })
	assert.Len(t, serviceMonitors, 2)
	assert.Len(t, serviceMonitors[0].Spec.Endpoints, 1)
	assert.Len(t, serviceMonitors[1].Spec.Endpoints, 1)
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
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		optionsBuilder := handlers.OptionsBuilder{
			Client: k8s,
		}

		hubEp, _ := url.Parse("http://remote-write.example.com")

		addonOpts := addon.Options{
			Platform: addon.PlatformOptions{
				Metrics: addon.MetricsOptions{CollectionEnabled: platformMetrics, HubEndpoint: *hubEp},
			},
			UserWorkloads: addon.UserWorkloadOptions{
				Metrics: addon.MetricsOptions{CollectionEnabled: userWorkloadMetrics},
			},
		}

		// opts, err := optionsBuilder.Build(context.Background(), mcAddon, cluster, addon.MetricsOptions{CollectionEnabled: platformMetrics, HubEndpoint: hubEp}, addon.MetricsOptions{CollectionEnabled: userWorkloadMetrics})
		opts, err := optionsBuilder.Build(context.Background(), mcAddon, cluster, addonOpts)
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

func newConfigReference(obj client.Object) addonapiv1alpha1.ConfigReference {
	resource := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind) + "s"

	return addonapiv1alpha1.ConfigReference{
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
	}
}

func runtimeToClientObjects(t *testing.T, objs []runtime.Object) []client.Object {
	clientObjs := make([]client.Object, 0, len(objs))
	for _, obj := range objs {
		co, ok := obj.(client.Object)
		if !ok {
			t.Fatalf("failed to convert %q to client.Object", obj.GetObjectKind().GroupVersionKind())
		}
		clientObjs = append(clientObjs, co)

	}
	return clientObjs
}

func formatObjects(objects []client.Object) string {
	s := []string{}
	for _, o := range objects {
		s = append(s, fmt.Sprintf("%s/%s/%s", o.GetObjectKind().GroupVersionKind().Kind, o.GetNamespace(), o.GetName()))
	}
	return strings.Join(s, "\n")
}

func newAddonOptions(platformEnabled, uwlEnabled bool) addon.Options {
	hubEp, _ := url.Parse("http://remote-write.example.com")
	return addon.Options{
		Platform: addon.PlatformOptions{
			Metrics: addon.MetricsOptions{
				CollectionEnabled: platformEnabled,
				HubEndpoint:       *hubEp,
			},
		},
		UserWorkloads: addon.UserWorkloadOptions{
			Metrics: addon.MetricsOptions{
				CollectionEnabled: uwlEnabled,
			},
		},
	}
}

func newCMOA() *addonapiv1alpha1.ClusterManagementAddOn {
	return &addonapiv1alpha1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  types.UID("test-cmao-uid"),
		},
		Spec: addonapiv1alpha1.ClusterManagementAddOnSpec{
			InstallStrategy: addonapiv1alpha1.InstallStrategy{
				Placements: []addonapiv1alpha1.PlacementStrategy{
					{
						PlacementRef: addonapiv1alpha1.PlacementRef{
							Namespace: "placement-ns",
							Name:      "placement-name",
						},
					},
				},
			},
		},
	}
}

func newManifestWork(name string, isOLMSubscrided bool) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
			Labels: map[string]string{
				addonapiv1alpha1.AddonLabelKey: addoncfg.Name,
			},
		},
		Status: workv1.ManifestWorkStatus{
			ResourceStatus: workv1.ManifestResourceStatus{
				Manifests: []workv1.ManifestCondition{
					{
						ResourceMeta: workv1.ManifestResourceMeta{
							Group:    apiextensionsv1.GroupName,
							Resource: "customresourcedefinitions",
							Name:     fmt.Sprintf("%s.%s", cooprometheusv1alpha1.PrometheusAgentName, cooprometheusv1alpha1.SchemeGroupVersion.Group),
						},
						StatusFeedbacks: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name: addoncfg.IsEstablishedFeedbackName,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: ptr.To("True"),
									},
								},
								{
									Name: addoncfg.LastTransitionTimeFeedbackName,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: ptr.To("12:00"),
									},
								},
							},
						},
					},
					{
						ResourceMeta: workv1.ManifestResourceMeta{
							Group:    apiextensionsv1.GroupName,
							Resource: "customresourcedefinitions",
							Name:     fmt.Sprintf("%s.%s", cooprometheusv1alpha1.ScrapeConfigName, cooprometheusv1alpha1.SchemeGroupVersion.Group),
						},
						StatusFeedbacks: workv1.StatusFeedbackResult{
							Values: []workv1.FeedbackValue{
								{
									Name: addoncfg.IsEstablishedFeedbackName,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: ptr.To("True"),
									},
								},
								{
									Name: addoncfg.LastTransitionTimeFeedbackName,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: ptr.To("12:00"),
									},
								},
								{
									Name: addoncfg.IsOLMManagedFeedbackName,
									Value: workv1.FieldValue{
										Type:   workv1.String,
										String: ptr.To(cases.Title(language.English).String(strconv.FormatBool(isOLMSubscrided))),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
