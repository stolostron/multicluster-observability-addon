package metrics_test

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
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
	hubNamespace := "open-cluster-management-observability"

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
				assert.Contains(t, agent[0].Spec.ConfigMaps, "my-configmap")
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
				// 4 (prom operator) + 5 (agent) + 2 secrets (mTLS to hub) + 1 cm (prom ca) + 2 rule + 2 scrape config + 1 configmap = 17
				expectedCount := 34
				if len(objects) != expectedCount {
					t.Fatalf("expected %d objects, but got %d:\n%s", expectedCount, len(objects), formatObjects(objects))
				}
				secrets := common.FilterResourcesByLabelSelector[*corev1.Secret](objects, nil)
				assert.Len(t, secrets, 4) // 4 secrets (mTLS to hub) + alertmananger secrets (accessor+ca in platform)

				// Ensure that the original resource annotation is set
				for _, obj := range secrets {
					origin := obj.Annotations[addoncfg.AnnotationOriginalResource]
					assert.NotEmpty(t, origin, "original resource annotation should not be empty", "name", obj.Name, "annotation", origin)
				}
				configmaps := common.FilterResourcesByLabelSelector[*corev1.ConfigMap](objects, nil)
				assert.Greater(t, len(configmaps), 1)

				// Ensure that the original resource annotation is set
				for _, obj := range configmaps {
					if obj.Name == config.PrometheusCAConfigMapName {
						// ignore this configmap directly defined in helm charts
						continue
					}
					origin := obj.Annotations[addoncfg.AnnotationOriginalResource]
					assert.NotEmpty(t, origin, "original resource annotation should not be empty", "name", obj.Name, "annotation", origin)
				}
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
				expectedCount := 26
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
				expectedCount := 34
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

				crds := common.FilterResourcesByLabelSelector[*apiextensionsv1.CustomResourceDefinition](objects, nil)
				checkedCRDs := 0
				for _, crd := range crds {
					if crd.Spec.Group != "monitoring.rhobs" {
						continue
					}
					checkedCRDs++
					assert.Contains(t, crd.Annotations, "addon.open-cluster-management.io/deletion-orphan")
				}
				assert.NotZero(t, checkedCRDs)

				// ensure that the number of objects is correct
				expectedCount := 26
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

				matchLabels := map[string]string{
					"app.kubernetes.io/name": "prometheus-operator",
				}

				// Ensure the ServiceMonitor is configured for HTTP
				sms := common.FilterResourcesByLabelSelector[*prometheusv1.ServiceMonitor](objects, matchLabels)
				assert.Len(t, sms, 1)
				operatorSM := sms[0]
				assert.Equal(t, "http", operatorSM.Spec.Endpoints[0].Scheme)
				assert.Nil(t, operatorSM.Spec.Endpoints[0].TLSConfig)

				// Ensure the Service is targeting the correct port
				svcs := common.FilterResourcesByLabelSelector[*corev1.Service](objects, matchLabels)
				assert.Len(t, svcs, 1)
				operatorSvc := svcs[0]
				assert.Equal(t, "metrics", operatorSvc.Spec.Ports[0].TargetPort.StrVal)
				assert.Equal(t, int32(8080), operatorSvc.Spec.Ports[0].Port)

				// Ensure the Deployment has the correct ports and no sidecar
				deps := common.FilterResourcesByLabelSelector[*appsv1.Deployment](objects, matchLabels)
				assert.Len(t, deps, 1)
				operatorDep := deps[0]

				// Check for metrics port on the main container
				metricsPortFound := false
				for _, port := range operatorDep.Spec.Template.Spec.Containers[0].Ports {
					if port.ContainerPort == 8080 && port.Name == "metrics" {
						metricsPortFound = true
						break
					}
				}
				assert.True(t, metricsPortFound, "Metrics port 8080 not found on operator container")
				// Check that kube-rbac-proxy sidecar is NOT present
				sidecarFound := false
				for _, c := range operatorDep.Spec.Template.Spec.Containers {
					if c.Name == "kube-rbac-proxy" {
						sidecarFound = true
						break
					}
				}
				assert.False(t, sidecarFound, "kube-rbac-proxy sidecar should not be present")

				// ensure that the number of objects is correct
				expectedCount := 70
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
				expectedCount := 71
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
	assert.NoError(t, operatorv1.AddToScheme(scheme))
	assert.NoError(t, hyperv1.AddToScheme(scheme))

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

			// Add a configmap to the default agent resources
			cm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-configmap",
					Namespace: hubNamespace,
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			defaultAgentResources = append(defaultAgentResources, cm)

			// add a cluster id
			clusterVersion := &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Spec: configv1.ClusterVersionSpec{
					ClusterID: configv1.ClusterID("97e51387-3da1-4ae4-89e3-f29bcd42fd42"),
				},
			}

			defaultAgentResources = append(defaultAgentResources, clusterVersion)

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
			uwlRulesAdditional.Annotations = map[string]string{config.TargetNamespaceAnnotation: "target-namespace"}
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

			// Add default ingress controller
			clientObjects = append(clientObjects, &operatorv1.IngressController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "openshift-ingress-operator",
				},
			})

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
					Get: func(ctx context.Context, clientww client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						err := clientww.Get(ctx, key, obj, opts...)
						if err != nil {
							return err
						}
						// Ensure GVK is set for PrometheusAgent objects during Get operations
						if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
							if pa.GroupVersionKind().Kind == "" {
								pa.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
							}
						}
						return nil
					},
					Update: func(ctx context.Context, clientww client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
						// Set GVK for PrometheusAgent objects during Update operations
						if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
							if pa.GroupVersionKind().Kind == "" {
								pa.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
							}
						}
						return clientww.Update(ctx, obj, opts...)
					},
					Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						var originalTypeMeta metav1.TypeMeta
						if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
							originalTypeMeta = pa.TypeMeta
							if pa.GroupVersionKind().Kind == "" {
								pa.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
							}
						}

						// Filter out SSA-specific options that are incompatible with merge patches
						var filteredOpts []client.PatchOption
						for _, opt := range opts {
							// Skip SSA-specific options by checking their string representation
							optStr := fmt.Sprintf("%T", opt)
							if strings.Contains(optStr, "forceOwnership") || strings.Contains(optStr, "FieldOwner") {
								continue
							}
							filteredOpts = append(filteredOpts, opt) // Keep all other options
						}

						err := clientww.Patch(ctx, obj, client.Merge, filteredOpts...)

						if err == nil && originalTypeMeta.Kind != "" {
							if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
								pa.TypeMeta = originalTypeMeta
							}
						}

						return err
					},
					List: func(ctx context.Context, clientww client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
						err := clientww.List(ctx, obj, opts...)
						if err != nil {
							return err
						}
						if paList, ok := obj.(*cooprometheusv1alpha1.PrometheusAgentList); ok {
							for i := range paList.Items {
								if paList.Items[i].GroupVersionKind().Kind == "" {
									paList.Items[i].SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
								}
							}
						}
						return nil
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
			// Update the prometheus agents to reference the configmap
			for i := range promAgents.Items {
				promAgents.Items[i].Spec.ConfigMaps = append(promAgents.Items[i].Spec.ConfigMaps, "my-configmap")
				err = client.Update(context.Background(), &promAgents.Items[i])
				require.NoError(t, err)
			}

			// Get updated agents for config references (they have proper GVK after update)
			updatedPromAgents := cooprometheusv1alpha1.PrometheusAgentList{}
			err = client.List(context.Background(), &updatedPromAgents)
			require.NoError(t, err)

			configReferences = append(configReferences, newConfigReference(&updatedPromAgents.Items[0]), newConfigReference(&updatedPromAgents.Items[1]))

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
					if obj.GetObjectKind().GroupVersionKind().Kind == "PrometheusRule" && accessor.GetName() == "uwl-rules-additional" {
						assert.Equal(t, "target-namespace", accessor.GetNamespace(), fmt.Sprintf("Object: %s/%s", obj.GetObjectKind().GroupVersionKind(), accessor.GetName()))
					} else {
						assert.Equal(t, tc.InstallNamespace, accessor.GetNamespace(), fmt.Sprintf("Object: %s/%s", obj.GetObjectKind().GroupVersionKind(), accessor.GetName()))
					}
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
	assert.NoError(t, workv1.AddToScheme(scheme))
	assert.NoError(t, operatorv1.AddToScheme(scheme))

	installNamespace := "open-cluster-management-addon-observability"
	hubNamespace := "open-cluster-management-observability"

	// Add user workload resources
	defaultAgentResources := []client.Object{}

	configReferences := []addonapiv1alpha1.ConfigReference{}
	for _, obj := range defaultAgentResources {
		configReferences = append(configReferences, newConfigReference(obj))
	}

	// add a cluster id
	clusterVersion := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Spec: configv1.ClusterVersionSpec{
			ClusterID: configv1.ClusterID("97e51387-3da1-4ae4-89e3-f29bcd42fd42"),
		},
	}

	defaultAgentResources = append(defaultAgentResources, clusterVersion)

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

	// Add default ingress controller
	clientObjects = append(clientObjects, &operatorv1.IngressController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "openshift-ingress-operator",
		},
	})

	// Setup a the local cluster as managed cluster
	managedCluster := addontesting.NewManagedCluster("cluster-1")
	managedCluster.Labels = map[string]string{
		config.LocalManagedClusterLabel:    "true",
		config.HypershiftAddonStateLabel:   "available",
		clusterinfov1beta1.LabelKubeVendor: string(clusterinfov1beta1.KubeVendorOpenShift),
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
			Get: func(ctx context.Context, clientww client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				err := clientww.Get(ctx, key, obj, opts...)
				if err != nil {
					return err
				}
				if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
					if pa.GroupVersionKind().Kind == "" {
						pa.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
					}
				}
				return nil
			},
			Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				var originalTypeMeta metav1.TypeMeta
				if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
					originalTypeMeta = pa.TypeMeta
					if pa.GroupVersionKind().Kind == "" {
						pa.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
					}
				}

				// Filter out SSA-specific options that are incompatible with merge patches
				var filteredOpts []client.PatchOption
				for _, opt := range opts {
					optStr := fmt.Sprintf("%T", opt)
					if strings.Contains(optStr, "forceOwnership") || strings.Contains(optStr, "FieldOwner") {
						continue
					}
					filteredOpts = append(filteredOpts, opt)
				}

				err := clientww.Patch(ctx, obj, client.Merge, filteredOpts...)

				if err == nil && originalTypeMeta.Kind != "" {
					if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
						pa.TypeMeta = originalTypeMeta
					}
				}

				return err
			},
			List: func(ctx context.Context, clientww client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
				err := clientww.List(ctx, obj, opts...)
				if err != nil {
					return err
				}
				// Ensure GVK is set for PrometheusAgent objects in lists
				if paList, ok := obj.(*cooprometheusv1alpha1.PrometheusAgentList); ok {
					for i := range paList.Items {
						if paList.Items[i].GroupVersionKind().Kind == "" {
							paList.Items[i].SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
						}
					}
				}
				return nil
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
	assert.Len(t, serviceMonitors, 4) // 2 for hcps and 1 for meta monitoring, 1 for obo-prometheus operator
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
