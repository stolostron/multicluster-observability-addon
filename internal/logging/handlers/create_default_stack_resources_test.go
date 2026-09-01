package handlers

import (
	"fmt"
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/cluster-lifecycle-api/constants"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestBuildDefaultStackResources(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lokiv1.AddToScheme(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))

	globalPlacement := addonv1beta1.PlacementStrategy{
		PlacementRef: addoncfg.GlobalPlacementRef,
	}
	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Placements: []addonv1beta1.PlacementStrategy{globalPlacement},
			},
		},
	}
	hub := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "local-cluster",
			Labels: map[string]string{constants.SelfManagedClusterLabelKey: "true"},
		},
	}
	spoke := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "spoke-1"},
	}

	t.Run("disabled default stack returns nothing", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).Build()
		objects, placementConfigs, clusterConfigs, err := BuildDefaultStackResources(t.Context(), k8s, cmao, addon.LogsOptions{}, addon.LogsOptions{}, "")
		require.NoError(t, err)
		assert.Empty(t, objects)
		assert.Empty(t, placementConfigs)
		assert.Empty(t, clusterConfigs)
	})

	t.Run("LokiStack is a hub cluster config, not a CMAO placement config", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hub, spoke).Build()
		platform := addon.LogsOptions{DefaultStack: true}

		objects, placementConfigs, clusterConfigs, err := BuildDefaultStackResources(t.Context(), k8s, cmao, platform, addon.LogsOptions{}, "")
		require.NoError(t, err)

		require.Len(t, placementConfigs, 1)
		assert.Equal(t, addoncfg.GlobalPlacementRef, placementConfigs[0].PlacementRef)
		assert.Equal(t, addoncfg.ClusterLogForwardersResource, placementConfigs[0].Config.Resource)
		for _, cfg := range placementConfigs {
			assert.NotEqual(t, addoncfg.LokiStacksResource, cfg.Config.Resource)
		}

		require.Len(t, clusterConfigs, 1)
		assert.Equal(t, "local-cluster", clusterConfigs[0].ClusterNamespace)
		assert.Equal(t, addoncfg.LokiStacksResource, clusterConfigs[0].Config.Resource)
		assert.Equal(t, "loki.grafana.com", clusterConfigs[0].Config.Group)
		assert.Equal(t, "mcoa-default-global", clusterConfigs[0].Config.Name)

		var foundLS bool
		for _, obj := range objects {
			if ls, ok := obj.(*lokiv1.LokiStack); ok {
				foundLS = true
				assert.Equal(t, addoncfg.GlobalPlacementNamespace+"/"+addoncfg.GlobalPlacementName, ls.Annotations[addoncfg.PlacementAnnotationKey])
			}
		}
		assert.True(t, foundLS, "expected a LokiStack object")
	})

	t.Run("uses labeled hub cluster name as MCAO namespace", func(t *testing.T) {
		customHub := &clusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "my-hub",
				Labels: map[string]string{constants.SelfManagedClusterLabelKey: "true"},
			},
		}
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(customHub).Build()
		platform := addon.LogsOptions{DefaultStack: true}

		_, _, clusterConfigs, err := BuildDefaultStackResources(t.Context(), k8s, cmao, platform, addon.LogsOptions{}, "")
		require.NoError(t, err)
		require.Len(t, clusterConfigs, 1)
		assert.Equal(t, "my-hub", clusterConfigs[0].ClusterNamespace)
	})
}

func TestBuildDefaultStackResources_ObjectKinds(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lokiv1.AddToScheme(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Placements: []addonv1beta1.PlacementStrategy{{PlacementRef: addoncfg.GlobalPlacementRef}},
			},
		},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "spoke-1"},
	}).Build()

	objects, _, _, err := BuildDefaultStackResources(t.Context(), k8s, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "")
	require.NoError(t, err)

	kinds := map[string]int{}
	for _, obj := range objects {
		kinds[obj.GetObjectKind().GroupVersionKind().Kind]++
	}
	assert.Equal(t, 1, kinds["ClusterLogForwarder"])
	assert.Equal(t, 1, kinds["LokiStack"])
}

func TestDefaultStackResourcesSurviveDeleteOrphan(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))
	require.NoError(t, lokiv1.AddToScheme(scheme))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  "cmao-uid",
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Placements: []addonv1beta1.PlacementStrategy{
					{
						PlacementRef: addoncfg.GlobalPlacementRef,
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()

	objs, _, _, err := BuildDefaultStackResources(ctx, fakeClient, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)

	for _, obj := range objs {
		require.NoError(t, controllerutil.SetControllerReference(cmao, obj, scheme))
		require.NoError(t, fakeClient.Create(ctx, obj))
	}

	require.NoError(t, common.DeleteOrphanResources(ctx, klog.Background(), fakeClient, cmao, &loggingv1.ClusterLogForwarderList{}))
	require.NoError(t, common.DeleteOrphanResources(ctx, klog.Background(), fakeClient, cmao, &lokiv1.LokiStackList{}))

	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName),
		Namespace: addoncfg.InstallNamespace,
	}
	require.NoError(t, fakeClient.Get(ctx, key, &loggingv1.ClusterLogForwarder{}), "CMAO-owned ClusterLogForwarder with a matching placement annotation must not be deleted")
	require.NoError(t, fakeClient.Get(ctx, key, &lokiv1.LokiStack{}), "CMAO-owned LokiStack with a matching placement annotation must not be deleted")
}
