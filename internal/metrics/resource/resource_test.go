package resource

import (
	"context"
	"net/url"
	"testing"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
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
				res := &prometheusalpha1.PrometheusAgentList{}
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
	prmetheusImage := "prometheus:version"
	placementRef := addonv1alpha1.PlacementRef{Name: "my-placement", Namespace: "my-namespace"}

	// Dynamic fake client doesn't support apply types of patch. This is overridden with an interceptor toward a
	// merge type patch that has no unwanted effect for this unit test.
	patchCalls := 0
	fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			patchCalls++
			return clientww.Patch(ctx, obj, client.Merge, opts...)
		},
	}).WithScheme(newTestScheme()).WithObjects(cmao).Build()
	d := DefaultStackResources{
		Client:          fakeClient,
		CMAO:            cmao,
		AddonOptions:    opts,
		Logger:          klog.Background(),
		PrometheusImage: prmetheusImage,
	}

	// >>> Platform agent
	retAgent, err := d.reconcileAgentForPlacement(context.Background(), placementRef, false)
	assert.NoError(t, err)

	foundAgent := prometheusalpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: retAgent.Config.Namespace, Name: retAgent.Config.Name}, &foundAgent)
	assert.NoError(t, err)

	// Check default fields
	assert.EqualValues(t, 1, *foundAgent.Spec.Replicas)
	// Check ssa fields
	// Commented while the stolostron build of prometheus is not based on v3 as it requires support for the --agent flag.
	// assert.Equal(t, prmetheusImage, *foundAgent.Spec.Image)
	assert.Nil(t, foundAgent.Spec.Image)
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

	foundAgent = prometheusalpha1.PrometheusAgent{}
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

	platformSC := &prometheusalpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: prometheusalpha1.ScrapeConfigsKind,
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
	uwlSC := &prometheusalpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: prometheusalpha1.ScrapeConfigsKind,
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
	hcpApiserverSC := &prometheusalpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: prometheusalpha1.ScrapeConfigsKind,
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
	hcpEtcdSC := &prometheusalpha1.ScrapeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: prometheusalpha1.ScrapeConfigsKind,
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
						HubEndpoint:       hubUrl,
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
					return clientww.Patch(ctx, obj, client.Merge, opts...)
				},
			}).WithScheme(newTestScheme()).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				Client:          fakeClient,
				CMAO:            cmao,
				Logger:          klog.Background(),
				AddonOptions:    addonOptions,
				PrometheusImage: "dummy",
			}

			dc, err := d.Reconcile(context.Background())
			assert.NoError(t, err)
			err = common.EnsureAddonConfig(context.Background(), klog.Background(), fakeClient, dc)
			assert.NoError(t, err)

			foundAgents := prometheusalpha1.PrometheusAgentList{}
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
		expects           func(*testing.T, []prometheusalpha1.ScrapeConfig)
	}{
		{
			name: "no scrape configs",
		},
		{
			name: "unmanaged SC is ignored",
			initObjs: []client.Object{
				&prometheusalpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusalpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: config.HubInstallNamespace,
						Name:      "test",
						Labels:    config.PlatformPrometheusMatchLabels,
					},
				},
			},
			expects: func(t *testing.T, objs []prometheusalpha1.ScrapeConfig) {
				assert.Len(t, objs, 0)
			},
		},
		{
			name: "SC target is enforced for platform",
			initObjs: []client.Object{
				&prometheusalpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusalpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.PlatformPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
					Spec: prometheusalpha1.ScrapeConfigSpec{
						ScrapeClassName: ptr.To("invalid"),
					},
				},
			},
			expects: func(t *testing.T, objs []prometheusalpha1.ScrapeConfig) {
				assert.Len(t, objs, 1)
				assert.Equal(t, *objs[0].Spec.ScrapeClassName, config.ScrapeClassCfgName)
			},
		},
		{
			name: "SC target is left for uwl",
			initObjs: []client.Object{
				&prometheusalpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusalpha1.ScrapeConfigsKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       config.HubInstallNamespace,
						Name:            "test",
						Labels:          config.UserWorkloadPrometheusMatchLabels,
						OwnerReferences: []metav1.OwnerReference{mcoOwnerRef},
					},
					Spec: prometheusalpha1.ScrapeConfigSpec{
						ScrapeClassName: ptr.To("custom"),
						StaticConfigs: []prometheusalpha1.StaticConfig{
							{
								Targets: []prometheusalpha1.Target{"test"},
							},
						},
					},
				},
			},
			isUWL: true,
			expects: func(t *testing.T, objs []prometheusalpha1.ScrapeConfig) {
				assert.Len(t, objs, 1)
				assert.Equal(t, *objs[0].Spec.ScrapeClassName, "custom")
			},
		},
		{
			name: "hcp SC is handled",
			initObjs: []client.Object{
				&prometheusalpha1.ScrapeConfig{
					TypeMeta: metav1.TypeMeta{
						Kind: prometheusalpha1.ScrapeConfigsKind,
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
			expects: func(t *testing.T, objs []prometheusalpha1.ScrapeConfig) {
				assert.Len(t, objs, 1)
				assert.Equal(t, *objs[0].Spec.ScrapeClassName, config.ScrapeClassCfgName)
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
					return clientww.Patch(ctx, obj, client.Merge, opts...)
				},
			}).WithScheme(newTestScheme()).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				CMAO:            cmao,
				Client:          fakeClient,
				Logger:          klog.Background(),
				PrometheusImage: "dummy",
			}

			dc, err := d.reconcileScrapeConfigs(context.Background(), mcoUID, tc.isUWL, tc.hasHostedClusters)
			assert.NoError(t, err)

			scrapeConfigs := []prometheusalpha1.ScrapeConfig{}
			for _, config := range dc {
				sc := prometheusalpha1.ScrapeConfig{}
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
					return clientww.Patch(ctx, obj, client.Merge, opts...)
				},
			}).WithScheme(newTestScheme()).WithObjects(initObjs...).Build()
			d := DefaultStackResources{
				CMAO:            cmao,
				Client:          fakeClient,
				Logger:          klog.Background(),
				PrometheusImage: "dummy",
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
	_ = prometheusalpha1.AddToScheme(s)
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
				HubEndpoint:       hubEp,
			},
		},
		UserWorkloads: addon.UserWorkloadOptions{
			Metrics: addon.MetricsOptions{
				CollectionEnabled: uwlEnabled,
			},
		},
	}
}
