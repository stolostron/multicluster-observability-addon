package resourcecreator

import (
	"context"
	"fmt"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	"github.com/stolostron/cluster-lifecycle-api/constants"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func loggingReconcileScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, addonv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, lokiv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, certmanagerv1.AddToScheme(scheme))
	return scheme
}

func patchApplyAsCreateOrUpdate() interceptor.Funcs {
	return interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if patch.Type() != types.ApplyPatchType {
				return c.Patch(ctx, obj, patch, opts...)
			}
			existing := obj.DeepCopyObject().(client.Object)
			err := c.Get(ctx, client.ObjectKeyFromObject(obj), existing)
			if apierrors.IsNotFound(err) {
				return c.Create(ctx, obj)
			}
			if err != nil {
				return err
			}
			obj.SetResourceVersion(existing.GetResourceVersion())
			return c.Update(ctx, obj)
		},
	}
}

func testCMAO() *addonv1beta1.ClusterManagementAddOn {
	return &addonv1beta1.ClusterManagementAddOn{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterManagementAddOn",
			APIVersion: addonv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  "cmao-uid",
		},
	}
}

func testCMAOWithPlacements(configs ...addonv1beta1.AddOnConfig) *addonv1beta1.ClusterManagementAddOn {
	cmao := testCMAO()
	cmao.Spec.InstallStrategy.Placements = []addonv1beta1.PlacementStrategy{
		{
			PlacementRef: addoncfg.GlobalPlacementRef,
			Configs:      configs,
		},
	}
	return cmao
}

func testMCAO(namespace string, configs ...addonv1beta1.AddOnConfig) *addonv1beta1.ManagedClusterAddOn {
	return &addonv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: namespace,
		},
		Spec: addonv1beta1.ManagedClusterAddOnSpec{Configs: configs},
	}
}

func clfAddonConfig() addonv1beta1.AddOnConfig {
	return addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    "observability.openshift.io",
			Resource: addoncfg.ClusterLogForwardersResource,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "mcoa-default-global",
			Namespace: addoncfg.InstallNamespace,
		},
	}
}

func testHubCluster() *clusterv1.ManagedCluster {
	return &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   addoncfg.HubNamespace,
			Labels: map[string]string{constants.SelfManagedClusterLabelKey: "true"},
		},
	}
}

func testHubMCAO(configs ...addonv1beta1.AddOnConfig) *addonv1beta1.ManagedClusterAddOn {
	return &addonv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.HubNamespace,
		},
		Spec: addonv1beta1.ManagedClusterAddOnSpec{Configs: configs},
	}
}

func testObjStorageSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifests.DefaultStorageObjStorageSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
	}
}

func lokiStackAddonConfig() addonv1beta1.AddOnConfig {
	return addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    lokiv1.GroupVersion.Group,
			Resource: addoncfg.LokiStacksResource,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Name:      "mcoa-default-global",
			Namespace: addoncfg.InstallNamespace,
		},
	}
}

func newLoggingReconciler(t *testing.T, objs ...client.Object) *ResourceCreatorReconciler {
	t.Helper()
	scheme := loggingReconcileScheme(t)
	return &ResourceCreatorReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			WithInterceptorFuncs(patchApplyAsCreateOrUpdate()).
			Build(),
		Log:    logr.Discard(),
		Scheme: scheme,
	}
}

func TestIsHubManagedClusterAddOn(t *testing.T) {
	scheme := loggingReconcileScheme(t)
	spokeMCAO := testMCAO("spoke-1")
	otherAddon := &addonv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: "other-addon", Namespace: addoncfg.HubNamespace},
	}
	labeledHub := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "my-hub",
			Labels: map[string]string{constants.SelfManagedClusterLabelKey: "true"},
		},
	}

	t.Run("matches local-cluster when it is the labeled hub", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(testHubCluster()).Build()
		assert.True(t, isHubManagedClusterAddOn(t.Context(), k8s, testHubMCAO()))
		assert.False(t, isHubManagedClusterAddOn(t.Context(), k8s, spokeMCAO))
		assert.False(t, isHubManagedClusterAddOn(t.Context(), k8s, otherAddon))
	})

	t.Run("falls back to local-cluster when no hub is labeled", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).Build()
		assert.True(t, isHubManagedClusterAddOn(t.Context(), k8s, testHubMCAO()))
		assert.False(t, isHubManagedClusterAddOn(t.Context(), k8s, spokeMCAO))
	})

	t.Run("matches the labeled hub cluster name", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(labeledHub).Build()
		assert.True(t, isHubManagedClusterAddOn(t.Context(), k8s, testMCAO("my-hub")))
		assert.False(t, isHubManagedClusterAddOn(t.Context(), k8s, testHubMCAO()))
		assert.False(t, isHubManagedClusterAddOn(t.Context(), k8s, spokeMCAO))
	})

	t.Run("falls back to local-cluster when listing clusters fails", func(t *testing.T) {
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return fmt.Errorf("list failed")
			},
		}).Build()
		assert.True(t, isHubManagedClusterAddOn(t.Context(), k8s, testHubMCAO()))
		assert.False(t, isHubManagedClusterAddOn(t.Context(), k8s, spokeMCAO))
	})
}

func TestApplyStorageAddonConfig(t *testing.T) {
	lokiCfg := lokiStackAddonConfig()
	clfCfg := clfAddonConfig()

	t.Run("attaches LokiStack config and keeps unrelated configs", func(t *testing.T) {
		r := newLoggingReconciler(t, testHubMCAO(clfCfg))
		result, err := r.applyStorageAddonConfig(t.Context(), addoncfg.HubNamespace, []addonv1beta1.AddOnConfig{lokiCfg})
		require.NoError(t, err)
		assert.True(t, result.IsZero())

		got := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace}, got))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg, lokiCfg}, got.Spec.Configs)
	})

	t.Run("requeues when hub MCAO is missing", func(t *testing.T) {
		r := newLoggingReconciler(t)
		result, err := r.applyStorageAddonConfig(t.Context(), addoncfg.HubNamespace, []addonv1beta1.AddOnConfig{lokiCfg})
		require.NoError(t, err)
		assert.Equal(t, addoncfg.DefaultContextTimeout, result.RequeueAfter)
	})

	t.Run("strips LokiStack configs when desired is empty", func(t *testing.T) {
		r := newLoggingReconciler(t, testHubMCAO(clfCfg, lokiCfg))
		result, err := r.applyStorageAddonConfig(t.Context(), addoncfg.HubNamespace, nil)
		require.NoError(t, err)
		assert.True(t, result.IsZero())

		got := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace}, got))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, got.Spec.Configs)
	})
}

func TestReconcileLoggingStorage(t *testing.T) {
	optsOn := addon.Options{Platform: addon.PlatformOptions{Logs: addon.LogsOptions{DefaultStack: true}}}
	optsOff := addon.Options{}

	t.Run("applies LokiStack, storage cert, and hub MCAO config", func(t *testing.T) {
		cmao := testCMAO()
		r := newLoggingReconciler(t, cmao, testHubCluster(), testHubMCAO(), testObjStorageSecret())

		result, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOn)
		require.NoError(t, err)
		assert.True(t, result.IsZero())

		ls := &lokiv1.LokiStack{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{
			Name:      "mcoa-default-global",
			Namespace: addoncfg.InstallNamespace,
		}, ls))
		assert.Equal(t, lokiv1.ManagementStateUnmanaged, ls.Spec.ManagementState)

		cert := &certmanagerv1.Certificate{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{
			Name:      manifests.DefaultStorageMTLSSecretName,
			Namespace: addoncfg.HubNamespace,
		}, cert))

		mcAddon := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace}, mcAddon))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{lokiStackAddonConfig()}, mcAddon.Spec.Configs)
	})

	t.Run("does not attach LokiStack to a spoke MCAO", func(t *testing.T) {
		spokeMCAO := testMCAO("spoke-1")
		cmao := testCMAO()
		r := newLoggingReconciler(t, cmao, testHubCluster(), testHubMCAO(), spokeMCAO, testObjStorageSecret())

		_, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOn)
		require.NoError(t, err)

		got := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: "spoke-1"}, got))
		assert.Empty(t, got.Spec.Configs)
	})

	t.Run("strips leftover LokiStack from CMAO placements and spoke MCAOs", func(t *testing.T) {
		clfCfg := clfAddonConfig()
		lokiCfg := lokiStackAddonConfig()
		cmao := testCMAOWithPlacements(clfCfg, lokiCfg)
		spoke := testMCAO("spoke-1", clfCfg, lokiCfg)
		r := newLoggingReconciler(t, cmao, testHubCluster(), testHubMCAO(), spoke, testObjStorageSecret())

		result, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOn)
		require.NoError(t, err)
		assert.True(t, result.IsZero())

		gotCMAO := &addonv1beta1.ClusterManagementAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name}, gotCMAO))
		require.Len(t, gotCMAO.Spec.InstallStrategy.Placements, 1)
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, gotCMAO.Spec.InstallStrategy.Placements[0].Configs)

		gotSpoke := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: "spoke-1"}, gotSpoke))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, gotSpoke.Spec.Configs)

		gotHub := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace}, gotHub))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{lokiCfg}, gotHub.Spec.Configs)
	})

	t.Run("strips leftover CMAO and spoke LokiStack when object storage secret is missing", func(t *testing.T) {
		clfCfg := clfAddonConfig()
		lokiCfg := lokiStackAddonConfig()
		cmao := testCMAOWithPlacements(clfCfg, lokiCfg)
		spoke := testMCAO("spoke-1", clfCfg, lokiCfg)
		hub := testHubMCAO(lokiCfg)
		r := newLoggingReconciler(t, cmao, testHubCluster(), hub, spoke)

		_, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOn)
		require.Error(t, err)

		gotCMAO := &addonv1beta1.ClusterManagementAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name}, gotCMAO))
		require.Len(t, gotCMAO.Spec.InstallStrategy.Placements, 1)
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, gotCMAO.Spec.InstallStrategy.Placements[0].Configs)

		gotSpoke := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: "spoke-1"}, gotSpoke))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, gotSpoke.Spec.Configs)

		gotHub := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace}, gotHub))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{lokiCfg}, gotHub.Spec.Configs)
	})

	t.Run("strips leftover spoke LokiStack while requeueing for a missing hub MCAO", func(t *testing.T) {
		clfCfg := clfAddonConfig()
		lokiCfg := lokiStackAddonConfig()
		cmao := testCMAOWithPlacements(lokiCfg)
		spoke := testMCAO("spoke-1", clfCfg, lokiCfg)
		r := newLoggingReconciler(t, cmao, testHubCluster(), spoke, testObjStorageSecret())

		result, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOn)
		require.NoError(t, err)
		assert.Equal(t, addoncfg.DefaultContextTimeout, result.RequeueAfter)

		gotSpoke := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: "spoke-1"}, gotSpoke))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, gotSpoke.Spec.Configs)
	})

	t.Run("requeues when hub MCAO is missing", func(t *testing.T) {
		cmao := testCMAO()
		r := newLoggingReconciler(t, cmao, testHubCluster(), testObjStorageSecret())

		result, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOn)
		require.NoError(t, err)
		assert.Equal(t, addoncfg.DefaultContextTimeout, result.RequeueAfter)
	})

	t.Run("strips hub MCAO LokiStack config when default stack is off", func(t *testing.T) {
		cmao := testCMAO()
		r := newLoggingReconciler(t, cmao, testHubCluster(), testHubMCAO(lokiStackAddonConfig()))

		result, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOff)
		require.NoError(t, err)
		assert.True(t, result.IsZero())

		got := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace}, got))
		assert.Empty(t, got.Spec.Configs)
	})

	t.Run("strips leftover CMAO and spoke LokiStack when default stack is off", func(t *testing.T) {
		clfCfg := clfAddonConfig()
		lokiCfg := lokiStackAddonConfig()
		cmao := testCMAOWithPlacements(clfCfg, lokiCfg)
		spoke := testMCAO("spoke-1", clfCfg, lokiCfg)
		r := newLoggingReconciler(t, cmao, testHubCluster(), testHubMCAO(lokiCfg), spoke)

		result, err := r.reconcileLoggingStorage(t.Context(), cmao, optsOff)
		require.NoError(t, err)
		assert.True(t, result.IsZero())

		gotCMAO := &addonv1beta1.ClusterManagementAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name}, gotCMAO))
		require.Len(t, gotCMAO.Spec.InstallStrategy.Placements, 1)
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, gotCMAO.Spec.InstallStrategy.Placements[0].Configs)

		gotSpoke := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: "spoke-1"}, gotSpoke))
		require.ElementsMatch(t, []addonv1beta1.AddOnConfig{clfCfg}, gotSpoke.Spec.Configs)

		gotHub := &addonv1beta1.ManagedClusterAddOn{}
		require.NoError(t, r.Get(t.Context(), client.ObjectKey{Name: addoncfg.Name, Namespace: addoncfg.HubNamespace}, gotHub))
		assert.Empty(t, gotHub.Spec.Configs)
	})
}
