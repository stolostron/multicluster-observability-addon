package resource

import (
	"context"
	"net/url"
	"testing"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
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
				assert.Equal(t, gotAgent.Labels["app.kubernetes.io/component"], config.UserWorkloadPrometheusMatchLabels["app.kubernetes.io/component"])
			} else {
				assert.Equal(t, gotAgent.Labels["app.kubernetes.io/component"], config.PlatformPrometheusMatchLabels["app.kubernetes.io/component"])
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
	retAgent, err := d.reconcileAgent(context.Background(), placementRef, false)
	assert.NoError(t, err)

	foundAgent := prometheusalpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(retAgent), &foundAgent)
	assert.NoError(t, err)

	// Check default fields
	assert.EqualValues(t, 1, *foundAgent.Spec.Replicas)
	// Check ssa fields
	// Commented while the stolostron build of prometheus is not based on v3 as it requires support for the --agent flag.
	// assert.Equal(t, prmetheusImage, *foundAgent.Spec.Image)
	assert.Nil(t, foundAgent.Spec.Image)
	assert.Equal(t, config.PlatformMetricsCollectorApp, foundAgent.Spec.ServiceAccountName)
	// Check placement labels
	assert.Equal(t, foundAgent.Labels[addon.PlacementRefNameLabelKey], placementRef.Name)
	// Check platform specific values: appName and ScrapeConfigNamespaceSelector
	assert.Equal(t, foundAgent.Spec.ServiceAccountName, config.PlatformMetricsCollectorApp)
	assert.Nil(t, foundAgent.Spec.ScrapeConfigNamespaceSelector)

	// Subsequent reconcile does not trigger update
	previousPatchCalls := patchCalls
	_, err = d.reconcileAgent(context.Background(), placementRef, false)
	assert.NoError(t, err)
	assert.Equal(t, previousPatchCalls, patchCalls)

	// >>> UWL agent
	retAgent, err = d.reconcileAgent(context.Background(), placementRef, true)
	assert.NoError(t, err)

	foundAgent = prometheusalpha1.PrometheusAgent{}
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(retAgent), &foundAgent)
	assert.NoError(t, err)

	// Check uwl specific values: appName and ScrapeConfigNamespaceSelector
	assert.Equal(t, foundAgent.Spec.ServiceAccountName, config.UserWorkloadMetricsCollectorApp)
	assert.Equal(t, foundAgent.Spec.ScrapeConfigNamespaceSelector, &metav1.LabelSelector{})
}

func TestEnsureAddonConfig(t *testing.T) {
	platformConfig := addonv1alpha1.AddOnConfig{
		ConfigReferent: addonv1alpha1.ConfigReferent{
			Namespace: "ns",
			Name:      "platform-agent",
		},
		ConfigGroupResource: addonv1alpha1.ConfigGroupResource{
			Group:    prometheusalpha1.SchemeGroupVersion.Group,
			Resource: prometheusalpha1.PrometheusAgentName,
		},
	}
	uwlConfig := addonv1alpha1.AddOnConfig{
		ConfigReferent: addonv1alpha1.ConfigReferent{
			Namespace: "ns",
			Name:      "uwl-agent",
		},
		ConfigGroupResource: addonv1alpha1.ConfigGroupResource{
			Group:    prometheusalpha1.SchemeGroupVersion.Group,
			Resource: prometheusalpha1.PrometheusAgentName,
		},
	}

	placementRefA := addonv1alpha1.PlacementRef{
		Namespace: "ns",
		Name:      "a",
	}
	placementRefB := addonv1alpha1.PlacementRef{
		Namespace: "ns",
		Name:      "b",
	}

	testCases := []struct {
		name                    string
		initialPlacements       []addonv1alpha1.PlacementStrategy
		inputConfigs            []defaultConfig
		expectUpdateCalls       int
		expectedFinalPlacements []addonv1alpha1.PlacementStrategy
	}{
		{
			name: "no configs to add",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
			inputConfigs: []defaultConfig{
				{placementRef: placementRefA, config: platformConfig},
			},
			expectUpdateCalls: 0,
			expectedFinalPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
		},
		{
			name: "one configs to add in one placement",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
				{
					PlacementRef: placementRefB,
				},
			},
			inputConfigs: []defaultConfig{
				{placementRef: placementRefA, config: platformConfig},
				{placementRef: placementRefB, config: platformConfig},
			},
			expectUpdateCalls: 1,
			expectedFinalPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefB,
				},
			},
		},
		{
			name:              "no placement",
			initialPlacements: []addonv1alpha1.PlacementStrategy{},
			inputConfigs: []defaultConfig{
				{placementRef: placementRefA, config: platformConfig},
				{placementRef: placementRefB, config: platformConfig},
			},
			expectUpdateCalls:       0,
			expectedFinalPlacements: []addonv1alpha1.PlacementStrategy{},
		},
		{
			name: "multiple configs to add in multiple placements",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
				{
					PlacementRef: placementRefB,
				},
			},
			inputConfigs: []defaultConfig{
				{placementRef: placementRefA, config: platformConfig},
				{placementRef: placementRefB, config: platformConfig},
				{placementRef: placementRefA, config: uwlConfig},
				{placementRef: placementRefB, config: uwlConfig},
			},
			expectUpdateCalls: 1,
			expectedFinalPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig, uwlConfig},
					PlacementRef: placementRefA,
				},
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig, uwlConfig},
					PlacementRef: placementRefB,
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cmao := newCMAO(tt.initialPlacements...)
			updateCalls := 0
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Update: func(ctx context.Context, clientww client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					updateCalls++
					return clientww.Update(ctx, obj, opts...)
				},
			}).WithScheme(newTestScheme()).WithObjects(cmao).Build()
			d := DefaultStackResources{
				Client: fakeClient,
				CMAO:   cmao,
				Logger: klog.Background(),
			}

			err := d.ensureAddonConfig(context.Background(), tt.inputConfigs)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectUpdateCalls, updateCalls)

			finalCmao := &addonv1alpha1.ClusterManagementAddOn{}
			err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(cmao), finalCmao)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedFinalPlacements, finalCmao.Spec.InstallStrategy.Placements)
		})
	}
}

func TestReconcile(t *testing.T) {
	// variables: placements, agents, addonOptions
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
	platformAgent.Labels = map[string]string{
		addon.PlacementRefNamespaceLabelKey: placementRefA.Namespace,
		addon.PlacementRefNameLabelKey:      placementRefA.Name,
	}

	testCases := []struct {
		name              string
		initialPlacements []addonv1alpha1.PlacementStrategy
		initObjs          []client.Object
		platformEnabled   bool
		uwlEnabled        bool
		expectAgentsCount int
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
			initObjs:          []client.Object{platformAgent},
			expectAgentsCount: 0,
		},
		{
			name: "one placement with enabled monitoring",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{},
					PlacementRef: placementRefA,
				},
			},
			platformEnabled:   true,
			uwlEnabled:        true,
			initObjs:          []client.Object{platformAgent},
			expectAgentsCount: 2,
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
			platformEnabled:   true,
			uwlEnabled:        false,
			initObjs:          []client.Object{platformAgent},
			expectAgentsCount: 2,
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
			platformEnabled:   true,
			uwlEnabled:        true,
			initObjs:          []client.Object{platformAgent},
			expectAgentsCount: 4,
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
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, clientww client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					return clientww.Patch(ctx, obj, client.Merge, opts...)
				},
			}).WithScheme(newTestScheme()).WithObjects(cmao).Build()
			d := DefaultStackResources{
				Client:          fakeClient,
				CMAO:            cmao,
				Logger:          klog.Background(),
				AddonOptions:    addonOptions,
				PrometheusImage: "dummy",
			}

			err := d.Reconcile(context.Background())
			assert.NoError(t, err)

			foundAgents := prometheusalpha1.PrometheusAgentList{}
			err = fakeClient.List(context.Background(), &foundAgents)
			assert.NoError(t, err)
			assert.Len(t, foundAgents.Items, tc.expectAgentsCount)
		})
	}
}

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = addonv1alpha1.AddToScheme(s)    // Add OCM Addon types
	_ = prometheusalpha1.AddToScheme(s) // Add Prometheus Operator types
	return s
}

func newCMAO(placements ...addonv1alpha1.PlacementStrategy) *addonv1alpha1.ClusterManagementAddOn {
	return &addonv1alpha1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addon.Name,
			UID:  types.UID("test-cmao-uid"),
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
