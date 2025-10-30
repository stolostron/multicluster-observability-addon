package resource

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestGetOrCreateDefaultAgent(t *testing.T) {
	cmao := newCMAO()

	// Existing Platform Agent
	placementRef := addonv1alpha1.PlacementRef{
		Name:      "global",
		Namespace: config.HubInstallNamespace,
	}
	platformAppName := config.PlatformMetricsCollectorApp
	existingPlatformAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(platformAppName, placementRef.Name), false, placementRef)
	assert.NoError(t, controllerutil.SetControllerReference(cmao, existingPlatformAgent, newTestScheme()))

	// Existing UWL Agent
	uwlAppName := config.UserWorkloadMetricsCollectorApp
	existingUWLAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(uwlAppName, placementRef.Name), true, placementRef)
	assert.NoError(t, controllerutil.SetControllerReference(cmao, existingUWLAgent, newTestScheme()))

	testCases := []struct {
		name         string
		placementRef addonv1alpha1.PlacementRef
		isUWL        bool
		initObjs     []client.Object
	}{
		{
			name: "creates platform agent",
			placementRef: addonv1alpha1.PlacementRef{
				Name:      "my-placement",
				Namespace: config.HubInstallNamespace,
			},
			isUWL:    false,
			initObjs: []client.Object{cmao},
		},
		{
			name:         "updates platform agent",
			placementRef: placementRef,
			isUWL:        false,
			initObjs:     []client.Object{cmao, existingPlatformAgent},
		},
		{
			name: "creates uwl agent",
			placementRef: addonv1alpha1.PlacementRef{
				Name:      "my-placement",
				Namespace: config.HubInstallNamespace,
			},
			isUWL:    true,
			initObjs: []client.Object{cmao},
		},
		{
			name:         "updates uwl agent",
			placementRef: placementRef,
			isUWL:        true,
			initObjs:     []client.Object{cmao, existingUWLAgent},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(tc.initObjs...).Build()
			d := DefaultStackResources{
				Client: fakeClient,
				CMAO:   cmao,
				Logger: klog.Background(),
			}

			gotAgent, err := d.getOrCreateDefaultAgent(context.Background(), tc.placementRef, tc.isUWL)
			assert.NoError(t, err)
			assert.True(t, controllerutil.HasControllerReference(gotAgent))

			// ensure there is only one agent
			if err == nil {
				res := &cooprometheusv1alpha1.PrometheusAgentList{}
				err := fakeClient.List(context.Background(), res)
				assert.NoError(t, err)
				assert.Len(t, res.Items, 1)
			}

			// ensure correct labels are set on the agent
			if tc.isUWL {
				assert.Equal(t, gotAgent.Labels[addoncfg.ComponentK8sLabelKey], config.UserWorkloadPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])
			} else {
				assert.Equal(t, gotAgent.Labels[addoncfg.ComponentK8sLabelKey], config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])
			}
		})
	}
}

func TestReconcileAgent(t *testing.T) {
	cmao := newCMAO()
	opts := newAddonOptions(true, true)
	kubeRbacImage := "kube-rbac-proxy:version"
	prometheusImage := "prometheus:version"
	placementRef := addonv1alpha1.PlacementRef{Name: "my-placement", Namespace: "my-namespace"}

	// Dynamic fake client doesn't support apply types of patch. This is overridden with an interceptor toward a
	// merge type patch that has no unwanted effect for this unit test.
	patchCalls := 0
	fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			patchCalls++

			// Preserve TypeMeta before patch operation
			var originalTypeMeta metav1.TypeMeta

			// Set obj type if missing using type assertions
			if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
				originalTypeMeta = pa.TypeMeta
				if pa.GroupVersionKind().Kind == "" {
					pa.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
				}
			} else if sc, ok := obj.(*cooprometheusv1alpha1.ScrapeConfig); ok {
				originalTypeMeta = sc.TypeMeta
				if sc.GroupVersionKind().Kind == "" {
					sc.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.ScrapeConfigsKind))
				}
			} else if pr, ok := obj.(*prometheusv1.PrometheusRule); ok {
				originalTypeMeta = pr.TypeMeta
				if pr.GroupVersionKind().Kind == "" {
					pr.SetGroupVersionKind(prometheusv1.SchemeGroupVersion.WithKind(prometheusv1.PrometheusRuleKind))
				}
			}

			// Filter out SSA-specific options that are incompatible with merge patches
			var filteredOpts []client.PatchOption
			for _, opt := range opts {
				// Skip SSA-specific options by checking their string representation
				optStr := fmt.Sprintf("%T", opt)
				if strings.Contains(optStr, "forceOwnership") || strings.Contains(optStr, "FieldOwner") {
					continue // Skip SSA-specific options
				}
				filteredOpts = append(filteredOpts, opt) // Keep all other options
			}

			err := clientww.Patch(ctx, obj, client.Merge, filteredOpts...)

			// Restore TypeMeta after patch operation
			if err == nil && originalTypeMeta.Kind != "" {
				if pa, ok := obj.(*cooprometheusv1alpha1.PrometheusAgent); ok {
					pa.TypeMeta = originalTypeMeta
				} else if sc, ok := obj.(*cooprometheusv1alpha1.ScrapeConfig); ok {
					sc.TypeMeta = originalTypeMeta
				} else if pr, ok := obj.(*prometheusv1.PrometheusRule); ok {
					pr.TypeMeta = originalTypeMeta
				}
			}

			return err
		},
	}).WithScheme(newTestScheme()).WithObjects(cmao).Build()
	d := DefaultStackResources{
		Client:             fakeClient,
		CMAO:               cmao,
		AddonOptions:       opts,
		Logger:             klog.Background(),
		KubeRBACProxyImage: kubeRbacImage,
		PrometheusImage:    prometheusImage,
	}

	// >>> Platform agent
	retAgent, err := d.reconcileAgentForPlacement(context.Background(), placementRef, false)
	assert.NoError(t, err)

	foundAgent := cooprometheusv1alpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: retAgent.Config.Namespace, Name: retAgent.Config.Name}, &foundAgent)
	assert.NoError(t, err)

	// Check default fields
	assert.EqualValues(t, 1, *foundAgent.Spec.Replicas)
	// Check ssa fields
	assert.Equal(t, kubeRbacImage, foundAgent.Spec.Containers[0].Image)
	assert.Equal(t, prometheusImage, *foundAgent.Spec.Image)
	assert.Equal(t, config.PlatformMetricsCollectorApp, foundAgent.Spec.ServiceAccountName)
	// Check placement labels
	assert.Equal(t, foundAgent.Labels[addoncfg.PlacementRefNameLabelKey], placementRef.Name)
	// Check platform specific values: appName and ScrapeConfigNamespaceSelector
	assert.Equal(t, foundAgent.Spec.ServiceAccountName, config.PlatformMetricsCollectorApp)
	assert.Nil(t, foundAgent.Spec.ScrapeConfigNamespaceSelector)

	// Subsequent reconcile does not trigger update
	previousPatchCalls := patchCalls
	_, err = d.reconcileAgentForPlacement(context.Background(), placementRef, false)
	assert.NoError(t, err)
	assert.Equal(t, previousPatchCalls, patchCalls)

	// >>> UWL agent
	retAgent, err = d.reconcileAgentForPlacement(context.Background(), placementRef, true)
	assert.NoError(t, err)

	foundAgent = cooprometheusv1alpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: retAgent.Config.Namespace, Name: retAgent.Config.Name}, &foundAgent)
	assert.NoError(t, err)

	// Check uwl specific values: appName and ScrapeConfigNamespaceSelector
	assert.Equal(t, foundAgent.Spec.ServiceAccountName, config.UserWorkloadMetricsCollectorApp)
	assert.Equal(t, foundAgent.Spec.ScrapeConfigNamespaceSelector, &metav1.LabelSelector{})
}

func TestReconcile(t *testing.T) {
	placementRefA := addonv1alpha1.PlacementRef{
		Namespace: "ns",
		Name:      "a",
	}
	placementRefB := addonv1alpha1.PlacementRef{
		Namespace: "ns",
		Name:      "b",
	}
	hubUrl, err := url.Parse("https://test.com")
	require.NoError(t, err)

	platformAppName := "platform-app"
	platformAgent := NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(platformAppName, placementRefA.Name), false, placementRefA)
	platformAgent.Labels = map[string]string{ // expected labels for identifying an agent configuration
		addoncfg.PlacementRefNamespaceLabelKey: placementRefA.Namespace,
		addoncfg.PlacementRefNameLabelKey:      placementRefA.Name,
		addoncfg.ManagedByK8sLabelKey:          addoncfg.Name,
		addoncfg.ComponentK8sLabelKey:          config.PlatformMetricsCollectorApp,
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
		initialPlacements  []addonv1alpha1.PlacementStrategy
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
			name: "one placement with disabled monitoring",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefA,
				},
			},
			initObjs:           []client.Object{platformAgent, platformSC},
			expectAgentsCount:  1, // exists in init objects but is ignored for the configs
			expectConfigsCount: 0,
		},
		{
			name: "one placement with enabled monitoring",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefA,
				},
			},
			platformEnabled:    true,
			uwlEnabled:         true,
			initObjs:           []client.Object{platformAgent, platformSC, uwlSC},
			expectAgentsCount:  2,
			expectConfigsCount: 4, // platform and uwl agents + scrapeConfigs
		},
		{
			name: "one placement with enabled monitoring and hcp",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefA,
				},
			},
			platformEnabled: true,
			uwlEnabled:      true,
			initObjs: []client.Object{
				hostedCluster, hcpApiserverSC, hcpEtcdSC, hcpApiserverRule, hcpEtcdRule,
			},
			expectAgentsCount:  2,
			expectConfigsCount: 6, // platform and uwl agents + 4 hcp scrapeConfigs and rules
		},
		{
			name: "one placement with enabled monitoring",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefA,
				},
			},
			platformEnabled:    true,
			uwlEnabled:         true,
			initObjs:           []client.Object{platformAgent, platformSC, uwlSC},
			expectAgentsCount:  2,
			expectConfigsCount: 4, // platform and uwl agents + scrapeConfigs
		},
		{
			name: "two placements with enabled platform monitoring",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefA,
				},
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefB,
				},
			},
			platformEnabled:    true,
			uwlEnabled:         false,
			initObjs:           []client.Object{platformAgent, platformSC, uwlSC},
			expectAgentsCount:  2, // one platform agent for each placement
			expectConfigsCount: 2, // platform agent and scrapeConfig
		},
		{
			name: "two placements with enabled monitoring",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefA,
				},
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefB,
				},
			},
			platformEnabled:    true,
			uwlEnabled:         true,
			initObjs:           []client.Object{platformAgent, platformSC, uwlSC},
			expectAgentsCount:  4, // 2 agent for each placement
			expectConfigsCount: 4, // 2 agents and 2 scs
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
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					// Preserve TypeMeta before patch operation
					var originalTypeMeta metav1.TypeMeta

					// Set obj type if missing using type assertions
					switch o := obj.(type) {
					case *cooprometheusv1alpha1.PrometheusAgent:
						originalTypeMeta = o.TypeMeta
						if o.GroupVersionKind().Kind == "" {
							o.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
						}
					case *cooprometheusv1alpha1.ScrapeConfig:
						originalTypeMeta = o.TypeMeta
						if o.GroupVersionKind().Kind == "" {
							o.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.ScrapeConfigsKind))
						}
					case *prometheusv1.PrometheusRule:
						originalTypeMeta = o.TypeMeta
						if o.GroupVersionKind().Kind == "" {
							o.SetGroupVersionKind(prometheusv1.SchemeGroupVersion.WithKind(prometheusv1.PrometheusRuleKind))
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

					// Restore TypeMeta after patch operation
					if err == nil {
						switch o := obj.(type) {
						case *cooprometheusv1alpha1.PrometheusAgent:
							o.TypeMeta = originalTypeMeta
						case *cooprometheusv1alpha1.ScrapeConfig:
							o.TypeMeta = originalTypeMeta
						case *prometheusv1.PrometheusRule:
							o.TypeMeta = originalTypeMeta
						}
					}

					return err
				},
				List: func(ctx context.Context, clientww client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
					err := clientww.List(ctx, obj, opts...)
					if err != nil {
						return err
					}

					// Set obj type if missing using type assertions
					switch list := obj.(type) {
					case *cooprometheusv1alpha1.PrometheusAgentList:
						for i := range list.Items {
							if list.Items[i].GroupVersionKind().Kind == "" {
								list.Items[i].SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.PrometheusAgentsKind))
							}
						}
					case *cooprometheusv1alpha1.ScrapeConfigList:
						for i := range list.Items {
							if list.Items[i].GroupVersionKind().Kind == "" {
								list.Items[i].SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.ScrapeConfigsKind))
							}
						}
					case *prometheusv1.PrometheusRuleList:
						for i := range list.Items {
							if list.Items[i].GroupVersionKind().Kind == "" {
								list.Items[i].SetGroupVersionKind(prometheusv1.SchemeGroupVersion.WithKind(prometheusv1.PrometheusRuleKind))
							}
						}
					}
					return nil
				},
			}).WithScheme(newTestScheme()).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				Client:             fakeClient,
				CMAO:               cmao,
				Logger:             klog.Background(),
				AddonOptions:       addonOptions,
				KubeRBACProxyImage: "dummy",
			}

			dc, err := d.Reconcile(context.Background())
			assert.NoError(t, err)
			err = common.EnsureAddonConfig(context.Background(), klog.Background(), fakeClient, dc)
			assert.NoError(t, err)

			foundAgents := cooprometheusv1alpha1.PrometheusAgentList{}
			err = fakeClient.List(context.Background(), &foundAgents)
			assert.NoError(t, err)
			assert.Len(t, foundAgents.Items, tc.expectAgentsCount)

			err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(cmao), cmao)
			assert.NoError(t, err)

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
	placementRefA := addonv1alpha1.PlacementRef{
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
				assert.Len(t, objs, 0)
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
				assert.Equal(t, *objs[0].Spec.ScrapeClassName, "custom")
				assert.Contains(t, objs[0].Labels, addoncfg.BackupLabelKey, "backup label key should be present")
				assert.Equal(t, addoncfg.BackupLabelValue, objs[0].Labels[addoncfg.BackupLabelKey])
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
			cmao := newCMAO(addonv1alpha1.PlacementStrategy{
				Configs:      []addonv1alpha1.AddOnConfig{},
				PlacementRef: placementRefA,
			})
			initObjs := append(tc.initObjs, cmao)
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					// Preserve TypeMeta before patch operation
					var originalTypeMeta metav1.TypeMeta

					// Set obj type if missing using type assertions
					if sc, ok := obj.(*cooprometheusv1alpha1.ScrapeConfig); ok {
						originalTypeMeta = sc.TypeMeta
						if sc.GroupVersionKind().Kind == "" {
							sc.SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.ScrapeConfigsKind))
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

					// Restore TypeMeta after patch operation
					if err == nil && originalTypeMeta.Kind != "" {
						if sc, ok := obj.(*cooprometheusv1alpha1.ScrapeConfig); ok {
							sc.TypeMeta = originalTypeMeta
						}
					}

					return err
				},
				List: func(ctx context.Context, clientww client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
					err := clientww.List(ctx, obj, opts...)
					if err != nil {
						return err
					}
					// Ensure GVK is set for objects in lists using type assertions
					if scList, ok := obj.(*cooprometheusv1alpha1.ScrapeConfigList); ok {
						for i := range scList.Items {
							if scList.Items[i].GroupVersionKind().Kind == "" {
								scList.Items[i].SetGroupVersionKind(cooprometheusv1alpha1.SchemeGroupVersion.WithKind(cooprometheusv1alpha1.ScrapeConfigsKind))
							}
						}
					}
					return nil
				},
			}).WithScheme(newTestScheme()).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				CMAO:               cmao,
				Client:             fakeClient,
				Logger:             klog.Background(),
				KubeRBACProxyImage: "dummy",
			}

			dc, err := d.reconcileScrapeConfigs(context.Background(), mcoUID, tc.isUWL, tc.hasHostedClusters)
			assert.NoError(t, err)

			scrapeConfigs := []cooprometheusv1alpha1.ScrapeConfig{}
			for _, config := range dc {
				sc := cooprometheusv1alpha1.ScrapeConfig{}
				err = fakeClient.Get(context.Background(), client.ObjectKey(config.Config.ConfigReferent), &sc)
				assert.NoError(t, err)
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
	placementRefA := addonv1alpha1.PlacementRef{
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
				assert.Len(t, objs, 0)
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
				assert.Len(t, objs, 0)
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
			cmao := newCMAO(addonv1alpha1.PlacementStrategy{
				Configs:      []addonv1alpha1.AddOnConfig{},
				PlacementRef: placementRefA,
			})
			initObjs := append(tc.initObjs, cmao)
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					// Preserve TypeMeta before patch operation
					var originalTypeMeta metav1.TypeMeta

					// Set obj type if missing using type assertions
					if pr, ok := obj.(*prometheusv1.PrometheusRule); ok {
						originalTypeMeta = pr.TypeMeta
						if pr.GroupVersionKind().Kind == "" {
							pr.SetGroupVersionKind(prometheusv1.SchemeGroupVersion.WithKind(prometheusv1.PrometheusRuleKind))
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

					// Restore TypeMeta after patch operation
					if err == nil && originalTypeMeta.Kind != "" {
						if pr, ok := obj.(*prometheusv1.PrometheusRule); ok {
							pr.TypeMeta = originalTypeMeta
						}
					}

					return err
				},
				List: func(ctx context.Context, clientww client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
					err := clientww.List(ctx, obj, opts...)
					if err != nil {
						return err
					}
					if prList, ok := obj.(*prometheusv1.PrometheusRuleList); ok {
						for i := range prList.Items {
							if prList.Items[i].GroupVersionKind().Kind == "" {
								prList.Items[i].SetGroupVersionKind(prometheusv1.SchemeGroupVersion.WithKind(prometheusv1.PrometheusRuleKind))
							}
						}
					}
					return nil
				},
			}).WithScheme(newTestScheme()).WithObjects(initObjs...).Build()
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
			assert.NoError(t, err)

			rules := []prometheusv1.PrometheusRule{}
			for _, config := range dc {
				rule := prometheusv1.PrometheusRule{}
				err = fakeClient.Get(context.Background(), client.ObjectKey(config.Config.ConfigReferent), &rule)
				assert.NoError(t, err)
				rules = append(rules, rule)
			}
			if tc.expects != nil {
				tc.expects(t, rules)
			}
		})
	}
}

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = addonv1alpha1.AddToScheme(s)
	_ = cooprometheusv1alpha1.AddToScheme(s)
	_ = prometheusv1.AddToScheme(s)
	_ = hyperv1.AddToScheme(s)
	return s
}

func newCMAO(placements ...addonv1alpha1.PlacementStrategy) *addonv1alpha1.ClusterManagementAddOn {
	return &addonv1alpha1.ClusterManagementAddOn{
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
		Spec: addonv1alpha1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1alpha1.InstallStrategy{
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
