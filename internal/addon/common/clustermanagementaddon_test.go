package common

import (
	"testing"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

func TestEnsureConfigsInAddon(t *testing.T) {
	paConfigGR := addonv1alpha1.ConfigGroupResource{
		Group:    prometheusalpha1.SchemeGroupVersion.Group,
		Resource: prometheusalpha1.PrometheusAgentName,
	}
	platformConfig := addonv1alpha1.AddOnConfig{
		ConfigGroupResource: paConfigGR,
		ConfigReferent: addonv1alpha1.ConfigReferent{
			Name:      "platform-agent",
			Namespace: "ns",
		},
	}
	uwlConfig := addonv1alpha1.AddOnConfig{
		ConfigGroupResource: paConfigGR,
		ConfigReferent: addonv1alpha1.ConfigReferent{
			Name:      "user-workload-agent",
			Namespace: "ns",
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
		name               string
		initialPlacements  []addonv1alpha1.PlacementStrategy
		inputConfigs       []DefaultConfig
		expectedPlacements []addonv1alpha1.PlacementStrategy
	}{
		{
			name: "empty config list",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
				},
			},
			inputConfigs: []DefaultConfig{},
			expectedPlacements: []addonv1alpha1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
				},
			},
		},
		{
			name: "no configs to add - all present",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
			},
			expectedPlacements: []addonv1alpha1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
				},
			},
		},
		{
			name: "add config to existing placement",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					PlacementRef: placementRefA,
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
				{PlacementRef: placementRefA, Config: uwlConfig},
			},
			expectedPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig, uwlConfig},
					PlacementRef: placementRefA,
				},
			},
		},
		{
			name: "add config to empty placement",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
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
			expectedPlacements: []addonv1alpha1.PlacementStrategy{
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
			name: "no matching placement - no change",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefB, Config: platformConfig},
			},
			expectedPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
		},
		{
			name: "multiple configs to multiple placements",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
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
			expectedPlacements: []addonv1alpha1.PlacementStrategy{
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
		{
			name: "duplicated configs",
			initialPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig},
					PlacementRef: placementRefA,
				},
			},
			inputConfigs: []DefaultConfig{
				{PlacementRef: placementRefA, Config: uwlConfig},
				{PlacementRef: placementRefA, Config: uwlConfig},
			},
			expectedPlacements: []addonv1alpha1.PlacementStrategy{
				{
					Configs:      []addonv1alpha1.AddOnConfig{platformConfig, uwlConfig},
					PlacementRef: placementRefA,
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cmao := &addonv1alpha1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: addoncfg.Name,
				},
				Spec: addonv1alpha1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1alpha1.InstallStrategy{
						Placements: tt.initialPlacements,
					},
				},
			}

			ensureConfigsInAddon(cmao, tt.inputConfigs)
			assert.ElementsMatch(t, tt.expectedPlacements, cmao.Spec.InstallStrategy.Placements)
		})
	}
}
