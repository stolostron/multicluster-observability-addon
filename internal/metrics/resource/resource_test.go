package resource

import (
	"context"
	"net/url"
	"testing"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestCreateDefaultAgent(t *testing.T) {
	globalPlacement := addonv1beta1.PlacementStrategy{
		PlacementRef: addonv1beta1.PlacementRef{
			Name:      "global",
			Namespace: config.HubInstallNamespace,
		},
	}

	testCases := []struct {
		name               string
		isUWL              bool
		placements         []addonv1beta1.PlacementStrategy
		initObjs           []client.Object
		expectCreated      bool
		expectTargetsDummy bool
	}{
		{
			name:               "creates dummy agent when no global placement",
			isUWL:              false,
			placements:         []addonv1beta1.PlacementStrategy{},
			expectCreated:      true,
			expectTargetsDummy: true,
		},
		{
			name:               "creates global agent when global placement exists and no agent covers it",
			isUWL:              false,
			placements:         []addonv1beta1.PlacementStrategy{globalPlacement},
			expectCreated:      true,
			expectTargetsDummy: false,
		},
		{
			name:       "creates dummy agent when global exists but another user agent already covers it",
			isUWL:      false,
			placements: []addonv1beta1.PlacementStrategy{globalPlacement},
			initObjs: []client.Object{
				func() client.Object {
					a := NewDefaultPrometheusAgent(config.HubInstallNamespace, "user-agent", false)
					a.Labels[addoncfg.PartOfK8sLabelKey] = addoncfg.Name
					a.Annotations = map[string]string{
						addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/global",
					}
					return a
				}(),
			},
			expectCreated:      true,
			expectTargetsDummy: true,
		},
		{
			name:       "returns nil when default global agent already exists",
			isUWL:      false,
			placements: []addonv1beta1.PlacementStrategy{globalPlacement},
			initObjs: []client.Object{
				func() client.Object {
					appName := config.PlatformMetricsCollectorApp
					a := NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(appName, "global")+"-default", false)
					a.Annotations = map[string]string{
						addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/global",
					}
					a.OwnerReferences = []metav1.OwnerReference{
						{UID: types.UID("test-cmao-uid"), Controller: ptr.To(true)},
					}
					return a
				}(),
			},
			expectCreated: false,
		},
		{
			name:       "returns nil when dummy agent already exists",
			isUWL:      false,
			placements: []addonv1beta1.PlacementStrategy{},
			initObjs: []client.Object{
				func() client.Object {
					appName := config.PlatformMetricsCollectorApp
					a := NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(appName, "dummy"), false)
					a.Annotations = map[string]string{
						addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/dummy",
					}
					a.OwnerReferences = []metav1.OwnerReference{
						{UID: types.UID("test-cmao-uid"), Controller: ptr.To(true)},
					}
					return a
				}(),
			},
			expectCreated: false,
		},
		{
			name:               "creates uwl dummy agent when no global placement",
			isUWL:              true,
			placements:         []addonv1beta1.PlacementStrategy{},
			expectCreated:      true,
			expectTargetsDummy: true,
		},
		{
			name:               "creates uwl global agent when global placement exists",
			isUWL:              true,
			placements:         []addonv1beta1.PlacementStrategy{globalPlacement},
			expectCreated:      true,
			expectTargetsDummy: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmao := newCMAO(tc.placements...)
			initObjs := append(tc.initObjs, cmao)
			fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				Client: fakeClient,
				CMAO:   cmao,
				Logger: klog.Background(),
			}

			gotAgent, err := d.CreateDefaultAgent(context.Background(), tc.isUWL)
			require.NoError(t, err)

			if !tc.expectCreated {
				assert.Nil(t, gotAgent)
				return
			}

			require.NotNil(t, gotAgent)
			assert.True(t, controllerutil.HasControllerReference(gotAgent))

			// ensure correct labels
			if tc.isUWL {
				assert.Equal(t, config.UserWorkloadMetricsCollectorApp, gotAgent.Labels[addoncfg.ComponentK8sLabelKey])
			} else {
				assert.Equal(t, config.PlatformMetricsCollectorApp, gotAgent.Labels[addoncfg.ComponentK8sLabelKey])
			}

			// ensure annotation is set
			annotation := gotAgent.Annotations[addoncfg.PlacementAnnotationKey]
			assert.NotEmpty(t, annotation)
			if tc.expectTargetsDummy {
				assert.Contains(t, annotation, "/dummy")
			} else {
				assert.Contains(t, annotation, "/global")
			}
		})
	}
}

func TestReconcileAgents(t *testing.T) {
	globalPlacement := addonv1beta1.PlacementStrategy{
		PlacementRef: addonv1beta1.PlacementRef{Name: "global", Namespace: config.HubInstallNamespace},
	}
	cmao := newCMAO(globalPlacement)
	opts := newAddonOptions(true, true)
	kubeRbacImage := "kube-rbac-proxy:version"
	prometheusImage := "prometheus:version"

	scheme := newTestScheme()
	fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(cmao).Build()
	d := DefaultStackResources{
		Client:             fakeClient,
		CMAO:               cmao,
		AddonOptions:       opts,
		Logger:             klog.Background(),
		KubeRBACProxyImage: kubeRbacImage,
		PrometheusImage:    prometheusImage,
	}

	// >>> Platform agents
	configs, err := d.reconcileAgents(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, configs, 1)

	foundAgent := cooprometheusv1alpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: configs[0].Config.Namespace, Name: configs[0].Config.Name}, &foundAgent)
	require.NoError(t, err)

	// Check default fields
	assert.EqualValues(t, 1, *foundAgent.Spec.Replicas)
	// Check ssa fields
	assert.Equal(t, kubeRbacImage, foundAgent.Spec.Containers[0].Image)
	assert.Equal(t, prometheusImage, *foundAgent.Spec.Image)
	assert.Equal(t, config.PlatformMetricsCollectorApp, foundAgent.Spec.ServiceAccountName)
	// Check platform specific values: ScrapeConfigNamespaceSelector
	assert.Nil(t, foundAgent.Spec.ScrapeConfigNamespaceSelector)
	// Check placement annotation
	assert.Contains(t, foundAgent.Annotations[addoncfg.PlacementAnnotationKey], "global")

	// Subsequent reconcile does not create additional agents
	configs2, err := d.reconcileAgents(context.Background(), false)
	require.NoError(t, err)
	assert.Len(t, configs2, 1)

	agents := cooprometheusv1alpha1.PrometheusAgentList{}
	err = fakeClient.List(context.Background(), &agents)
	require.NoError(t, err)
	platformCount := 0
	for _, a := range agents.Items {
		if a.Labels[addoncfg.ComponentK8sLabelKey] == config.PlatformMetricsCollectorApp {
			platformCount++
		}
	}
	assert.Equal(t, 1, platformCount)

	// >>> UWL agents
	configs, err = d.reconcileAgents(context.Background(), true)
	require.NoError(t, err)
	require.Len(t, configs, 1)

	foundAgent = cooprometheusv1alpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: configs[0].Config.Namespace, Name: configs[0].Config.Name}, &foundAgent)
	require.NoError(t, err)

	// Check uwl specific values: appName and ScrapeConfigNamespaceSelector
	assert.Equal(t, config.UserWorkloadMetricsCollectorApp, foundAgent.Spec.ServiceAccountName)
	assert.Equal(t, &metav1.LabelSelector{}, foundAgent.Spec.ScrapeConfigNamespaceSelector)
}

func TestReconcileAgentsUserDefined(t *testing.T) {
	globalPlacement := addonv1beta1.PlacementStrategy{
		PlacementRef: addonv1beta1.PlacementRef{Name: "global", Namespace: config.HubInstallNamespace},
	}
	placementB := addonv1beta1.PlacementStrategy{
		PlacementRef: addonv1beta1.PlacementRef{Name: "custom-placement", Namespace: config.HubInstallNamespace},
	}
	kubeRbacImage := "kube-rbac-proxy:version"
	prometheusImage := "prometheus:version"

	t.Run("SSA enforcement on user agent with conflicting fields", func(t *testing.T) {
		cmao := newCMAO(globalPlacement)
		opts := newAddonOptions(true, true)

		userAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, "my-custom-platform-agent", false)
		userAgent.Labels[addoncfg.PartOfK8sLabelKey] = addoncfg.Name
		userAgent.Annotations = map[string]string{
			addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/global",
		}
		userAgent.Spec.Image = ptr.To("wrong-image:latest")
		userAgent.Spec.ServiceAccountName = "wrong-sa"
		userAgent.Spec.ArbitraryFSAccessThroughSMs = cooprometheusv1.ArbitraryFSAccessThroughSMsConfig{Deny: false}

		scheme := newTestScheme()
		fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(cmao, userAgent).Build()
		d := DefaultStackResources{
			Client:             fakeClient,
			CMAO:               cmao,
			AddonOptions:       opts,
			Logger:             klog.Background(),
			KubeRBACProxyImage: kubeRbacImage,
			PrometheusImage:    prometheusImage,
		}

		configs, err := d.reconcileAgents(context.Background(), false)
		require.NoError(t, err)

		// Find the config for the user agent (targeting global)
		var userAgentConfig *common.DefaultConfig
		for i, cfg := range configs {
			if cfg.PlacementRef.Name == "global" {
				userAgentConfig = &configs[i]
				break
			}
		}
		require.NotNil(t, userAgentConfig, "expected a config targeting global placement from user agent")

		foundAgent := cooprometheusv1alpha1.PrometheusAgent{}
		err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: config.HubInstallNamespace, Name: "my-custom-platform-agent"}, &foundAgent)
		require.NoError(t, err)

		// SSA should enforce mandatory fields over user values
		assert.Equal(t, prometheusImage, *foundAgent.Spec.Image)
		assert.Equal(t, config.PlatformMetricsCollectorApp, foundAgent.Spec.ServiceAccountName)
		assert.True(t, foundAgent.Spec.ArbitraryFSAccessThroughSMs.Deny)
		assert.NotEmpty(t, foundAgent.Spec.Containers, "kube-rbac-proxy sidecar should be injected")
		assert.Equal(t, kubeRbacImage, foundAgent.Spec.Containers[0].Image)
	})

	t.Run("user agent with multiple placement annotations", func(t *testing.T) {
		cmao := newCMAO(globalPlacement, placementB)
		opts := newAddonOptions(true, false)

		userAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, "multi-placement-agent", false)
		userAgent.Labels[addoncfg.PartOfK8sLabelKey] = addoncfg.Name
		userAgent.Annotations = map[string]string{
			addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/global," + config.HubInstallNamespace + "/custom-placement",
		}

		scheme := newTestScheme()
		fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(cmao, userAgent).Build()
		d := DefaultStackResources{
			Client:             fakeClient,
			CMAO:               cmao,
			AddonOptions:       opts,
			Logger:             klog.Background(),
			KubeRBACProxyImage: kubeRbacImage,
			PrometheusImage:    prometheusImage,
		}

		configs, err := d.reconcileAgents(context.Background(), false)
		require.NoError(t, err)

		// User agent targets both placements, plus dummy agent creates a config for dummy
		globalConfigs := 0
		customConfigs := 0
		for _, cfg := range configs {
			if cfg.PlacementRef.Name == "global" {
				globalConfigs++
			}
			if cfg.PlacementRef.Name == "custom-placement" {
				customConfigs++
			}
		}
		assert.Equal(t, 1, globalConfigs, "user agent should produce a config for global")
		assert.Equal(t, 1, customConfigs, "user agent should produce a config for custom-placement")
	})

	t.Run("user agent without placement annotation generates no configs", func(t *testing.T) {
		cmao := newCMAO(globalPlacement)
		opts := newAddonOptions(true, false)

		userAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, "no-annotation-agent", false)
		userAgent.Labels[addoncfg.PartOfK8sLabelKey] = addoncfg.Name
		// No placement annotation set — agent has correct labels but no annotation

		scheme := newTestScheme()
		fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(cmao, userAgent).Build()
		d := DefaultStackResources{
			Client:             fakeClient,
			CMAO:               cmao,
			AddonOptions:       opts,
			Logger:             klog.Background(),
			KubeRBACProxyImage: kubeRbacImage,
			PrometheusImage:    prometheusImage,
		}

		configs, err := d.reconcileAgents(context.Background(), false)
		require.NoError(t, err)

		// The agent without annotation should not generate any configs
		// Only the dummy agent (created by CreateDefaultAgent since global is covered by no one with annotation)
		// Actually since "no-annotation-agent" doesn't target global, CreateDefaultAgent creates global-default
		// global-default targets global → 1 config
		for _, cfg := range configs {
			assert.NotEqual(t, "no-annotation-agent", cfg.Config.Name,
				"agent without annotation should not produce a config")
		}

		// Verify SSA was still applied to the no-annotation agent
		foundAgent := cooprometheusv1alpha1.PrometheusAgent{}
		err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: config.HubInstallNamespace, Name: "no-annotation-agent"}, &foundAgent)
		require.NoError(t, err)
		assert.Equal(t, prometheusImage, *foundAgent.Spec.Image, "SSA should still enforce image")
		assert.Equal(t, config.PlatformMetricsCollectorApp, foundAgent.Spec.ServiceAccountName, "SSA should still enforce serviceAccountName")
	})

	t.Run("user agent targeting non-global placement", func(t *testing.T) {
		cmao := newCMAO(globalPlacement, placementB)
		opts := newAddonOptions(true, false)

		userAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, "custom-placement-agent", false)
		userAgent.Labels[addoncfg.PartOfK8sLabelKey] = addoncfg.Name
		userAgent.Annotations = map[string]string{
			addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/custom-placement",
		}

		scheme := newTestScheme()
		fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(cmao, userAgent).Build()
		d := DefaultStackResources{
			Client:             fakeClient,
			CMAO:               cmao,
			AddonOptions:       opts,
			Logger:             klog.Background(),
			KubeRBACProxyImage: kubeRbacImage,
			PrometheusImage:    prometheusImage,
		}

		configs, err := d.reconcileAgents(context.Background(), false)
		require.NoError(t, err)

		// User agent targets custom-placement, CreateDefaultAgent creates global-default for global
		globalConfigs := 0
		customConfigs := 0
		for _, cfg := range configs {
			if cfg.PlacementRef.Name == "global" {
				globalConfigs++
			}
			if cfg.PlacementRef.Name == "custom-placement" {
				customConfigs++
			}
		}
		assert.Equal(t, 1, globalConfigs, "default agent should produce a config for global")
		assert.Equal(t, 1, customConfigs, "user agent should produce a config for custom-placement")

		// Verify user agent had SSA applied
		foundAgent := cooprometheusv1alpha1.PrometheusAgent{}
		err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: config.HubInstallNamespace, Name: "custom-placement-agent"}, &foundAgent)
		require.NoError(t, err)
		assert.Equal(t, prometheusImage, *foundAgent.Spec.Image)
		assert.True(t, foundAgent.Spec.ArbitraryFSAccessThroughSMs.Deny)
	})

	t.Run("unrecognized agent is ignored", func(t *testing.T) {
		cmao := newCMAO(globalPlacement)
		opts := newAddonOptions(true, false)

		// Agent has matching labels but no owner ref and no part-of label
		unrecognizedAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, "rogue-agent", false)
		unrecognizedAgent.Annotations = map[string]string{
			addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/global",
		}

		scheme := newTestScheme()
		fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(cmao, unrecognizedAgent).Build()
		d := DefaultStackResources{
			Client:             fakeClient,
			CMAO:               cmao,
			AddonOptions:       opts,
			Logger:             klog.Background(),
			KubeRBACProxyImage: kubeRbacImage,
			PrometheusImage:    prometheusImage,
		}

		configs, err := d.reconcileAgents(context.Background(), false)
		require.NoError(t, err)

		// The unrecognized agent should NOT generate any configs
		for _, cfg := range configs {
			assert.NotEqual(t, "rogue-agent", cfg.Config.Name,
				"unrecognized agent should not produce configs")
		}

		// The unrecognized agent should NOT have SSA applied to it
		foundAgent := cooprometheusv1alpha1.PrometheusAgent{}
		err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: config.HubInstallNamespace, Name: "rogue-agent"}, &foundAgent)
		require.NoError(t, err)
		assert.Nil(t, foundAgent.Spec.Image, "unrecognized agent should not have image overwritten")

		// CreateDefaultAgent should also not consider the unrecognized agent as covering global
		// so a default global agent should have been created
		agents := cooprometheusv1alpha1.PrometheusAgentList{}
		err = fakeClient.List(context.Background(), &agents)
		require.NoError(t, err)
		defaultFound := false
		for _, a := range agents.Items {
			if a.Name == makeAgentName(config.PlatformMetricsCollectorApp, "global")+"-default" {
				defaultFound = true
			}
		}
		assert.True(t, defaultFound, "default global agent should be created since unrecognized agent is ignored")
	})
}

func TestReconcileAgentWithRegistries(t *testing.T) {
	globalPlacement := addonv1beta1.PlacementStrategy{
		PlacementRef: addonv1beta1.PlacementRef{Name: "global", Namespace: config.HubInstallNamespace},
	}
	cmao := newCMAO(globalPlacement)
	registries := []addonv1beta1.ImageMirror{
		{
			Source: "quay.io/prometheus/prometheus",
			Mirror: "my-registry.com/prometheus/prometheus",
		},
		{
			Source: "quay.io/kube/rbac-proxy",
			Mirror: "my-registry.com/kube/rbac-proxy",
		},
	}
	opts := newAddonOptions(true, true)
	opts.Registries = registries

	baseImages := map[string]string{
		"prometheus_config_reloader":    "quay.io/prometheus/config-reloader",
		"kube_rbac_proxy":               "quay.io/kube/rbac-proxy",
		"obo_prometheus_rhel9_operator": "quay.io/prometheus/obo-operator",
		"kube_state_metrics":            "quay.io/kube/kube-state-metrics",
		"node_exporter":                 "quay.io/kube/node-exporter",
		"prometheus":                    "quay.io/prometheus/prometheus",
		"endpoint_monitoring_operator":  "quay.io/stolostron/endpoint-monitoring-operator",
	}

	imagesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ImagesConfigMapObjKey.Name,
			Namespace: config.ImagesConfigMapObjKey.Namespace,
		},
		Data: baseImages,
	}

	scheme := newTestScheme()
	fakeClient := fake.NewClientBuilder().
		WithInterceptorFuncs(ensureGVKIsSet(scheme)).
		WithScheme(scheme).
		WithObjects(cmao, imagesCM).
		Build()

	images, err := config.GetImageOverrides(context.Background(), fakeClient, opts.Registries, klog.Background())
	require.NoError(t, err)

	d := DefaultStackResources{
		Client:             fakeClient,
		CMAO:               cmao,
		AddonOptions:       opts,
		Logger:             klog.Background(),
		KubeRBACProxyImage: images.KubeRBACProxy,
		PrometheusImage:    images.Prometheus,
	}

	configs, err := d.reconcileAgents(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, configs, 1)

	foundAgent := cooprometheusv1alpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: configs[0].Config.Namespace, Name: configs[0].Config.Name}, &foundAgent)
	require.NoError(t, err)

	// Check overridden images
	assert.Equal(t, "my-registry.com/prometheus/prometheus", *foundAgent.Spec.Image)
	assert.Equal(t, "my-registry.com/kube/rbac-proxy", foundAgent.Spec.Containers[0].Image)
}

func TestReconcile(t *testing.T) {
	globalPlacementRef := addonv1beta1.PlacementRef{
		Namespace: config.HubInstallNamespace,
		Name:      "global",
	}
	hubUrl, err := url.Parse("https://test.com")
	require.NoError(t, err)

	platformAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, "existing-platform-agent", false)
	platformAgent.Labels[addoncfg.PartOfK8sLabelKey] = addoncfg.Name
	platformAgent.Annotations = map[string]string{
		addoncfg.PlacementAnnotationKey: config.HubInstallNamespace + "/global",
	}

	platformSC := &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.HubInstallNamespace,
			Name:      "platform",
			Labels:    config.PlatformPrometheusMatchLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("mco-operator"),
					Controller: ptr.To(true),
				},
			},
		},
	}
	uwlSC := &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.HubInstallNamespace,
			Name:      "uwl",
			Labels:    config.UserWorkloadPrometheusMatchLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("mco-operator"),
					Controller: ptr.To(true),
				},
			},
		},
	}
	hostedCluster := &hyperv1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hcp",
			Namespace: "hcpns",
		},
	}
	hcpApiserverSC := &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.HubInstallNamespace,
			Name:      "apiserver",
			Labels:    config.ApiserverHcpUserWorkloadPrometheusMatchLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("mco-operator"),
					Controller: ptr.To(true),
				},
			},
		},
	}
	hcpApiserverRule := &prometheusv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind: prometheusv1.PrometheusRuleKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.HubInstallNamespace,
			Name:      "hcp-apiserver-rule",
			Labels:    config.ApiserverHcpUserWorkloadPrometheusMatchLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("mco-operator"),
					Controller: ptr.To(true),
				},
			},
		},
	}
	hcpEtcdSC := &cooprometheusv1alpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.HubInstallNamespace,
			Name:      "etcd",
			Labels:    config.EtcdHcpUserWorkloadPrometheusMatchLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("mco-operator"),
					Controller: ptr.To(true),
				},
			},
		},
	}
	hcpEtcdRule := &prometheusv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			Kind: prometheusv1.PrometheusRuleKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.HubInstallNamespace,
			Name:      "hcp-etcd-rule",
			Labels:    config.EtcdHcpUserWorkloadPrometheusMatchLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("mco-operator"),
					Controller: ptr.To(true),
				},
			},
		},
	}

	testCases := []struct {
		name               string
		initialPlacements  []addonv1beta1.PlacementStrategy
		initObjs           []client.Object
		platformEnabled    bool
		uwlEnabled         bool
		expectAgentsCount  int
		expectConfigsCount int
	}{
		{
			name:              "no placement with disabled monitoring",
			expectAgentsCount: 0,
		},
		{
			name: "global placement with disabled monitoring",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{},
					PlacementRef: globalPlacementRef,
				},
			},
			initObjs:           []client.Object{platformAgent, platformSC},
			expectAgentsCount:  1,
			expectConfigsCount: 0,
		},
		{
			name: "global placement with enabled platform and uwl monitoring",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{},
					PlacementRef: globalPlacementRef,
				},
			},
			platformEnabled:    true,
			uwlEnabled:         true,
			initObjs:           []client.Object{platformSC, uwlSC},
			expectAgentsCount:  2, // one default platform agent + one default uwl agent
			expectConfigsCount: 4, // platform agent + uwl agent + platformSC + uwlSC
		},
		{
			name: "global placement with enabled monitoring and hcp",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{},
					PlacementRef: globalPlacementRef,
				},
			},
			platformEnabled: true,
			uwlEnabled:      true,
			initObjs: []client.Object{
				hostedCluster, hcpApiserverSC, hcpEtcdSC, hcpApiserverRule, hcpEtcdRule,
			},
			expectAgentsCount:  2, // one default platform agent + one default uwl agent
			expectConfigsCount: 6, // platform agent + uwl agent + 2 hcp scrapeConfigs + 2 hcp rules
		},
		{
			name: "global placement with existing platform agent covering global",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{},
					PlacementRef: globalPlacementRef,
				},
			},
			platformEnabled:    true,
			uwlEnabled:         true,
			initObjs:           []client.Object{platformAgent, platformSC, uwlSC},
			expectAgentsCount:  3, // existing platform agent + dummy platform agent + default uwl agent
			expectConfigsCount: 4, // existing platform agent (targets global) + uwl agent + platformSC + uwlSC
		},
		{
			name: "global placement with enabled platform only",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{},
					PlacementRef: globalPlacementRef,
				},
			},
			platformEnabled:    true,
			uwlEnabled:         false,
			initObjs:           []client.Object{platformSC},
			expectAgentsCount:  1, // one default platform agent
			expectConfigsCount: 2, // platform agent + platformSC
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmao := newCMAO(tc.initialPlacements...)
			addonOptions := addon.Options{
				Platform: addon.PlatformOptions{
					Metrics: addon.MetricsOptions{
						CollectionEnabled: tc.platformEnabled,
						HubEndpoint:       *hubUrl,
					},
				},
				UserWorkloads: addon.UserWorkloadOptions{
					Metrics: addon.MetricsOptions{
						CollectionEnabled: tc.uwlEnabled,
					},
				},
			}
			initObjs := append(tc.initObjs, cmao)
			scheme := newTestScheme()
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				Client:             fakeClient,
				CMAO:               cmao,
				Logger:             klog.Background(),
				AddonOptions:       addonOptions,
				KubeRBACProxyImage: "dummy",
			}

			dc, err := d.Reconcile(context.Background())
			require.NoError(t, err)
			err = common.EnsureAddonConfig(context.Background(), klog.Background(), fakeClient, dc)
			require.NoError(t, err)

			foundAgents := cooprometheusv1alpha1.PrometheusAgentList{}
			err = fakeClient.List(context.Background(), &foundAgents)
			require.NoError(t, err)
			assert.Len(t, foundAgents.Items, tc.expectAgentsCount)

			err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(cmao), cmao)
			require.NoError(t, err)

			for _, placement := range cmao.Spec.InstallStrategy.Placements {
				assert.Len(t, placement.Configs, tc.expectConfigsCount)
			}
		})
	}
}

func TestReconcileScrapeConfigs(t *testing.T) {
	mcoUID := types.UID("mco")
	mcoOwnerRef := metav1.OwnerReference{
		UID:        mcoUID,
		Controller: ptr.To(true),
	}
	placementRefA := addonv1beta1.PlacementRef{
		Namespace: "ns",
		Name:      "a",
	}
	testCases := []struct {
		name              string
		initObjs          []client.Object
		isUWL             bool
		hasHostedClusters bool
		expects           func(*testing.T, []cooprometheusv1alpha1.ScrapeConfig)
	}{
		{
			name: "no scrape configs",
		},
		{
			name: "unmanaged SC is ignored",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "test",
						Labels:    config.PlatformPrometheusMatchLabels,
					},
				},
			},
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Empty(t, objs)
			},
		},
		{
			name: "SC target is enforced for platform",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.PlatformPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
					Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
						ScrapeClassName: ptr.To("invalid"),
					},
				},
			},
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Len(t, objs, 1)
				assert.Equal(t, "not-configurable", *objs[0].Spec.ScrapeClassName)
				assert.Contains(t, objs[0].Labels, addoncfg.BackupLabelKey, "backup label key should be present")
				assert.Equal(t, addoncfg.BackupLabelValue, objs[0].Labels[addoncfg.BackupLabelKey])
			},
		},
		{
			name: "SC target is left for uwl",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.UserWorkloadPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
					Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
						ScrapeClassName: ptr.To("custom"),
						StaticConfigs: []cooprometheusv1alpha1.StaticConfig{
							{
								Targets: []cooprometheusv1alpha1.Target{"test"},
							},
						},
					},
				},
			},
			isUWL: true,
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Len(t, objs, 1)
				assert.Equal(t, "custom", *objs[0].Spec.ScrapeClassName)
				assert.Contains(t, objs[0].Labels, addoncfg.BackupLabelKey, "backup label key should be present")
				assert.Equal(t, addoncfg.BackupLabelValue, objs[0].Labels[addoncfg.BackupLabelKey])
			},
		},
		{
			name: "user-defined SC with single placement annotation",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-sc",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: "ns/a",
						},
					},
				},
			},
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Len(t, objs, 1)
				assert.Equal(t, "user-sc", objs[0].Name)
				assert.Contains(t, objs[0].Labels, addoncfg.BackupLabelKey)
				assert.Equal(t, addoncfg.BackupLabelValue, objs[0].Labels[addoncfg.BackupLabelKey])
			},
		},
		{
			name: "user-defined SC with multiple placement annotations",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-sc-multi",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: "ns/a,ns/b",
						},
					},
				},
			},
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Len(t, objs, 2)
			},
		},
		{
			name: "user-defined SC without placement annotation is skipped",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-sc-no-annotation",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
					},
				},
			},
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Empty(t, objs)
			},
		},
		{
			name: "user-defined SC coexists with MCO-managed SC",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "mco-sc",
						Labels:          config.PlatformPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-sc",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: "ns/a",
						},
					},
				},
			},
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Len(t, objs, 2)
				names := []string{objs[0].Name, objs[1].Name}
				assert.Contains(t, names, "mco-sc")
				assert.Contains(t, names, "user-sc")
			},
		},
		{
			name: "hcp SC is handled",
			initObjs: []client.Object{
				&cooprometheusv1alpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: cooprometheusv1alpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.EtcdHcpUserWorkloadPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
			},
			isUWL:             true,
			hasHostedClusters: true,
			expects: func(t *testing.T, objs []cooprometheusv1alpha1.ScrapeConfig) {
				assert.Len(t, objs, 1)
				assert.Empty(t, objs[0].Spec.ScrapeClassName)
				assert.Contains(t, objs[0].Labels, addoncfg.BackupLabelKey, "backup label key should be present")
				assert.Equal(t, addoncfg.BackupLabelValue, objs[0].Labels[addoncfg.BackupLabelKey])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmao := newCMAO(addonv1beta1.PlacementStrategy{
				Configs:      []addonv1beta1.AddOnConfig{},
				PlacementRef: placementRefA,
			})
			initObjs := append(tc.initObjs, cmao)
			scheme := newTestScheme()
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				CMAO:               cmao,
				Client:             fakeClient,
				Logger:             klog.Background(),
				KubeRBACProxyImage: "dummy",
			}

			dc, err := d.reconcileScrapeConfigs(context.Background(), mcoUID, tc.isUWL, tc.hasHostedClusters)
			require.NoError(t, err)

			scrapeConfigs := []cooprometheusv1alpha1.ScrapeConfig{}
			for _, config := range dc {
				sc := cooprometheusv1alpha1.ScrapeConfig{}
				err = fakeClient.Get(context.Background(), client.ObjectKey(config.Config.ConfigReferent), &sc)
				require.NoError(t, err)
				scrapeConfigs = append(scrapeConfigs, sc)
			}
			if tc.expects != nil {
				tc.expects(t, scrapeConfigs)
			}
		})
	}
}

func TestGetPrometheusRules(t *testing.T) {
	mcoUID := types.UID("mco")
	mcoOwnerRef := metav1.OwnerReference{
		UID:        mcoUID,
		Controller: ptr.To(true),
	}
	placementRefA := addonv1beta1.PlacementRef{
		Namespace: "ns",
		Name:      "a",
	}
	testCases := []struct {
		name              string
		initObjs          []client.Object
		platformEnabled   bool
		uwlEnabled        bool
		hasHostedClusters bool
		expects           func(*testing.T, []prometheusv1.PrometheusRule)
	}{
		{
			name: "no rule",
		},
		{
			name: "unmanaged rule is ignored",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "test",
						Labels:    config.PlatformPrometheusMatchLabels,
					},
				},
			},
			platformEnabled: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Empty(t, objs)
			},
		},
		{
			name: "disabled rules are ignored",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.PlatformPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test-uwl",
						Labels:          config.UserWorkloadPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
			},
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Empty(t, objs)
			},
		},
		{
			name: "platform rule is fetched",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.PlatformPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
			},
			platformEnabled: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Len(t, objs, 1)
			},
		},
		{
			name: "uwl rule is fetched",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.UserWorkloadPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
			},
			uwlEnabled: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Len(t, objs, 1)
			},
		},
		{
			name: "user-defined rule with single placement annotation",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-rule",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: "ns/a",
						},
					},
				},
			},
			platformEnabled: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Len(t, objs, 1)
				assert.Equal(t, "user-rule", objs[0].Name)
			},
		},
		{
			name: "user-defined rule with multiple placement annotations",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-rule-multi",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: "ns/a,ns/b",
						},
					},
				},
			},
			platformEnabled: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Len(t, objs, 2)
			},
		},
		{
			name: "user-defined rule without placement annotation is skipped",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-rule-no-annotation",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
					},
				},
			},
			platformEnabled: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Empty(t, objs)
			},
		},
		{
			name: "user-defined rule coexists with MCO-managed rule",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "mco-rule",
						Labels:          config.PlatformPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "user-rule",
						Labels: map[string]string{
							addoncfg.ComponentK8sLabelKey: config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
							addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
						},
						Annotations: map[string]string{
							addoncfg.PlacementAnnotationKey: "ns/a",
						},
					},
				},
			},
			platformEnabled: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Len(t, objs, 2)
				names := []string{objs[0].Name, objs[1].Name}
				assert.Contains(t, names, "mco-rule")
				assert.Contains(t, names, "user-rule")
			},
		},
		{
			name: "hcp rules are fetched",
			initObjs: []client.Object{
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "etcd",
						Labels:          config.EtcdHcpUserWorkloadPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
				&prometheusv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusv1.PrometheusRuleKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "apiserver",
						Labels:          config.ApiserverHcpUserWorkloadPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
				},
			},
			uwlEnabled:        true,
			platformEnabled:   true,
			hasHostedClusters: true,
			expects: func(t *testing.T, objs []prometheusv1.PrometheusRule) {
				assert.Len(t, objs, 2)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmao := newCMAO(addonv1beta1.PlacementStrategy{
				Configs:      []addonv1beta1.AddOnConfig{},
				PlacementRef: placementRefA,
			})
			initObjs := append(tc.initObjs, cmao)
			scheme := newTestScheme()
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(ensureGVKIsSet(scheme)).WithScheme(scheme).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				CMAO:               cmao,
				Client:             fakeClient,
				Logger:             klog.Background(),
				KubeRBACProxyImage: "dummy",
				AddonOptions: addon.Options{
					Platform: addon.PlatformOptions{
						Metrics: addon.MetricsOptions{
							CollectionEnabled: tc.platformEnabled,
						},
					},
					UserWorkloads: addon.UserWorkloadOptions{
						Metrics: addon.MetricsOptions{
							CollectionEnabled: tc.uwlEnabled,
						},
					},
				},
			}

			dc, err := d.getPrometheusRules(context.Background(), mcoUID, tc.hasHostedClusters)
			require.NoError(t, err)

			rules := []prometheusv1.PrometheusRule{}
			for _, config := range dc {
				rule := prometheusv1.PrometheusRule{}
				err = fakeClient.Get(context.Background(), client.ObjectKey(config.Config.ConfigReferent), &rule)
				require.NoError(t, err)
				rules = append(rules, rule)
			}
			if tc.expects != nil {
				tc.expects(t, rules)
			}
		})
	}
}

func TestGeneratePlacementRefs(t *testing.T) {
	d := DefaultStackResources{}

	testCases := []struct {
		name        string
		annotations string
		expected    []addonv1beta1.PlacementRef
		expectErr   bool
	}{
		{
			name:        "empty string returns nil",
			annotations: "",
			expected:    nil,
		},
		{
			name:        "single placement",
			annotations: "ns-a/placement-a",
			expected: []addonv1beta1.PlacementRef{
				{Namespace: "ns-a", Name: "placement-a"},
			},
		},
		{
			name:        "multiple placements",
			annotations: "ns-a/placement-a,ns-b/placement-b",
			expected: []addonv1beta1.PlacementRef{
				{Namespace: "ns-a", Name: "placement-a"},
				{Namespace: "ns-b", Name: "placement-b"},
			},
		},
		{
			name:        "trailing comma is tolerated",
			annotations: "ns-a/placement-a,",
			expected: []addonv1beta1.PlacementRef{
				{Namespace: "ns-a", Name: "placement-a"},
			},
		},
		{
			name:        "missing separator returns error",
			annotations: "no-separator",
			expectErr:   true,
		},
		{
			name:        "empty namespace returns error",
			annotations: "/placement-a",
			expectErr:   true,
		},
		{
			name:        "empty name returns error",
			annotations: "ns-a/",
			expectErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			refs, err := d.generatePlacementRefs(tc.annotations)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, refs)
		})
	}
}

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = kubescheme.AddToScheme(s)
	_ = addonv1beta1.Install(s)
	_ = cooprometheusv1alpha1.AddToScheme(s)
	_ = cooprometheusv1.AddToScheme(s)
	_ = prometheusv1.AddToScheme(s)
	_ = hyperv1.AddToScheme(s)
	return s
}

func newCMAO(placements ...addonv1beta1.PlacementStrategy) *addonv1beta1.ClusterManagementAddOn {
	return &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  types.UID("test-cmao-uid"),
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("mco-operator"), // needed to identify scrapeConfigs owned by mco
					Controller: ptr.To(true),
				},
			},
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Placements: placements,
			},
		},
	}
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

func ensureGVKIsSet(scheme *runtime.Scheme) interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(ctx context.Context, clientww client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			err := clientww.Get(ctx, key, obj, opts...)
			if err != nil {
				return err
			}
			gvk, err := apiutil.GVKForObject(obj, scheme)
			if err == nil {
				obj.GetObjectKind().SetGroupVersionKind(gvk)
			}
			return nil
		},
		Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			gvk, _ := apiutil.GVKForObject(obj, scheme)
			if !gvk.Empty() {
				obj.GetObjectKind().SetGroupVersionKind(gvk)
			}
			err := clientww.Patch(ctx, obj, patch, opts...)
			if err == nil && !gvk.Empty() {
				obj.GetObjectKind().SetGroupVersionKind(gvk)
			}
			return err
		},
		List: func(ctx context.Context, clientww client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
			err := clientww.List(ctx, obj, opts...)
			if err != nil {
				return err
			}
			return meta.EachListItem(obj, func(object runtime.Object) error {
				gvk, err := apiutil.GVKForObject(object, scheme)
				if err != nil {
					return nil
				}
				object.GetObjectKind().SetGroupVersionKind(gvk)
				return nil
			})
		},
	}
}
