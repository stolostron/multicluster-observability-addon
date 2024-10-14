package handlers

import (
	"context"
	"testing"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/assert"
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
	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	assert.NoError(t, prometheusalpha1.AddToScheme(scheme))
	assert.NoError(t, prometheusv1.AddToScheme(scheme))
	assert.NoError(t, clusterv1.AddToScheme(scheme))
	assert.NoError(t, addonapiv1alpha1.AddToScheme(scheme))

	hubNamespace := "test-hub-namespace"
	spokeName := "test-spoke" // is both the namespace and the name of the ManagedCluster
	imagesCMName := "images-list"

	// Resources
	platformAgent := &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent",
			Namespace: hubNamespace,
			Labels: map[string]string{
				"app": platformMetricsCollectorApp,
			},
		},
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				LogLevel: "debug",
			},
		},
	}

	platformScrapeConfig := &prometheusalpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-scrape-config",
			Namespace: hubNamespace,
			Labels: map[string]string{
				"app": platformMetricsCollectorApp,
			},
		},
	}

	platformRule := &prometheusv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-rule",
			Namespace: hubNamespace,
			Labels: map[string]string{
				"app": platformMetricsCollectorApp,
			},
		},
	}

	uwlAgent := &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-prometheus-agent-uwl",
			Namespace: hubNamespace,
			Labels: map[string]string{
				"app": userWorkloadMetricsCollectorApp,
			},
		},
		Spec: prometheusalpha1.PrometheusAgentSpec{
			CommonPrometheusFields: prometheusv1.CommonPrometheusFields{
				LogLevel: "warn",
			},
		},
	}

	commonResources := []client.Object{
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
		platformScrapeConfig,
		platformRule,
		uwlAgent,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clientCertSecretName,
				Namespace: hubNamespace,
			},
			Data: map[string][]byte{
				"tls.crt": []byte("test-crt"),
				"tls.key": []byte("test-key"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hubCASecretName,
				Namespace: hubNamespace,
			},
			Data: map[string][]byte{
				"ca.crt": []byte("test-ca"),
			},
		},
	}

	testCases := map[string]struct {
		addon                *addonapiv1alpha1.ManagedClusterAddOn
		platformEnabled      bool
		userWorkloadsEnabled bool
		resources            []client.Object
		expects              func(t *testing.T, opts Options, err error)
	}{
		"platform collection is enabled": {
			resources: commonResources,
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
								Resource: addon.PrometheusAgentResource,
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
								Group:    "monitoring.coreos.com",
								Resource: addon.PrometheusScrapeConfigResource,
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
								Resource: addon.PrometheusRuleResource,
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
			},
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
			resources: commonResources,
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
								Resource: addon.PrometheusAgentResource,
							},
							DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
								ConfigReferent: addonapiv1alpha1.ConfigReferent{
									Name:      uwlAgent.Name,
									Namespace: uwlAgent.Namespace,
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
			},
		},
		// "not found config reference": {
		// "missing required config reference": {
		// "missing managed cluster": {
		// "missing referenced secret": {

		// Test failure cases: missing image override, missing config resource, missing Prometheus agent, missing secrets, missing managed cluster
		// "error case - missing managed cluster": {
		// 	resources: []client.Object{
		// 		&corev1.ConfigMap{
		// 			ObjectMeta: metav1.ObjectMeta{
		// 				Name:      imagesCMName,
		// 				Namespace: hubNamespace,
		// 			},
		// 			Data: map[string]string{
		// 				"prometheus_operator": "prom-operator-image",
		// 				"haproxy":             "haproxy-image",
		// 			},
		// 		},
		// 	},
		// 	expects: func(t *testing.T, opts manifests.Options, err error) {
		// 		assert.Error(t, err)
		// 	},
		// },
		// "missing HAProxy image in ConfigMap": {
		// 	resources: []client.Object{
		// 		&clusterv1.ManagedCluster{
		// 			ObjectMeta: metav1.ObjectMeta{
		// 				Name: "test-cluster",
		// 				Labels: map[string]string{
		// 					clusterIDLabel: "test-cluster-id",
		// 				},
		// 			},
		// 		},
		// 		&corev1.ConfigMap{
		// 			ObjectMeta: metav1.ObjectMeta{
		// 				Name:      imagesCMName,
		// 				Namespace: hubNamespace,
		// 			},
		// 			Data: map[string]string{
		// 				"prometheus_operator": "prom-operator-image",
		// 			},
		// 		},
		// 	},
		// 	expects: func(t *testing.T, opts manifests.Options, err error) {
		// 		assert.NoError(t, err)
		// 		// Default HAProxy image should be used
		// 		assert.Equal(t, "registry.connect.redhat.com/haproxytech/haproxy@sha256:07ee4e701e6ce23d6c35b37d159244fb14ef9c90190710542ce60492cbe4d68a", opts.Images.HAProxy)
		// 	},
		// },
	}

	// Run the test cases
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			resources := []client.Object{}
			resources = append(resources, tc.resources...)
			resources = append(resources, tc.addon)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(resources...).Build()
			platform := addon.MetricsOptions{CollectionEnabled: tc.platformEnabled}
			userWorkloads := addon.MetricsOptions{CollectionEnabled: tc.userWorkloadsEnabled}

			optsBuilder := &OptionsBuilder{
				Client:          fakeClient,
				HubNamespace:    hubNamespace,
				ImagesConfigMap: types.NamespacedName{Name: imagesCMName, Namespace: hubNamespace},
				RemoteWriteURL:  "https://example.com/write",
			}
			opts, err := optsBuilder.Build(context.Background(), tc.addon, platform, userWorkloads)

			tc.expects(t, opts, err)
		})
	}
}
