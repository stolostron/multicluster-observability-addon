package handlers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setupTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, clusterv1beta1.Install(scheme))
	return scheme
}

func newTestOptionsBuilder(t *testing.T, objs ...runtime.Object) *OptionsBuilder {
	t.Helper()
	scheme := setupTestScheme(t)
	clientObjs := make([]runtime.Object, len(objs))
	copy(clientObjs, objs)

	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, obj := range objs {
		if co, ok := obj.(metav1.Object); ok {
			_ = co // type assertion check
		}
	}
	c := builder.WithRuntimeObjects(objs...).Build()
	return &OptionsBuilder{
		Client: c,
		Logger: logr.Discard(),
	}
}

func newPlatformOpts(nsEnabled, virtEnabled bool) addon.Options {
	return addon.Options{
		Platform: addon.PlatformOptions{
			Enabled: true,
			AnalyticsOptions: addon.AnalyticsOptions{
				RightSizing: addon.RightSizingOptions{
					NamespaceEnabled:      nsEnabled,
					VirtualizationEnabled: virtEnabled,
				},
			},
		},
	}
}

func createTestPlacement(name string) *clusterv1beta1.Placement {
	return &clusterv1beta1.Placement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rightsizing.PlacementNamespace,
		},
		Spec: rightsizing.GetDefaultRSPlacement().Spec,
	}
}

func createTestConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{"config.yaml": "test"},
	}
}

func TestReconcileRSResources_CleanupNamespace(t *testing.T) {
	nsPlacement := createTestPlacement(rightsizing.NamespacePlacementName)
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtPlacement := createTestPlacement(rightsizing.VirtualizationPlacementName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsPlacement, nsCM, virtPlacement, virtCM)
	ctx := context.TODO()

	// Disable namespace RS, keep virt enabled
	opts := newPlatformOpts(false, true)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	// Namespace placement and configmap should be deleted
	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespacePlacementName, Namespace: rightsizing.PlacementNamespace,
	}, &clusterv1beta1.Placement{})
	assert.True(t, apierrors.IsNotFound(err), "namespace placement should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "namespace configmap should be deleted")

	// Virt placement should still exist (updated, not deleted)
	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationPlacementName, Namespace: rightsizing.PlacementNamespace,
	}, &clusterv1beta1.Placement{})
	assert.NoError(t, err, "virtualization placement should still exist")
}

func TestReconcileRSResources_CleanupVirtualization(t *testing.T) {
	nsPlacement := createTestPlacement(rightsizing.NamespacePlacementName)
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtPlacement := createTestPlacement(rightsizing.VirtualizationPlacementName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsPlacement, nsCM, virtPlacement, virtCM)
	ctx := context.TODO()

	// Enable namespace RS, disable virt
	opts := newPlatformOpts(true, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	// Virt placement and configmap should be deleted
	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationPlacementName, Namespace: rightsizing.PlacementNamespace,
	}, &clusterv1beta1.Placement{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization placement should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization configmap should be deleted")

	// Namespace placement should still exist
	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespacePlacementName, Namespace: rightsizing.PlacementNamespace,
	}, &clusterv1beta1.Placement{})
	assert.NoError(t, err, "namespace placement should still exist")
}

func TestReconcileRSResources_CleanupBoth(t *testing.T) {
	nsPlacement := createTestPlacement(rightsizing.NamespacePlacementName)
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtPlacement := createTestPlacement(rightsizing.VirtualizationPlacementName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsPlacement, nsCM, virtPlacement, virtCM)
	ctx := context.TODO()

	// Disable both
	opts := newPlatformOpts(false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	// All RS resources should be deleted
	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespacePlacementName, Namespace: rightsizing.PlacementNamespace,
	}, &clusterv1beta1.Placement{})
	assert.True(t, apierrors.IsNotFound(err), "namespace placement should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "namespace configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationPlacementName, Namespace: rightsizing.PlacementNamespace,
	}, &clusterv1beta1.Placement{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization placement should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization configmap should be deleted")
}

func TestReconcileRSResources_CleanupIdempotent(t *testing.T) {
	// No resources exist — cleanup should succeed without error
	ob := newTestOptionsBuilder(t)
	ctx := context.TODO()

	opts := newPlatformOpts(false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)
}

func TestReconcileRSResources_PlatformDisabledCleansUp(t *testing.T) {
	nsPlacement := createTestPlacement(rightsizing.NamespacePlacementName)

	ob := newTestOptionsBuilder(t, nsPlacement)
	ctx := context.TODO()

	// Platform disabled with both RS features disabled — cleanup still runs.
	// ReconcileRSResources does NOT gate on Platform.Enabled (see NOTE in code).
	opts := addon.Options{
		Platform: addon.PlatformOptions{Enabled: false},
	}
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	// Placement should be deleted (cleanup runs regardless of Platform.Enabled)
	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespacePlacementName, Namespace: rightsizing.PlacementNamespace,
	}, &clusterv1beta1.Placement{})
	assert.True(t, apierrors.IsNotFound(err), "placement should be deleted when both RS features are disabled")
}
