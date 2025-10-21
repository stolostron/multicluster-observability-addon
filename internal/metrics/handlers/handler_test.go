package handlers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	clusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestBuildOptions(t *testing.T) {
	const (
		hubNamespace = "test-hub-namespace"
		spokeName    = "test-spoke"
		clusterID    = "test-cluster-id"
	)

	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))
	require.NoError(t, hyperv1.AddToScheme(scheme))
	require.NoError(t, workv1.AddToScheme(scheme))

	platformAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent",
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				LogLevel:   "debug",
				ConfigMaps: []string{"test-haproxy-config"},
				RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
					{
						Name: ptr.To(config.RemoteWriteCfgName),
					},
				},
				Secrets: []string{config.ClientCertSecretName, config.HubCASecretName},
			},
		},
	}

	platformHAProxyCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-haproxy-config",
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
		Data: map[string]string{},
	}

	platformScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-scrape-config",
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
	}

	platformRule := &prometheusv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-rule",
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
	}

	uwlAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent-uwl",
			Namespace: hubNamespace,
			Labels:    config.UserWorkloadPrometheusMatchLabels,
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				LogLevel:   "warn",
				ConfigMaps: []string{"test-haproxy-config-uwl"},
				RemoteWrite: []cooprometheusv1.RemoteWriteSpec{
					{
						Name: ptr.To(config.RemoteWriteCfgName),
					},
				},
			},
		},
	}

	uwlHAProxyCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-haproxy-config-uwl",
			Namespace: hubNamespace,
			Labels:    config.UserWorkloadPrometheusMatchLabels,
		},
		Data: map[string]string{},
	}

	platformManagedClusterAddOn := &addonapiv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: spokeName,
			Name:      "observability-controller",
		},
		Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "monitoring.rhobs",
						Resource: cooprometheusv1alpha1.PrometheusAgentName,
					},
					DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
						ConfigReferent: addonapiv1alpha1.ConfigReferent{
							Name:      platformAgent.Name,
							Namespace: platformAgent.Namespace,
						},
					},
				},
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "",
						Resource: "configmaps",
					},
					DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
						ConfigReferent: addonapiv1alpha1.ConfigReferent{
							Name:      platformHAProxyCM.Name,
							Namespace: platformHAProxyCM.Namespace,
						},
					},
				},
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "monitoring.rhobs",
						Resource: cooprometheusv1alpha1.ScrapeConfigName,
					},
					DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
						ConfigReferent: addonapiv1alpha1.ConfigReferent{
							Name:      platformScrapeConfig.Name,
							Namespace: platformScrapeConfig.Namespace,
						},
					},
				},
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "monitoring.coreos.com",
						Resource: prometheusv1.PrometheusRuleName,
					},
					DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
						ConfigReferent: addonapiv1alpha1.ConfigReferent{
							Name:      platformRule.Name,
							Namespace: platformRule.Namespace,
						},
					},
				},
			},
		},
	}

	cmao := &addonapiv1alpha1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  types.UID("test-cmao-uid"),
		},
	}
	require.NoError(t, controllerutil.SetOwnerReference(cmao, platformAgent, scheme))
	require.NoError(t, controllerutil.SetOwnerReference(cmao, uwlAgent, scheme))

	// Alertmanager secrets
	alertmanagerAccessorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.AlertmanagerAccessorSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string][]byte{
			"key":  []byte("data"),
			"pass": []byte("data"),
		},
	}
	alertmanagerRouterCA := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.RouterDefaultCertsConfigMapObjKey.Name,
			Namespace: config.RouterDefaultCertsConfigMapObjKey.Namespace,
		},
		Data: map[string][]byte{
			"tls.key": []byte("data"),
		},
	}

	createResources := func() []client.Object {
		return []client.Object{
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: hubNamespace,
				},
			},
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: spokeName,
				},
			},
			&clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: spokeName,
					Labels: map[string]string{
						config.ManagedClusterLabelClusterID: "test-cluster-id",
						clusterinfov1beta1.LabelKubeVendor:  string(clusterinfov1beta1.KubeVendorOpenShift),
					},
				},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ImagesConfigMapObjKey.Name,
					Namespace: config.ImagesConfigMapObjKey.Namespace,
				},
				Data: map[string]string{
					"obo_prometheus_operator":    "obo-prom-operator-image",
					"kube_rbac_proxy":            "kube-rbac-proxy-image",
					"prometheus_config_reloader": "prometheus-config-reload-image",
				},
			},
			platformAgent,
			platformHAProxyCM,
			platformScrapeConfig,
			platformRule,
			uwlAgent,
			uwlHAProxyCM,
			cmao,
			alertmanagerAccessorSecret,
			alertmanagerRouterCA,
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ClientCertSecretName,
					Namespace: hubNamespace,
				},
				Data: map[string][]byte{
					"tls.crt": []byte("test-crt"),
					"tls.key": []byte("test-key"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.HubCASecretName,
					Namespace: hubNamespace,
				},
				Data: map[string][]byte{
					"ca.crt": []byte("test-ca"),
				},
			},
		}
	}

	testCases := map[string]struct {
		addon                *addonapiv1alpha1.ManagedClusterAddOn
		platformEnabled      bool
		userWorkloadsEnabled bool
		resources            func() []client.Object
		expects              func(t *testing.T, opts Options, err error)
	}{
		"no metrics collection enabled": {
			resources: createResources,
			expects: func(t *testing.T, opts Options, err error) {
				assert.Empty(t, opts.ClusterName)
				assert.Empty(t, opts.ClusterID)
				assert.Nil(t, opts.Platform.PrometheusAgent)
				assert.Nil(t, opts.UserWorkloads.PrometheusAgent)
			},
		},
		"missing cluster ID": {
			addon:           platformManagedClusterAddOn,
			platformEnabled: true,
			resources: func() []client.Object {
				res := filterOutResource[*clusterv1.ManagedCluster](createResources(), "")
				res = append(res, &clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: spokeName,
					},
				})
				return res
			},
			expects: func(t *testing.T, opts Options, err error) {
				assert.NoError(t, err)
				assert.Equal(t, spokeName, opts.ClusterID) // Cluster ID should be set to the cluster name
			},
		},
		"missing image override": {
			addon:           platformManagedClusterAddOn,
			platformEnabled: true,
			resources: func() []client.Object {
				res := filterOutResource[*corev1.ConfigMap](createResources(), config.ImagesConfigMapObjKey.Name)
				res = append(res, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.ImagesConfigMapObjKey.Name,
						Namespace: config.ImagesConfigMapObjKey.Namespace,
					},
					Data: map[string]string{ // Missing image overrides for config reloader
						"obo_prometheus_operator": "obo_prom-operator-image",
						"kube_rbac_proxy":         "kube-rbac-proxy-image",
					},
				})
				return res
			},
			expects: func(t *testing.T, opts Options, err error) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, config.ErrMissingImageOverride)
			},
		},
		"missing config reference": {
			addon: &addonapiv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: spokeName,
					Name:      "observability-controller",
				},
			},
			platformEnabled: true,
			resources:       createResources,
			expects: func(t *testing.T, opts Options, err error) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, errInvalidConfigResourcesCount)
			},
		},

		"platform collection is enabled, coo is not installed": {
			resources: func() []client.Object {
				ret := createResources()
				ret = append(ret, newManifestWork(spokeName, false))
				return ret
			},
			addon:           platformManagedClusterAddOn,
			platformEnabled: true,
			expects: func(t *testing.T, opts Options, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				// Check that cluster identifiers are set
				assert.Equal(t, "test-cluster-id", opts.ClusterID)
				assert.Equal(t, "test-spoke", opts.ClusterName)
				// Check that image overrides are set
				assert.Equal(t, "obo-prom-operator-image", opts.Images.CooPrometheusOperatorImage)
				assert.Equal(t, "kube-rbac-proxy-image", opts.Images.KubeRBACProxy)
				assert.Equal(t, "prometheus-config-reload-image", opts.Images.PrometheusConfigReloader)
				// Check that the Prometheus agent is set
				assert.NotNil(t, opts.Platform.PrometheusAgent)
				assert.Equal(t, platformAgent.Spec.LogLevel, opts.Platform.PrometheusAgent.Spec.LogLevel)
				assert.Len(t, opts.Platform.PrometheusAgent.Spec.RemoteWrite, 1)
				// Check that relabelling is added to the remote write config
				assert.Equal(t, spokeName, *opts.Platform.PrometheusAgent.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[0].Replacement)
				assert.Equal(t, config.ClusterNameMetricLabel, opts.Platform.PrometheusAgent.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[0].TargetLabel)
				assert.Len(t, opts.Platform.PrometheusAgent.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs, 5)
				// Check that the secrets are set
				assert.Len(t, opts.Secrets, 6)
				// Check that user workloads are not enabled
				assert.Nil(t, opts.UserWorkloads.PrometheusAgent)
				// Check that scrape configs are set
				assert.Len(t, opts.Platform.ScrapeConfigs, 1)
				// Check that the Prometheus rule is set
				assert.Len(t, opts.Platform.Rules, 1)
				assert.False(t, opts.COOIsSubscribed)
				assert.NotEmpty(t, opts.CRDEstablishedAnnotation)
			},
		},
		"platform collection is enabled, coo is installed": {
			resources: func() []client.Object {
				ret := createResources()
				ret = append(ret, newManifestWork(spokeName, true))
				return ret
			},
			addon:           platformManagedClusterAddOn,
			platformEnabled: true,
			expects: func(t *testing.T, opts Options, err error) {
				assert.True(t, opts.COOIsSubscribed)
				assert.Empty(t, opts.CRDEstablishedAnnotation)
			},
		},
		"user workloads collection is enabled": {
			resources: createResources,
			addon: &addonapiv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: spokeName,
					Name:      "observability-controller",
				},
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1alpha1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "monitoring.rhobs",
								Resource: cooprometheusv1alpha1.PrometheusAgentName,
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      uwlAgent.Name,
									Namespace: uwlAgent.Namespace,
								},
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "",
								Resource: "configmaps",
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      uwlHAProxyCM.Name,
									Namespace: uwlHAProxyCM.Namespace,
								},
							},
						},
					},
				},
			},
			userWorkloadsEnabled: true,
			expects: func(t *testing.T, opts Options, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, opts.UserWorkloads.PrometheusAgent)
				assert.Nil(t, opts.Platform.PrometheusAgent)
				assert.Equal(t, uwlAgent.Spec.LogLevel, opts.UserWorkloads.PrometheusAgent.Spec.LogLevel)
				// Check that relabelling is added to the remote write config
				assert.Equal(t, spokeName, *opts.UserWorkloads.PrometheusAgent.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[0].Replacement)
				assert.Equal(t, config.ClusterNameMetricLabel, opts.UserWorkloads.PrometheusAgent.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs[0].TargetLabel)
				assert.Len(t, opts.UserWorkloads.PrometheusAgent.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs, 5)
				assert.False(t, opts.COOIsSubscribed)
			},
		},
		"user workload is enabled and is hypershift hub": {
			addon: &addonapiv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: spokeName,
					Name:      "observability-controller",
				},
				Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
					ConfigReferences: []addonapiv1alpha1.ConfigReference{
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "monitoring.rhobs",
								Resource: cooprometheusv1alpha1.PrometheusAgentName,
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      uwlAgent.Name,
									Namespace: uwlAgent.Namespace,
								},
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "",
								Resource: "configmaps",
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      uwlHAProxyCM.Name,
									Namespace: uwlHAProxyCM.Namespace,
								},
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "",
								Resource: "scrapeconfigs",
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      "etcd-base",
									Namespace: hubNamespace,
								},
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "",
								Resource: "scrapeconfigs",
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      "apiserver-base",
									Namespace: hubNamespace,
								},
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "",
								Resource: "prometheusrules",
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      "etcd-base",
									Namespace: hubNamespace,
								},
							},
						},
						{
							ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
								Group:    "",
								Resource: "prometheusrules",
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      "apiserver-base",
									Namespace: hubNamespace,
								},
							},
						},
					},
				},
			},
			resources: func() []client.Object {
				res := filterOutResource[*clusterv1.ManagedCluster](createResources(), "")
				res = append(res, &clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "local-cluster-custom-name",
						Labels: map[string]string{
							addoncfg.ManagedClusterLabelClusterID:                       "test-cluster-id",
							"feature.open-cluster-management.io/addon-hypershift-addon": "available",
							"local-cluster": "true",
						},
					},
				})
				res = append(res, newHCPResources()...)
				res = append(res, newHCPConfigResources(hubNamespace)...)
				return res
			},
			userWorkloadsEnabled: true,
			expects: func(t *testing.T, opts Options, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, opts.UserWorkloads.ServiceMonitors)
				assert.Len(t, opts.UserWorkloads.ServiceMonitors, 2)
				assert.NotNil(t, opts.UserWorkloads.ScrapeConfigs)
				assert.Len(t, opts.UserWorkloads.ScrapeConfigs, 2)
				assert.NotNil(t, opts.UserWorkloads.Rules)
				assert.Len(t, opts.UserWorkloads.Rules, 2)

				var etcdMetrics, apiserverMetrics []string
				for _, sm := range opts.UserWorkloads.ServiceMonitors {
					if sm.Name == config.AcmEtcdServiceMonitorName {
						etcdMetrics = extractMetricsFilterFromServiceMonitor(sm)
					}
					if sm.Name == config.AcmApiServerServiceMonitorName {
						apiserverMetrics = extractMetricsFilterFromServiceMonitor(sm)
					}
				}

				assert.Equal(t, []string{"etcd_metric", "etcd_rule_dependent_metric"}, etcdMetrics)
				assert.Equal(t, []string{"apiserver_metric", "apiserver_rule_dependent_metric"}, apiserverMetrics)

				assert.Len(t, opts.UserWorkloads.PrometheusAgent.Spec.CommonPrometheusFields.RemoteWrite[0].WriteRelabelConfigs, 8)
			},
		},
	}

	// Run the test cases
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			resources := tc.resources()
			if tc.addon != nil {
				resources = append(resources, tc.addon)
			}
			hubEp, _ := url.Parse("http://remote-write.example.com")
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(resources...).Build()
			platform := addon.MetricsOptions{CollectionEnabled: tc.platformEnabled, HubEndpoint: hubEp}
			userWorkloads := addon.MetricsOptions{CollectionEnabled: tc.userWorkloadsEnabled}

			optsBuilder := &OptionsBuilder{
				Client:         fakeClient,
				RemoteWriteURL: "https://example.com/write",
			}
			managedClusters := &clusterv1.ManagedClusterList{}
			err := fakeClient.List(context.Background(), managedClusters)
			require.NoError(t, err)
			require.Len(t, managedClusters.Items, 1)
			foundManagedCluster := managedClusters.Items[0]
			opts, err := optsBuilder.Build(context.Background(), tc.addon, &foundManagedCluster, platform, userWorkloads)

			tc.expects(t, opts, err)
		})
	}
}

func filterOutResource[T client.Object](resources []client.Object, name string) []client.Object {
	filtered := make([]client.Object, 0, len(resources))

	for _, res := range resources {
		if _, ok := res.(T); ok && (res.GetName() == name || name == "") {
			continue
		}

		filtered = append(filtered, res)
	}

	return filtered
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

func newHCPResources() []client.Object {
	targetPort := intstr.FromString("target")
	return []client.Object{
		&hyperv1.HostedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
			Spec: hyperv1.HostedClusterSpec{
				ClusterID: "cluster-id",
			},
		},
		&prometheusv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.HypershiftEtcdServiceMonitorName,
				Namespace: "namespace-name",
			},
			Spec: prometheusv1.ServiceMonitorSpec{
				Endpoints: []prometheusv1.Endpoint{
					{
						Port: "metrics",
					},
				},
			},
		},
		&prometheusv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.HypershiftApiServerServiceMonitorName,
				Namespace: "namespace-name",
			},
			Spec: prometheusv1.ServiceMonitorSpec{
				Endpoints: []prometheusv1.Endpoint{
					{
						TargetPort: &targetPort,
						Port:       "client",
					},
				},
			},
		},
	}
}

func newHCPConfigResources(ns string) []client.Object {
	return []client.Object{
		&cooprometheusv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-base",
				Namespace: ns,
				Labels:    config.EtcdHcpUserWorkloadPrometheusMatchLabels,
			},
			Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
				Params: map[string][]string{
					"match[]": {
						`{__name__="etcd_metric"}`,
					},
				},
			},
		},
		&cooprometheusv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apiserver-base",
				Namespace: ns,
				Labels:    config.ApiserverHcpUserWorkloadPrometheusMatchLabels,
			},
			Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
				Params: map[string][]string{
					"match[]": {
						`{__name__="apiserver_metric"}`,
					},
				},
			},
		},
		&prometheusv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-base",
				Namespace: ns,
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
		},
		&prometheusv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apiserver-base",
				Namespace: ns,
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
		},
	}
}

func extractMetricsFilterFromServiceMonitor(sm *prometheusv1.ServiceMonitor) []string {
	for _, relabel := range sm.Spec.Endpoints[0].MetricRelabelConfigs {
		if relabel.Action != "keep" {
			continue
		}

		if relabel.SourceLabels[0] != "__name__" {
			continue
		}

		return strings.Split(strings.Trim(relabel.Regex, "()"), "|")
	}

	return []string{}
}
