package handlers

import (
	"context"
	"testing"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildOptions(t *testing.T) {
	const (
		hubNamespace = "test-hub-namespace"
		spokeName    = "test-spoke"
		imagesCMName = "images-list"
		clusterID    = "test-cluster-id"
	)

	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, kubescheme.AddToScheme(scheme))
	require.NoError(t, prometheusalpha1.AddToScheme(scheme))
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	platformAgent := &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent",
			Namespace: hubNamespace,
			Labels:    config.PlatformPrometheusMatchLabels,
		},
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				LogLevel: "debug",
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

	platformScrapeConfig := &prometheusalpha1.ScrapeConfig{
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

	uwlAgent := &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent-uwl",
			Namespace: hubNamespace,
			Labels:    config.UserWorkloadPrometheusMatchLabels,
		},
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				LogLevel: "warn",
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
						Group:    "monitoring.coreos.com",
						Resource: prometheusalpha1.PrometheusAgentName,
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
						Group:    "monitoring.coreos.com",
						Resource: prometheusalpha1.ScrapeConfigName,
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
						clusterIDLabel: "test-cluster-id",
					},
				},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      imagesCMName,
					Namespace: hubNamespace,
				},
				Data: map[string]string{
					"prometheus_operator":        "prom-operator-image",
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
				res := filterOutResource[*corev1.ConfigMap](createResources(), imagesCMName)
				res = append(res, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      imagesCMName,
						Namespace: hubNamespace,
					},
					Data: map[string]string{ // Missing image overrides for config reloader
						"prometheus_operator": "prom-operator-image",
						"kube_rbac_proxy":     "kube-rbac-proxy-image",
					},
				})
				return res
			},
			expects: func(t *testing.T, opts Options, err error) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrMissingImageOverride)
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
				assert.ErrorIs(t, err, ErrInvalidConfigResourcesCount)
			},
		},

		"platform collection is enabled": {
			resources:       createResources,
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
				assert.Equal(t, "prom-operator-image", opts.Images.PrometheusOperator)
				assert.Equal(t, "kube-rbac-proxy-image", opts.Images.KubeRBACProxy)
				assert.Equal(t, "prometheus-config-reload-image", opts.Images.PrometheusConfigReloader)
				// Check that the Prometheus agent is set
				assert.NotNil(t, opts.Platform.PrometheusAgent)
				assert.Equal(t, platformAgent.Spec.LogLevel, opts.Platform.PrometheusAgent.Spec.LogLevel)
				assert.Len(t, opts.Platform.PrometheusAgent.Spec.RemoteWrite, 1)
				// Check that the HAProxy config map is set
				assert.Len(t, opts.ConfigMaps, 1)
				// Check that the secrets are set
				assert.Len(t, opts.Secrets, 2)
				// Check that user workloads are not enabled
				assert.Nil(t, opts.UserWorkloads.PrometheusAgent)
				// Check that scrape configs are set
				assert.Len(t, opts.Platform.ScrapeConfigs, 1)
				// Check that the Prometheus rule is set
				assert.Len(t, opts.Platform.Rules, 1)
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
								Group:    "monitoring.coreos.com",
								Resource: prometheusalpha1.PrometheusAgentName,
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
				// Check that the HAProxy config map is set
				assert.Len(t, opts.ConfigMaps, 1)
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

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(resources...).Build()
			platform := addon.MetricsOptions{CollectionEnabled: tc.platformEnabled}
			userWorkloads := addon.MetricsOptions{CollectionEnabled: tc.userWorkloadsEnabled}

			optsBuilder := &OptionsBuilder{
				Client:          fakeClient,
				ImagesConfigMap: types.NamespacedName{Name: imagesCMName, Namespace: hubNamespace},
				RemoteWriteURL:  "https://example.com/write",
			}
			opts, err := optsBuilder.Build(context.Background(), tc.addon, platform, userWorkloads)

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
