package handlers

import (
	"fmt"
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	"github.com/stolostron/cluster-lifecycle-api/constants"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
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

func TestBuildDefaultStackStorageResources(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, lokiv1.AddToScheme(scheme))
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))

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
		objects, clusterConfigs, err := BuildDefaultStackStorageResources(t.Context(), k8s, addon.LogsOptions{}, addon.LogsOptions{}, "")
		require.NoError(t, err)
		assert.Empty(t, objects)
		assert.Empty(t, clusterConfigs)
	})

	t.Run("LokiStack is an MCAO cluster config, not a placement config", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hub, spoke).Build()
		platform := addon.LogsOptions{DefaultStack: true}

		objects, clusterConfigs, err := BuildDefaultStackStorageResources(t.Context(), k8s, platform, addon.LogsOptions{}, "")
		require.NoError(t, err)

		require.Len(t, clusterConfigs, 1)
		assert.Equal(t, "local-cluster", clusterConfigs[0].ClusterNamespace)
		assert.Equal(t, addoncfg.LokiStacksResource, clusterConfigs[0].Config.Resource)
		assert.Equal(t, "loki.grafana.com", clusterConfigs[0].Config.Group)
		assert.Equal(t, "mcoa-default-global", clusterConfigs[0].Config.Name)

		var foundLS bool
		var storageCertNS string
		for _, obj := range objects {
			switch obj.GetObjectKind().GroupVersionKind().Kind {
			case "LokiStack":
				foundLS = true
				ls := obj.(*lokiv1.LokiStack)
				assert.Equal(t, addoncfg.InstallNamespace, ls.Namespace)
				assert.Equal(t, lokiv1.ManagementStateUnmanaged, ls.Spec.ManagementState)
			case "Certificate":
				if obj.GetName() == manifests.DefaultStorageMTLSSecretName {
					storageCertNS = obj.GetNamespace()
				}
			}
		}
		assert.True(t, foundLS, "expected a LokiStack object")
		assert.Equal(t, "local-cluster", storageCertNS)
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

		objects, clusterConfigs, err := BuildDefaultStackStorageResources(t.Context(), k8s, platform, addon.LogsOptions{}, "")
		require.NoError(t, err)
		require.Len(t, clusterConfigs, 1)
		assert.Equal(t, "my-hub", clusterConfigs[0].ClusterNamespace)

		var storageCertNS string
		for _, obj := range objects {
			if obj.GetName() == manifests.DefaultStorageMTLSSecretName {
				storageCertNS = obj.GetNamespace()
			}
		}
		assert.Equal(t, "my-hub", storageCertNS)
	})
}

func TestDefaultStackStorageResourcesSurviveDeleteOrphan(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))
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

	objs, _, err := BuildDefaultStackStorageResources(ctx, fakeClient, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)

	for _, obj := range objs {
		if _, ok := obj.(*lokiv1.LokiStack); !ok {
			continue
		}
		require.NoError(t, controllerutil.SetControllerReference(cmao, obj, scheme))
		require.NoError(t, fakeClient.Create(ctx, obj))
	}

	require.NoError(t, common.DeleteOrphanResources(ctx, klog.Background(), fakeClient, cmao, &lokiv1.LokiStackList{}))

	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName),
		Namespace: addoncfg.InstallNamespace,
	}
	require.NoError(t, fakeClient.Get(ctx, key, &lokiv1.LokiStack{}), "CMAO-owned LokiStack with a matching placement annotation must not be deleted")
}
