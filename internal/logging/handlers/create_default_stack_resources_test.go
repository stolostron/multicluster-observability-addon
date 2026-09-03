package handlers

import (
	"context"
	"fmt"
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	lmanifests "github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// buildTestScheme registers all types required by the logging handler tests.
func buildTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))
	require.NoError(t, lokiv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

// buildTestCMAO returns a CMAO with the given placement strategies.
func buildTestCMAO(placements ...addonapiv1beta1.PlacementStrategy) *addonapiv1beta1.ClusterManagementAddOn {
	return &addonapiv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  "cmao-uid",
		},
		Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonapiv1beta1.InstallStrategy{
				Placements: placements,
			},
		},
	}
}

// globalPlacement returns a PlacementStrategy referencing the global placement.
func globalPlacement() addonapiv1beta1.PlacementStrategy {
	return addonapiv1beta1.PlacementStrategy{
		PlacementRef: addoncfg.GlobalPlacementRef,
	}
}

func buildObjStorageSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lmanifests.DefaultStorageObjStorageSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
	}
}

func TestDeleteOrphanResources_KeepsMatchingResources(t *testing.T) {
	ctx := t.Context()
	scheme := buildTestScheme(t)
	cmao := buildTestCMAO(globalPlacement())

	objStorageSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lmanifests.DefaultStorageObjStorageSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao, objStorageSecret).Build()

	platform := addon.LogsOptions{DefaultStack: true}

	clfObjs, _, err := BuildCLFResources(ctx, fakeClient, cmao, platform, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)

	lsObjs, _, err := BuildLokiStackResources(ctx, fakeClient, platform, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)

	allObjs := append(clfObjs, lsObjs...)
	for _, obj := range allObjs {
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

func TestBuildDefaultStackResources_MissingObjStorageSecret(t *testing.T) {
	ctx := t.Context()
	scheme := buildTestScheme(t)
	cmao := buildTestCMAO(globalPlacement())

	// Deliberately no object storage secret seeded into the fake client.
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()

	_, _, err := BuildLokiStackResources(ctx, fakeClient, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
	require.Error(t, err)
	require.ErrorIs(t, err, errObjStorageSecretNotFound)
}

func TestUnmanagedStackReturnsEmpty(t *testing.T) {
	ctx := t.Context()
	scheme := buildTestScheme(t)
	cmao := buildTestCMAO(globalPlacement())
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()
	unmanagedOpts := addon.LogsOptions{DefaultStack: false}

	cases := []struct {
		name string
		call func() ([]client.Object, []common.DefaultConfig, error)
	}{
		{
			name: "BuildCLFResources",
			call: func() ([]client.Object, []common.DefaultConfig, error) {
				return BuildCLFResources(ctx, fakeClient, cmao, unmanagedOpts, addon.LogsOptions{}, "hub.example.com")
			},
		},
		{
			name: "BuildLokiStackResources",
			call: func() ([]client.Object, []common.DefaultConfig, error) {
				return BuildLokiStackResources(ctx, fakeClient, unmanagedOpts, addon.LogsOptions{}, "hub.example.com")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objs, cfgs, err := tc.call()
			require.NoError(t, err)
			assert.Empty(t, objs)
			assert.Empty(t, cfgs)
		})
	}
}

func TestBuildCLFResources(t *testing.T) {
	t.Run("managed stack: no placements returns empty", func(t *testing.T) {
		ctx := t.Context()
		scheme := buildTestScheme(t)
		cmao := buildTestCMAO() // no placements
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()

		objs, cfgs, err := BuildCLFResources(ctx, fakeClient, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
		require.NoError(t, err)
		assert.Empty(t, objs)
		assert.Empty(t, cfgs)
	})

	t.Run("managed stack: creates one CLF per placement", func(t *testing.T) {
		ctx := t.Context()
		scheme := buildTestScheme(t)

		p1 := addonapiv1beta1.PlacementStrategy{
			PlacementRef: addonapiv1beta1.PlacementRef{Name: "placement-a", Namespace: "ns-a"},
		}
		p2 := addonapiv1beta1.PlacementStrategy{
			PlacementRef: addonapiv1beta1.PlacementRef{Name: "placement-b", Namespace: "ns-b"},
		}
		cmao := buildTestCMAO(p1, p2)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()

		objs, cfgs, err := BuildCLFResources(ctx, fakeClient, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
		require.NoError(t, err)
		require.Len(t, objs, 2, "one CLF per placement")
		require.Len(t, cfgs, 2, "one defaultConfig per placement")

		// Verify names and namespaces are correct.
		names := map[string]bool{}
		for _, obj := range objs {
			clf, ok := obj.(*loggingv1.ClusterLogForwarder)
			require.True(t, ok, "returned object must be a ClusterLogForwarder")
			assert.Equal(t, addoncfg.InstallNamespace, clf.Namespace)
			assert.Equal(t, loggingv1.ManagementStateUnmanaged, clf.Spec.ManagementState)
			names[clf.Name] = true
		}
		assert.True(t, names[fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, "placement-a")])
		assert.True(t, names[fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, "placement-b")])

		// Verify each defaultConfig references the correct placement.
		refs := map[addonapiv1beta1.PlacementRef]bool{}
		for _, cfg := range cfgs {
			refs[cfg.PlacementRef] = true
		}
		assert.True(t, refs[p1.PlacementRef])
		assert.True(t, refs[p2.PlacementRef])
	})
}

func TestBuildLokiStackResources(t *testing.T) {
	t.Run("managed stack: creates LokiStack with global placement config", func(t *testing.T) {
		ctx := t.Context()
		scheme := buildTestScheme(t)
		// No managed clusters — no per-tenant certs generated.
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(buildObjStorageSecret()).Build()

		objs, cfgs, err := BuildLokiStackResources(ctx, fakeClient, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
		require.NoError(t, err)
		require.NotEmpty(t, objs, "at least the LokiStack must be returned")
		require.Len(t, cfgs, 1, "exactly one defaultConfig for the global LokiStack placement")

		// Verify the LokiStack object.
		ls, ok := objs[0].(*lokiv1.LokiStack)
		require.True(t, ok, "first returned object must be a LokiStack")
		assert.Equal(t, fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName), ls.Name)
		assert.Equal(t, addoncfg.InstallNamespace, ls.Namespace)
		assert.Equal(t, lokiv1.ManagementStateUnmanaged, ls.Spec.ManagementState)

		// Verify the defaultConfig points to the global placement.
		assert.Equal(t, addoncfg.GlobalPlacementRef, cfgs[0].PlacementRef)
		assert.Equal(t, addoncfg.LokiStacksResource, cfgs[0].Config.Resource)
	})
}

// CLF errors should not block LokiStack install
func TestBuildLokiStackResourcesWhenCLFFails(t *testing.T) {
	ctx := t.Context()
	scheme := buildTestScheme(t)
	cmao := buildTestCMAO(globalPlacement())

	// Intercept Get calls: return a server error for CLF, pass through everything else.
	clfGetFails := interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*loggingv1.ClusterLogForwarder); ok {
				return fmt.Errorf("simulated API server error on CLF Get")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}

	brokenClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cmao, buildObjStorageSecret()).
		WithInterceptorFuncs(clfGetFails).
		Build()

	platform := addon.LogsOptions{DefaultStack: true}

	// CLF build fail
	_, _, clfErr := BuildCLFResources(ctx, brokenClient, cmao, platform, addon.LogsOptions{}, "hub.example.com")
	require.Error(t, clfErr, "BuildCLFResources must fail when CLF Get returns an API error")
	assert.Contains(t, clfErr.Error(), "simulated API server error on CLF Get")

	lsObjs, lsDefaultConfig, err := BuildLokiStackResources(ctx, brokenClient, platform, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err, "BuildLokiStackResources must succeed even when CLF Get fails")

	// Verify the LokiStack is present and correctly formed.
	require.NotEmpty(t, lsObjs, "LokiStack must be returned despite CLF error")
	ls, ok := lsObjs[0].(*lokiv1.LokiStack)
	require.True(t, ok, "first returned object must be a LokiStack")
	assert.Equal(t, fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName), ls.Name)
	assert.Equal(t, addoncfg.InstallNamespace, ls.Namespace)

	// Verify the defaultConfig is present so the CMAO placement is updated.
	require.Len(t, lsDefaultConfig, 1, "LokiStack defaultConfig must be present")
	assert.Equal(t, addoncfg.GlobalPlacementRef, lsDefaultConfig[0].PlacementRef)
}
