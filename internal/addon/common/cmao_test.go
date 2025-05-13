package common_test

import (
	"context"
	"testing"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
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
)

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
		inputConfigs            []common.DefaultConfig
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
			inputConfigs: []common.DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
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
			inputConfigs: []common.DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
				{PlacementRef: placementRefB, Config: platformConfig},
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
			inputConfigs: []common.DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
				{PlacementRef: placementRefB, Config: platformConfig},
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
			inputConfigs: []common.DefaultConfig{
				{PlacementRef: placementRefA, Config: platformConfig},
				{PlacementRef: placementRefB, Config: platformConfig},
				{PlacementRef: placementRefA, Config: uwlConfig},
				{PlacementRef: placementRefB, Config: uwlConfig},
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
			testScheme := runtime.NewScheme()
			require.NoError(t, addonv1alpha1.AddToScheme(testScheme))
			require.NoError(t, prometheusalpha1.AddToScheme(testScheme))
			updateCalls := 0
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Update: func(ctx context.Context, clientww client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					updateCalls++
					return clientww.Update(ctx, obj, opts...)
				},
			}).WithScheme(testScheme).WithObjects(cmao).Build()

			err := common.EnsureAddonConfig(context.Background(), klog.Background(), fakeClient, tt.inputConfigs)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectUpdateCalls, updateCalls)

			finalCmao := &addonv1alpha1.ClusterManagementAddOn{}
			err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(cmao), finalCmao)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedFinalPlacements, finalCmao.Spec.InstallStrategy.Placements)
		})
	}
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
