package handlers

import (
	"fmt"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
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

func TestBuildDefaultStackCollectionResources(t *testing.T) {
	scheme := runtime.NewScheme()
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
	spoke := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "spoke-1"},
	}

	t.Run("disabled default stack returns nothing", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).Build()
		objects, placementConfigs, err := BuildDefaultStackCollectionResources(t.Context(), k8s, cmao, addon.LogsOptions{}, addon.LogsOptions{}, "")
		require.NoError(t, err)
		assert.Empty(t, objects)
		assert.Empty(t, placementConfigs)
	})

	t.Run("CLF is a CMAO placement config; LokiStack is not", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(spoke).Build()
		platform := addon.LogsOptions{DefaultStack: true}

		objects, placementConfigs, err := BuildDefaultStackCollectionResources(t.Context(), k8s, cmao, platform, addon.LogsOptions{}, "")
		require.NoError(t, err)

		require.Len(t, placementConfigs, 1)
		assert.Equal(t, addoncfg.GlobalPlacementRef, placementConfigs[0].PlacementRef)
		assert.Equal(t, addoncfg.ClusterLogForwardersResource, placementConfigs[0].Config.Resource)
		for _, cfg := range placementConfigs {
			assert.NotEqual(t, addoncfg.LokiStacksResource, cfg.Config.Resource)
		}

		kinds := map[string]int{}
		for _, obj := range objects {
			kinds[obj.GetObjectKind().GroupVersionKind().Kind]++
		}
		assert.Equal(t, 1, kinds["ClusterLogForwarder"])
		assert.Equal(t, 0, kinds["LokiStack"])
	})
}

func TestDefaultStackCollectionResourcesSurviveDeleteOrphan(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))

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

	objs, _, err := BuildDefaultStackCollectionResources(ctx, fakeClient, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)

	for _, obj := range objs {
		require.NoError(t, controllerutil.SetControllerReference(cmao, obj, scheme))
		require.NoError(t, fakeClient.Create(ctx, obj))
	}

	require.NoError(t, common.DeleteOrphanResources(ctx, klog.Background(), fakeClient, cmao, &loggingv1.ClusterLogForwarderList{}))

	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName),
		Namespace: addoncfg.InstallNamespace,
	}
	require.NoError(t, fakeClient.Get(ctx, key, &loggingv1.ClusterLogForwarder{}), "CMAO-owned ClusterLogForwarder with a matching placement annotation must not be deleted")
}
