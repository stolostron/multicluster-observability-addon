package integration

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/defaulthubstack"
	cooresource "github.com/stolostron/multicluster-observability-addon/internal/coo/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func startDefaultHubStackManager(ctx context.Context, t *testing.T, testEnv *TestEnv) ctrl.Manager {
	t.Helper()

	testLogger := testr.New(t)
	ctrl.SetLogger(testLogger)

	disableNameValidation := true
	mgr, err := ctrl.NewManager(testEnv.Cfg, ctrl.Options{
		Scheme: testEnv.Scheme,
		Cache:  NewTestCacheOptions(),
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Controller: config.Controller{
			SkipNameValidation: &disableNameValidation,
		},
		Logger: testLogger,
	})
	require.NoError(t, err)

	require.NoError(t, defaulthubstack.SetupWithManager(mgr, testLogger))

	mgrCtx, cancelMgr := context.WithCancel(ctx)
	mgrErrCh := make(chan error, 1)
	go func() {
		mgrErrCh <- mgr.Start(mgrCtx)
	}()

	t.Cleanup(func() {
		cancelMgr()
		select {
		case mgrErr := <-mgrErrCh:
			assert.NoError(t, mgrErr, "controller manager exited with unexpected error")
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for controller manager to shutdown")
		}
	})

	require.True(t, mgr.GetCache().WaitForCacheSync(ctx), "failed waiting for manager caches to sync")
	return mgr
}

// TestDefaultHubStack_LifecycleAndDynamicWatchSelfHealing verifies DefaultHubStackReconciler
// against a real API server and controller-runtime Manager with ReaderFailOnMissingInformer enabled:
//  1. Provisioning COO operator resources (Namespace, OperatorGroup, Subscription), analytics Namespace,
//     PersesDatasources, PersesDashboards (including cardinality dashboards triggered via OnlyMetadata
//     ConfigMap watch + APIReader), and UIPlugin via Server-Side Apply.
//  2. Dynamic watch registration (registerDynamicWatches) and self-healing when a managed UIPlugin
//     or PersesDatasource is deleted externally.
//  3. Feature disablement cleanup of managed resources (cached Client for watched resources,
//     APIReader for unwatched Namespace and OperatorGroup) and orphan dashboards while
//     preserving user-created dashboards.
func TestDefaultHubStack_LifecycleAndDynamicWatchSelfHealing(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startDefaultHubStackManager(ctx, t, testEnv)

	// 1. Create cardinality rules ConfigMap (watched with builder.OnlyMetadata, read via APIReader)
	cardinalityCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "thanos-ruler-custom-rules",
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{
			"custom_rules.yaml": "record: cluster:cardinality:sum",
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cardinalityCM))

	// 2. Create AddOnDeploymentConfig enabling platform metrics UI, incident detection, and right-sizing
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{Name: addon.KeyPlatformMetricsCollection, Value: string(addon.PrometheusAgentV1alpha1)},
				{Name: addon.KeyPlatformMetricsUI, Value: string(addon.UIPluginV1alpha1)},
				{Name: addon.KeyMetricsHubHostname, Value: "https://thanos-querier.example.com"},
				{Name: addon.KeyPlatformIncidentDetection, Value: string(addon.UIPluginV1alpha1)},
				{Name: addon.KeyRightSizingDelegated, Value: "true"},
				{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	// 3. Verify COO operator installation (Namespace, OperatorGroup, Subscription) and analytics Namespace
	require.Eventually(t, func() bool {
		sub := &operatorsv1alpha1.Subscription{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{
			Name:      addoncfg.CooSubscriptionName,
			Namespace: addoncfg.CooSubscriptionNamespace,
		}, sub); err != nil {
			return false
		}
		og := &operatorsv1.OperatorGroup{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{
			Name:      addoncfg.CooSubscriptionNamespace,
			Namespace: addoncfg.CooSubscriptionNamespace,
		}, og); err != nil {
			return false
		}
		analyticsNS := &corev1.Namespace{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{Name: addoncfg.AnalyticsNamespace}, analyticsNS); err != nil {
			return false
		}
		return sub.Spec != nil && sub.Spec.Channel == addoncfg.CooSubscriptionChannel
	}, 15*time.Second, 200*time.Millisecond, "expected COO Subscription, OperatorGroup, and analytics Namespace to be provisioned")

	// 4. Verify UIPlugin, PersesDatasources, and PersesDashboards (including cardinality and analytics dashboards)
	var initialUIPluginUID types.UID
	var initialDSUID types.UID
	dsKey := types.NamespacedName{
		Name:      "rbac-query-proxy-datasource",
		Namespace: addoncfg.InstallNamespace,
	}
	require.Eventually(t, func() bool {
		uip := &uiplugin.UIPlugin{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{Name: "monitoring"}, uip); err != nil {
			return false
		}
		if uip.Spec.Monitoring == nil ||
			uip.Spec.Monitoring.Perses == nil || !uip.Spec.Monitoring.Perses.Enabled ||
			uip.Spec.Monitoring.ACM == nil || !uip.Spec.Monitoring.ACM.Enabled ||
			uip.Spec.Monitoring.Incidents == nil || !uip.Spec.Monitoring.Incidents.Enabled {
			return false
		}
		initialUIPluginUID = uip.UID

		ds := &persesv1.PersesDatasource{}
		if err := testEnv.K8sClient.Get(ctx, dsKey, ds); err != nil {
			return false
		}
		initialDSUID = ds.UID

		analyticsDS := &persesv1.PersesDatasource{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{
			Name:      "rbac-query-proxy-datasource",
			Namespace: addoncfg.AnalyticsNamespace,
		}, analyticsDS); err != nil {
			return false
		}

		// Verify cardinality dashboard was provisioned (proving ConfigMap APIReader path succeeded)
		cardDB := &persesv1.PersesDashboard{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{
			Name:      "acm-metrics-cardinality-overview",
			Namespace: addoncfg.InstallNamespace,
		}, cardDB); err != nil {
			return false
		}

		// Verify analytics dashboards were provisioned in analytics namespace
		analyticsDBList := &persesv1.PersesDashboardList{}
		if err := testEnv.K8sClient.List(ctx, analyticsDBList, client.InNamespace(addoncfg.AnalyticsNamespace)); err != nil {
			return false
		}
		return len(analyticsDBList.Items) > 0
	}, 20*time.Second, 250*time.Millisecond, "expected UIPlugin, PersesDatasources, cardinality dashboard, and analytics dashboards to be created")

	// 5. Verify Dynamic Watch Self-Healing: deleting a managed UIPlugin and PersesDatasource
	// triggers the dynamically registered watches and recreates them via Server-Side Apply.
	uipToDelete := &uiplugin.UIPlugin{ObjectMeta: metav1.ObjectMeta{Name: "monitoring"}}
	require.NoError(t, testEnv.K8sClient.Delete(ctx, uipToDelete))

	dsToDelete := &persesv1.PersesDatasource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsKey.Name,
			Namespace: dsKey.Namespace,
		},
	}
	require.NoError(t, testEnv.K8sClient.Delete(ctx, dsToDelete))

	require.Eventually(t, func() bool {
		uip := &uiplugin.UIPlugin{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{Name: "monitoring"}, uip); err != nil {
			return false
		}
		ds := &persesv1.PersesDatasource{}
		if err := testEnv.K8sClient.Get(ctx, dsKey, ds); err != nil {
			return false
		}
		return uip.UID != initialUIPluginUID && ds.UID != initialDSUID &&
			uip.Labels[addoncfg.ManagedByK8sLabelKey] == cooresource.ManagedByLabelValue
	}, 15*time.Second, 200*time.Millisecond, "expected dynamically watched UIPlugin and PersesDatasource to be self-healed after deletion")

	// 6. Seed a user-created PersesDashboard (without MCOA managed-by label or known name)
	// and verify feature disablement cleans up all MCOA-managed resources while preserving the user dashboard.
	userDashboard := &persesv1.PersesDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-custom-dashboard",
			Namespace: addoncfg.InstallNamespace,
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, userDashboard))

	require.NoError(t, testEnv.K8sClient.Get(ctx, client.ObjectKeyFromObject(aodc), aodc))
	aodc.Spec.CustomizedVariables = nil
	require.NoError(t, testEnv.K8sClient.Update(ctx, aodc))

	require.Eventually(t, func() bool {
		// UIPlugin should be deleted
		uip := &uiplugin.UIPlugin{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{Name: "monitoring"}, uip); !apierrors.IsNotFound(err) {
			return false
		}

		// Datasources should be deleted
		if err := testEnv.K8sClient.Get(ctx, dsKey, &persesv1.PersesDatasource{}); !apierrors.IsNotFound(err) {
			return false
		}

		// COO Subscription and OperatorGroup should be deleted
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{
			Name:      addoncfg.CooSubscriptionName,
			Namespace: addoncfg.CooSubscriptionNamespace,
		}, &operatorsv1alpha1.Subscription{}); !apierrors.IsNotFound(err) {
			return false
		}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{
			Name:      addoncfg.CooSubscriptionNamespace,
			Namespace: addoncfg.CooSubscriptionNamespace,
		}, &operatorsv1.OperatorGroup{}); !apierrors.IsNotFound(err) {
			return false
		}

		// Analytics namespace should be deleted (or terminating in envtest since namespace controller is absent)
		analyticsNS := &corev1.Namespace{}
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{Name: addoncfg.AnalyticsNamespace}, analyticsNS); err == nil {
			if analyticsNS.DeletionTimestamp.IsZero() {
				return false
			}
		} else if !apierrors.IsNotFound(err) {
			return false
		}

		// Only the user-created dashboard should remain in the install namespace
		dbList := &persesv1.PersesDashboardList{}
		if err := testEnv.K8sClient.List(ctx, dbList, client.InNamespace(addoncfg.InstallNamespace)); err != nil {
			return false
		}
		return len(dbList.Items) == 1 && slices.ContainsFunc(dbList.Items, func(d persesv1.PersesDashboard) bool {
			return d.Name == userDashboard.Name
		})
	}, 20*time.Second, 250*time.Millisecond, "expected all MCOA-managed hub resources to be cleaned up and user dashboard preserved")
}
