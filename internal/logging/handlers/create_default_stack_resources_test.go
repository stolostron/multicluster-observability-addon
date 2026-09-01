package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/cluster-lifecycle-api/constants"
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

func hubCluster(name string) *clusterv1.ManagedCluster {
	return &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{constants.SelfManagedClusterLabelKey: "true"},
		},
	}
}

func TestDeleteOrphanResources_KeepsMatchingResources(t *testing.T) {
	ctx := t.Context()
	scheme := buildTestScheme(t)
	cmao := buildTestCMAO(globalPlacement())

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao, buildObjStorageSecret()).Build()

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

	clfObjs, clfCfgs, err := BuildCLFResources(ctx, fakeClient, cmao, unmanagedOpts, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)
	assert.Empty(t, clfObjs)
	assert.Empty(t, clfCfgs)

	lsObjs, lsCfgs, err := BuildLokiStackResources(ctx, fakeClient, unmanagedOpts, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)
	assert.Empty(t, lsObjs)
	assert.Empty(t, lsCfgs)
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
	t.Run("managed stack: LokiStack is a hub cluster config, not a CMAO placement config", func(t *testing.T) {
		ctx := t.Context()
		scheme := buildTestScheme(t)
		cmao := buildTestCMAO(globalPlacement())
		hub := hubCluster("local-cluster")
		spoke := &clusterv1.ManagedCluster{ObjectMeta: metav1.ObjectMeta{Name: "spoke-1"}}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao, hub, spoke, buildObjStorageSecret()).Build()

		clfObjs, placementConfigs, err := BuildCLFResources(ctx, fakeClient, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
		require.NoError(t, err)
		require.Len(t, placementConfigs, 1)
		assert.Equal(t, addoncfg.GlobalPlacementRef, placementConfigs[0].PlacementRef)
		assert.Equal(t, addoncfg.ClusterLogForwardersResource, placementConfigs[0].Config.Resource)
		for _, cfg := range placementConfigs {
			assert.NotEqual(t, addoncfg.LokiStacksResource, cfg.Config.Resource)
		}

		lsObjs, clusterConfigs, err := BuildLokiStackResources(ctx, fakeClient, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
		require.NoError(t, err)
		require.Len(t, clusterConfigs, 1)
		assert.Equal(t, "local-cluster", clusterConfigs[0].ClusterNamespace)
		assert.Equal(t, addoncfg.LokiStacksResource, clusterConfigs[0].Config.Resource)
		assert.Equal(t, "loki.grafana.com", clusterConfigs[0].Config.Group)
		assert.Equal(t, "mcoa-default-global", clusterConfigs[0].Config.Name)

		var foundLS bool
		for _, obj := range lsObjs {
			if ls, ok := obj.(*lokiv1.LokiStack); ok {
				foundLS = true
				assert.Equal(t, fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName), ls.Name)
				assert.Equal(t, addoncfg.InstallNamespace, ls.Namespace)
				assert.Equal(t, lokiv1.ManagementStateUnmanaged, ls.Spec.ManagementState)
				assert.Equal(t, addoncfg.GlobalPlacementNamespace+"/"+addoncfg.GlobalPlacementName, ls.Annotations[addoncfg.PlacementAnnotationKey])
			}
		}
		assert.True(t, foundLS, "expected a LokiStack object")

		kinds := map[string]int{}
		for _, obj := range append(clfObjs, lsObjs...) {
			kinds[obj.GetObjectKind().GroupVersionKind().Kind]++
		}
		assert.Equal(t, 1, kinds["ClusterLogForwarder"])
		assert.Equal(t, 1, kinds["LokiStack"])
	})

	t.Run("uses labeled hub cluster name as MCAO namespace", func(t *testing.T) {
		ctx := t.Context()
		scheme := buildTestScheme(t)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hubCluster("my-hub"), buildObjStorageSecret()).Build()

		_, clusterConfigs, err := BuildLokiStackResources(ctx, fakeClient, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
		require.NoError(t, err)
		require.Len(t, clusterConfigs, 1)
		assert.Equal(t, "my-hub", clusterConfigs[0].ClusterNamespace)
	})
}

// CLF errors should not block LokiStack install
var errSimulatedCLFGet = errors.New("simulated API server error on CLF Get")

func TestBuildLokiStackResourcesWhenCLFFails(t *testing.T) {
	ctx := t.Context()
	scheme := buildTestScheme(t)
	cmao := buildTestCMAO(globalPlacement())

	// Intercept Get calls: return a server error for CLF, pass through everything else.
	clfGetFails := interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*loggingv1.ClusterLogForwarder); ok {
				return errSimulatedCLFGet
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
	require.ErrorIs(t, clfErr, errSimulatedCLFGet)

	lsObjs, lsClusterConfig, err := BuildLokiStackResources(ctx, brokenClient, platform, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err, "BuildLokiStackResources must succeed even when CLF Get fails")

	// Verify the LokiStack is present and correctly formed.
	require.NotEmpty(t, lsObjs, "LokiStack must be returned despite CLF error")
	ls, ok := lsObjs[0].(*lokiv1.LokiStack)
	require.True(t, ok, "first returned object must be a LokiStack")
	assert.Equal(t, fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName), ls.Name)
	assert.Equal(t, addoncfg.InstallNamespace, ls.Namespace)

	// Verify the hub MCAO cluster config is present so storage is not fanned out via CMAO.
	require.Len(t, lsClusterConfig, 1, "LokiStack cluster config must be present")
	assert.Equal(t, addoncfg.HubNamespace, lsClusterConfig[0].ClusterNamespace)
	assert.Equal(t, addoncfg.LokiStacksResource, lsClusterConfig[0].Config.Resource)
}
