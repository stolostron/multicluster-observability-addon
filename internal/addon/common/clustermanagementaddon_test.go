package common

import (
	"context"
	"testing"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureConfigsInAddon(t *testing.T) {
	paConfigGR := addonv1beta1.ConfigGroupResource{
		Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
		Resource: cooprometheusv1alpha1.PrometheusAgentName,
	}
	platformConfig := addonv1beta1.AddOnConfig{
		ConfigGroupResource: paConfigGR,
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "platform-agent",
			Namespace: "ns",
		},
	}
	uwlConfig := addonv1beta1.AddOnConfig{
		ConfigGroupResource: paConfigGR,
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "user-workload-agent",
			Namespace: "ns",
		},
	}

	placementRefA := addonv1beta1.PlacementRef{
		Namespace: "ns",
		Name:      "a",
	}
	placementRefB := addonv1beta1.PlacementRef{
		Namespace: "ns",
		Name:      "b",
	}

	testCases := []struct {
		name               string
		initialPlacements  []addonv1beta1.PlacementStrategy
		inputConfigs       []DefaultConfig
		expectedPlacements []addonv1beta1.PlacementStrategy
	}{
		{
			name: "empty config list - no change",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
				},
			},
			inputConfigs: []DefaultConfig{},
			expectedPlacements: []addonv1beta1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
				},
			},
		},
		{
			name: "no configs to add - all present",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
			},
			expectedPlacements: []addonv1beta1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
				},
			},
		},
		{
			name: "add config to existing placement",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
				{PlacementRef: placementRefA, Config: uwlConfig},
			},
			expectedPlacements: []addonv1beta1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1beta1.AddOnConfig{platformConfig, uwlConfig},
				},
			},
		},
		{
			name: "add config to empty placement",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
				{
					PlacementRef: placementRefB,
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
				{PlacementRef: placementRefB, Config: platformConfig},
			},
			expectedPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
					PlacementRef: placementRefB,
				},
			},
		},
		{
			name: "no matching placement - no change",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefB, Config: platformConfig},
			},
			expectedPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
		},
		{
			name: "multiple configs to multiple placements",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
				{
					PlacementRef: placementRefB,
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
				{PlacementRef: placementRefA, Config: uwlConfig},
				{PlacementRef: placementRefB, Config: platformConfig},
				{PlacementRef: placementRefB, Config: uwlConfig},
			},
			expectedPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig, uwlConfig},
					PlacementRef: placementRefA,
				},
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig, uwlConfig},
					PlacementRef: placementRefB,
				},
			},
		},
		{
			name: "duplicated configs",
			initialPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: uwlConfig},
				{PlacementRef: placementRefA, Config: uwlConfig},
			},
			expectedPlacements: []addonv1beta1.PlacementStrategy{
				{
					Configs:      []addonv1beta1.AddOnConfig{platformConfig, uwlConfig},
					PlacementRef: placementRefA,
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cmao := &addonv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: addoncfg.Name,
				},
				Spec: addonv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1beta1.InstallStrategy{
						Placements: tt.initialPlacements,
					},
				},
			}

			ensureConfigsInAddon(cmao, tt.inputConfigs)
			assert.ElementsMatch(t, tt.expectedPlacements, cmao.Spec.InstallStrategy.Placements)
		})
	}
}

func newCMAOTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, prometheusv1.AddToScheme(scheme))
	require.NoError(t, prometheusv1alpha1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1.AddToScheme(scheme))
	require.NoError(t, cooprometheusv1alpha1.AddToScheme(scheme))
	return scheme
}

func TestRemoveStaleConfigs(t *testing.T) {
	scheme := newCMAOTestScheme(t)

	placementRefA := addonv1beta1.PlacementRef{Namespace: "ns", Name: "a"}

	scrapeConfigCfg := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
			Resource: cooprometheusv1alpha1.ScrapeConfigName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "my-scrapeconfig",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	coreosScrapeConfigCfg := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    prometheusv1alpha1.SchemeGroupVersion.Group,
			Resource: prometheusv1alpha1.ScrapeConfigName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "my-coreos-scrapeconfig",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	promRuleCfg := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    prometheusv1.SchemeGroupVersion.Group,
			Resource: prometheusv1.PrometheusRuleName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "my-promrule",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	cooPromRuleCfg := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    cooprometheusv1.SchemeGroupVersion.Group,
			Resource: cooprometheusv1.PrometheusRuleName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "my-coo-promrule",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	agentCfg := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
			Resource: cooprometheusv1alpha1.PrometheusAgentName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "my-agent",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	existingScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-scrapeconfig",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	existingCoreosScrapeConfig := &prometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-coreos-scrapeconfig",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	existingPromRule := &prometheusv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-promrule",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	existingCooPromRule := &cooprometheusv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-coo-promrule",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	testCases := []struct {
		name            string
		existingObjects []client.Object
		initialConfigs  []addonv1beta1.AddOnConfig
		expectedConfigs []addonv1beta1.AddOnConfig
		expectError     bool
	}{
		{
			name:            "existing resources are kept",
			existingObjects: []client.Object{existingScrapeConfig.DeepCopy(), existingCoreosScrapeConfig.DeepCopy(), existingPromRule.DeepCopy(), existingCooPromRule.DeepCopy()},
			initialConfigs:  []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
			expectedConfigs: []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
		},
		{
			name:            "non-existent scrapeconfig is removed",
			existingObjects: []client.Object{existingCoreosScrapeConfig.DeepCopy(), existingPromRule.DeepCopy(), existingCooPromRule.DeepCopy()},
			initialConfigs:  []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
			expectedConfigs: []addonv1beta1.AddOnConfig{coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
		},
		{
			name:            "non-existent coreos scrapeconfig is removed",
			existingObjects: []client.Object{existingScrapeConfig.DeepCopy(), existingPromRule.DeepCopy(), existingCooPromRule.DeepCopy()},
			initialConfigs:  []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
			expectedConfigs: []addonv1beta1.AddOnConfig{scrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
		},
		{
			name:            "non-existent prometheusrule is removed",
			existingObjects: []client.Object{existingScrapeConfig.DeepCopy(), existingCoreosScrapeConfig.DeepCopy(), existingCooPromRule.DeepCopy()},
			initialConfigs:  []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
			expectedConfigs: []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, cooPromRuleCfg, agentCfg},
		},
		{
			name:            "non-existent coo prometheusrule is removed",
			existingObjects: []client.Object{existingScrapeConfig.DeepCopy(), existingCoreosScrapeConfig.DeepCopy(), existingPromRule.DeepCopy()},
			initialConfigs:  []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
			expectedConfigs: []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, agentCfg},
		},
		{
			name:            "all stale configs removed",
			existingObjects: []client.Object{},
			initialConfigs:  []addonv1beta1.AddOnConfig{scrapeConfigCfg, coreosScrapeConfigCfg, promRuleCfg, cooPromRuleCfg, agentCfg},
			expectedConfigs: []addonv1beta1.AddOnConfig{agentCfg},
		},
		{
			name:            "non-scrapeconfig-or-promrule configs are never removed",
			existingObjects: []client.Object{},
			initialConfigs:  []addonv1beta1.AddOnConfig{agentCfg},
			expectedConfigs: []addonv1beta1.AddOnConfig{agentCfg},
		},
		{
			name:            "empty configs - no change",
			existingObjects: []client.Object{},
			initialConfigs:  []addonv1beta1.AddOnConfig{},
			expectedConfigs: []addonv1beta1.AddOnConfig{},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.existingObjects...).Build()

			cmao := &addonv1beta1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name},
				Spec: addonv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1beta1.InstallStrategy{
						Placements: []addonv1beta1.PlacementStrategy{
							{
								PlacementRef: placementRefA,
								Configs:      tt.initialConfigs,
							},
						},
					},
				},
			}

			err := removeStaleConfigs(context.Background(), fakeClient, cmao)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedConfigs, cmao.Spec.InstallStrategy.Placements[0].Configs)
		})
	}
}

func TestDoesScrapeConfigOrPrometheusRuleExist(t *testing.T) {
	scheme := newCMAOTestScheme(t)

	existingScrapeConfig := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-sc",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	existingCoreosScrapeConfig := &prometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-coreos-sc",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	existingPromRule := &prometheusv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-rule",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	existingCooRule := &cooprometheusv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-coo-rule",
			Namespace: addoncfg.InstallNamespace,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		existingScrapeConfig,
		existingCoreosScrapeConfig,
		existingPromRule,
		existingCooRule,
	).Build()

	testCases := []struct {
		name           string
		cfg            addonv1beta1.AddOnConfig
		expected       bool
		expectNotFound bool
	}{
		{
			name: "existing coreos scrapeconfig returns true",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    prometheusv1alpha1.SchemeGroupVersion.Group,
					Resource: prometheusv1alpha1.ScrapeConfigName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "existing-coreos-sc",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expected: true,
		},
		{
			name: "non-existent coreos scrapeconfig returns not found",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    prometheusv1alpha1.SchemeGroupVersion.Group,
					Resource: prometheusv1alpha1.ScrapeConfigName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "does-not-exist",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expectNotFound: true,
		},
		{
			name: "existing coo prometheusrule returns true",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    cooprometheusv1.SchemeGroupVersion.Group,
					Resource: cooprometheusv1.PrometheusRuleName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "existing-coo-rule",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expected: true,
		},
		{
			name: "non-existent coo prometheusrule returns not found",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    cooprometheusv1.SchemeGroupVersion.Group,
					Resource: cooprometheusv1.PrometheusRuleName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "does-not-exist",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expectNotFound: true,
		},
		{
			name: "existing scrapeconfig returns true",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
					Resource: cooprometheusv1alpha1.ScrapeConfigName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "existing-sc",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expected: true,
		},
		{
			name: "non-existent scrapeconfig returns not found",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
					Resource: cooprometheusv1alpha1.ScrapeConfigName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "does-not-exist",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expectNotFound: true,
		},
		{
			name: "existing prometheusrule returns true",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    prometheusv1.SchemeGroupVersion.Group,
					Resource: prometheusv1.PrometheusRuleName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "existing-rule",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expected: true,
		},
		{
			name: "non-existent prometheusrule returns not found",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    prometheusv1.SchemeGroupVersion.Group,
					Resource: prometheusv1.PrometheusRuleName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "gone-rule",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expectNotFound: true,
		},
		{
			name: "other resource type returns false with no error",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
					Resource: cooprometheusv1alpha1.PrometheusAgentName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "some-agent",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expected: false,
		},
		{
			name: "unsupported group for scrapeconfig returns false",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    "invalid-group.com",
					Resource: cooprometheusv1alpha1.ScrapeConfigName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "existing-sc",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expected: false,
		},
		{
			name: "unsupported group for prometheusrule returns false",
			cfg: addonv1beta1.AddOnConfig{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    "invalid-group.com",
					Resource: prometheusv1.PrometheusRuleName,
				},
				ConfigReferent: addonv1beta1.ConfigReferent{
					Name:      "existing-rule",
					Namespace: addoncfg.InstallNamespace,
				},
			},
			expected: false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := doesScrapeConfigOrPrometheusRuleExist(context.Background(), fakeClient, tt.cfg)
			if tt.expectNotFound {
				require.Error(t, err)
				assert.True(t, apierrors.IsNotFound(err), "expected NotFound error, got: %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
