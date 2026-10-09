package integration

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	addon "github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/resourcecreator"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const defaultAgentName = "mcoa-default-platform-metrics-collector-global-default"

func setupTestNamespace(ctx context.Context, t *testing.T, k8sClient client.Client, name string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := k8sClient.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}
}

func startResourceCreatorManager(ctx context.Context, t *testing.T, testEnv *TestEnv) ctrl.Manager {
	t.Helper()

	testLogger := testr.New(t)
	ctrl.SetLogger(testLogger)

	disableNameValidation := true
	mgr, err := ctrl.NewManager(testEnv.Cfg, ctrl.Options{
		Scheme: testEnv.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Controller: config.Controller{
			SkipNameValidation: &disableNameValidation,
		},
		Logger: testLogger,
	})
	require.NoError(t, err)

	require.NoError(t, resourcecreator.SetupWithManager(mgr, testLogger))

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

func defaultImageOverridesData() map[string]string {
	return map[string]string{
		"prometheus_config_reloader":    "quay.io/custom-repo/reloader:v0.1.0",
		"kube_rbac_proxy":               "quay.io/custom-repo/kube-rbac-proxy:v0.18.1",
		"obo_prometheus_rhel9_operator": "quay.io/custom-repo/obo-operator:v0.1.0",
		"kube_state_metrics":            "quay.io/custom-repo/ksm:v2.10.0",
		"node_exporter":                 "quay.io/custom-repo/node-exporter:v1.8.0",
		"prometheus":                    "quay.io/custom-repo/prometheus:v2.55.0",
		"endpoint_monitoring_operator":  "quay.io/custom-repo/emo:v0.1.0",
	}
}

// TestResourceCreator_DefaultMetricsPipeline verifies that creating AODC and CMAO provisions
// the default PrometheusAgent via Server-Side Apply, updates CMAO placement configs,
// and self-heals upon resource deletion.
func TestResourceCreator_DefaultMetricsPipeline(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startResourceCreatorManager(ctx, t, testEnv)

	// 1. Create AddOnDeploymentConfig enabling platform metrics
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformMetricsCollection,
					Value: string(addon.PrometheusAgentV1alpha1),
				},
				{
					Name:  addon.KeyMetricsHubHostname,
					Value: "thanos.apps.hub-cluster.example.com",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	// 2. Create ClusterManagementAddOn with placement "global"
	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Type: "Placements",
				Placements: []addonv1beta1.PlacementStrategy{
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "global",
							Namespace: addoncfg.InstallNamespace,
						},
					},
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cmao))

	// 3. Assert PrometheusAgent was created by the manager
	promAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	promAgentKey := types.NamespacedName{
		Name:      defaultAgentName,
		Namespace: addoncfg.InstallNamespace,
	}

	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, promAgentKey, promAgent); err != nil {
			return false
		}
		return promAgent.Labels[addoncfg.ManagedByK8sLabelKey] == addoncfg.Name &&
			promAgent.Labels[addoncfg.ComponentK8sLabelKey] == "platform-metrics-collector" &&
			strings.Contains(promAgent.Annotations[addoncfg.PlacementAnnotationKey], "open-cluster-management-observability/global")
	}, 10*time.Second, 100*time.Millisecond, "expected platform PrometheusAgent to be created and labeled")

	// 4. Assert ClusterManagementAddOn configs were updated by the manager to reference the PrometheusAgent
	expectedAgentConfig := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
			Resource: cooprometheusv1alpha1.PrometheusAgentName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Namespace: addoncfg.InstallNamespace,
			Name:      defaultAgentName,
		},
	}

	updatedCMAO := &addonv1beta1.ClusterManagementAddOn{}
	cmaoKey := types.NamespacedName{Name: addoncfg.Name}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, cmaoKey, updatedCMAO); err != nil {
			return false
		}
		if len(updatedCMAO.Spec.InstallStrategy.Placements) == 0 {
			return false
		}
		placement := updatedCMAO.Spec.InstallStrategy.Placements[0]
		return placement.Name == "global" && slices.Contains(placement.Configs, expectedAgentConfig)
	}, 10*time.Second, 100*time.Millisecond, "expected CMAO global placement configs to contain the default PrometheusAgent reference")

	// 5. Test Event Trigger: Self-Healing / Drift Correction
	// Deleting the created PrometheusAgent must trigger enqueueForMCOAOwnedResources,
	// causing the manager to automatically recreate the agent.
	require.NoError(t, testEnv.K8sClient.Delete(ctx, promAgent))

	recreatedAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, promAgentKey, recreatedAgent); err != nil {
			return false
		}
		return recreatedAgent.UID != promAgent.UID &&
			recreatedAgent.Labels[addoncfg.ManagedByK8sLabelKey] == addoncfg.Name
	}, 10*time.Second, 100*time.Millisecond, "expected manager to self-heal and recreate deleted PrometheusAgent")
}

// TestResourceCreator_CustomConfiguration_SSAIdempotent verifies that mutating configuration in
// AddOnDeploymentConfig dynamically updates existing PrometheusAgent specs via SSA without field conflicts.
func TestResourceCreator_CustomConfiguration_SSAIdempotent(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startResourceCreatorManager(ctx, t, testEnv)

	// Initial setup
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformMetricsCollection,
					Value: string(addon.PrometheusAgentV1alpha1),
				},
				{
					Name:  addon.KeyMetricsHubHostname,
					Value: "initial-hub.example.com",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Type: "Placements",
				Placements: []addonv1beta1.PlacementStrategy{
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "global",
							Namespace: addoncfg.InstallNamespace,
						},
					},
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cmao))

	promAgentKey := types.NamespacedName{
		Name:      defaultAgentName,
		Namespace: addoncfg.InstallNamespace,
	}

	// 1. Wait for initial creation
	promAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, promAgentKey, promAgent); err != nil {
			return false
		}
		return len(promAgent.Spec.RemoteWrite) > 0 &&
			strings.Contains(string(promAgent.Spec.RemoteWrite[0].URL), "initial-hub.example.com")
	}, 10*time.Second, 100*time.Millisecond, "expected PrometheusAgent to be created with initial remote-write endpoint")

	// 2. Trigger Event: Mutate AODC with updated hostname
	updatedAODC := &addonv1beta1.AddOnDeploymentConfig{}
	require.NoError(t, testEnv.K8sClient.Get(ctx, client.ObjectKeyFromObject(aodc), updatedAODC))
	updatedAODC.Spec.CustomizedVariables = []addonv1beta1.CustomizedVariable{
		{
			Name:  addon.KeyPlatformMetricsCollection,
			Value: string(addon.PrometheusAgentV1alpha1),
		},
		{
			Name:  addon.KeyMetricsHubHostname,
			Value: "updated-thanos-hub.example.com",
		},
	}
	require.NoError(t, testEnv.K8sClient.Update(ctx, updatedAODC))

	// 3. Manager reacts to AODC update event, executes SSA to update PrometheusAgent spec without conflict
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, promAgentKey, promAgent); err != nil {
			return false
		}
		return len(promAgent.Spec.RemoteWrite) > 0 &&
			strings.Contains(string(promAgent.Spec.RemoteWrite[0].URL), "updated-thanos-hub.example.com")
	}, 10*time.Second, 100*time.Millisecond, "expected PrometheusAgent remote-write URL to be updated via SSA")
}

// TestResourceCreator_ImageOverrides verifies that custom container images defined in the global images
// ConfigMap are injected into the PrometheusAgent spec during reconciliation.
func TestResourceCreator_ImageOverrides(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startResourceCreatorManager(ctx, t, testEnv)

	// 1. Create image overrides ConfigMap with complete required schema
	imgConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mconfig.ImagesConfigMapObjKey.Name,
			Namespace: mconfig.ImagesConfigMapObjKey.Namespace,
		},
		Data: defaultImageOverridesData(),
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, imgConfigMap))

	// 2. Create AODC and CMAO to trigger reconciliation
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformMetricsCollection,
					Value: string(addon.PrometheusAgentV1alpha1),
				},
				{
					Name:  addon.KeyMetricsHubHostname,
					Value: "thanos.example.com",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Type: "Placements",
				Placements: []addonv1beta1.PlacementStrategy{
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "global",
							Namespace: addoncfg.InstallNamespace,
						},
					},
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cmao))

	promAgentKey := types.NamespacedName{
		Name:      defaultAgentName,
		Namespace: addoncfg.InstallNamespace,
	}

	// 3. Manager reconciles and injects custom image override
	promAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, promAgentKey, promAgent); err != nil {
			return false
		}
		return promAgent.Spec.Image != nil && *promAgent.Spec.Image == "quay.io/custom-repo/prometheus:v2.55.0"
	}, 10*time.Second, 100*time.Millisecond, "expected PrometheusAgent to have custom overridden image injected")
}

// TestResourceCreator_TerminalErrorOnMalformedAODC verifies that reconciling a malformed AddOnDeploymentConfig
// isolates the error cleanly without persisting partial or corrupted resources into etcd.
func TestResourceCreator_TerminalErrorOnMalformedAODC(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startResourceCreatorManager(ctx, t, testEnv)

	// Create AODC with invalid hostname (e.g. only a colon ":")
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformMetricsCollection,
					Value: string(addon.PrometheusAgentV1alpha1),
				},
				{
					Name:  addon.KeyMetricsHubHostname,
					Value: ":",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Type: "Placements",
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cmao))

	// Black-box failure isolation assertion:
	// Verify that despite manager attempting reconciliation, no PrometheusAgent
	// is ever created or written into etcd.
	promAgentKey := types.NamespacedName{
		Name:      defaultAgentName,
		Namespace: addoncfg.InstallNamespace,
	}
	assert.Never(t, func() bool {
		promAgent := &cooprometheusv1alpha1.PrometheusAgent{}
		err := testEnv.K8sClient.Get(ctx, promAgentKey, promAgent)
		return err == nil
	}, 2*time.Second, 100*time.Millisecond, "no PrometheusAgent should ever be created when configuration is invalid")
}

// TestResourceCreator_UserModifications_SSAFieldPreservation verifies the SSA co-ownership contract:
// user-managed fields (resources, logLevel, custom labels) are preserved across reconciliations,
// while MCOA-managed invariant fields (serviceAccountName) are reverted upon drift.
func TestResourceCreator_UserModifications_SSAFieldPreservation(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startResourceCreatorManager(ctx, t, testEnv)

	// 1. Initial provisioning via AODC and CMAO
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformMetricsCollection,
					Value: string(addon.PrometheusAgentV1alpha1),
				},
				{
					Name:  addon.KeyMetricsHubHostname,
					Value: "thanos.example.com",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Type: "Placements",
				Placements: []addonv1beta1.PlacementStrategy{
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "global",
							Namespace: addoncfg.InstallNamespace,
						},
					},
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cmao))

	promAgentKey := types.NamespacedName{
		Name:      defaultAgentName,
		Namespace: addoncfg.InstallNamespace,
	}

	// 2. Wait for initial creation of PrometheusAgent
	promAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	require.Eventually(t, func() bool {
		return testEnv.K8sClient.Get(ctx, promAgentKey, promAgent) == nil
	}, 10*time.Second, 100*time.Millisecond, "expected PrometheusAgent to be created")

	// 3. User modifies the live PrometheusAgent directly:
	//    - Customizes unmanaged fields: spec.resources.requests, spec.logLevel, custom label
	//    - Tampers with MCOA-enforced field: spec.serviceAccountName
	require.NoError(t, testEnv.K8sClient.Get(ctx, promAgentKey, promAgent))
	customMemory := resource.MustParse("500Mi")
	if promAgent.Spec.Resources.Requests == nil {
		promAgent.Spec.Resources.Requests = corev1.ResourceList{}
	}
	promAgent.Spec.Resources.Requests[corev1.ResourceMemory] = customMemory
	promAgent.Spec.LogLevel = "debug"
	if promAgent.Labels == nil {
		promAgent.Labels = map[string]string{}
	}
	promAgent.Labels["user.custom.io/tier"] = "production"
	promAgent.Spec.ServiceAccountName = "tampered-malicious-sa"

	require.NoError(t, testEnv.K8sClient.Update(ctx, promAgent))

	// 4. Assert SSA Invariant Reversion and User Field Preservation:
	//    The controller manager detects the update event, identifies drift on ServiceAccountName,
	//    executes SSA to restore the enforced ServiceAccountName, while preserving the user's
	//    custom resources, logLevel, and custom labels.
	reconciledAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, promAgentKey, reconciledAgent); err != nil {
			return false
		}
		// Invariant field must be reverted by MCOA SSA
		serviceAccountRestored := reconciledAgent.Spec.ServiceAccountName == mconfig.PlatformMetricsCollectorApp
		// User fields must be preserved by SSA
		userMemoryPreserved := reconciledAgent.Spec.Resources.Requests != nil &&
			reconciledAgent.Spec.Resources.Requests[corev1.ResourceMemory].Equal(customMemory)
		userLogLevelPreserved := reconciledAgent.Spec.LogLevel == "debug"
		userLabelPreserved := reconciledAgent.Labels["user.custom.io/tier"] == "production"

		return serviceAccountRestored && userMemoryPreserved && userLogLevelPreserved && userLabelPreserved
	}, 10*time.Second, 100*time.Millisecond, "expected SSA to revert tampered serviceAccountName while preserving user-customized fields")
}

// TestResourceCreator_UserDefinedScrapeConfig_SSAAndPlacementPropagation verifies that user-defined ScrapeConfigs
// are detected, have SSA invariants enforced (HTTPS scheme, backup label, static configs) while preserving
// user fields (interval, path, labels), and are dynamically injected into CMAO placement configs.
func TestResourceCreator_UserDefinedScrapeConfig_SSAAndPlacementPropagation(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startResourceCreatorManager(ctx, t, testEnv)

	// 1. Initial provisioning via AODC and CMAO with MCO controller owner reference
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformMetricsCollection,
					Value: string(addon.PrometheusAgentV1alpha1),
				},
				{
					Name:  addon.KeyMetricsHubHostname,
					Value: "thanos.example.com",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	mcoUID := types.UID("test-mco-uid-123")
	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "observability.open-cluster-management.io/v1beta2",
					Kind:       "MultiClusterObservability",
					Name:       "observability",
					UID:        mcoUID,
					Controller: ptr.To(true),
				},
			},
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Type: "Placements",
				Placements: []addonv1beta1.PlacementStrategy{
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "global",
							Namespace: addoncfg.InstallNamespace,
						},
					},
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cmao))

	// Wait for default PrometheusAgent to be created initially
	promAgentKey := types.NamespacedName{
		Name:      defaultAgentName,
		Namespace: addoncfg.InstallNamespace,
	}
	require.Eventually(t, func() bool {
		return testEnv.K8sClient.Get(ctx, promAgentKey, &cooprometheusv1alpha1.PrometheusAgent{}) == nil
	}, 10*time.Second, 100*time.Millisecond, "expected PrometheusAgent to be created initially")

	// 2. Create user-defined ScrapeConfig:
	//    - Labels: part-of MCOA, component: metrics-collector, custom label user.io/app: custom-service
	//    - Annotation: placement targeting global
	//    - Spec: custom scrapeInterval: 45s, custom metricsPath: /metrics/custom,
	//      and tampered invariant scheme: HTTP (desired is HTTPS).
	scName := "custom-app-metrics"
	userSC := &cooprometheusv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scName,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
				addoncfg.ComponentK8sLabelKey: mconfig.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey],
				"user.io/app":                 "custom-service",
			},
			Annotations: map[string]string{
				addoncfg.PlacementAnnotationKey: fmt.Sprintf("%s/global", addoncfg.InstallNamespace),
			},
		},
		Spec: cooprometheusv1alpha1.ScrapeConfigSpec{
			ScrapeInterval: ptr.To(cooprometheusv1.Duration("45s")),
			MetricsPath:    ptr.To("/metrics/custom"),
			Scheme:         ptr.To(cooprometheusv1.SchemeHTTP),
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, userSC))

	// 3. Verify that the controller manager catches the ScrapeConfig event via enqueueForMCOControlledResources
	//    and executes SSA:
	//    - Invariant scheme is restored to HTTPS
	//    - Invariant backup label is applied
	//    - Invariant staticConfigs are enforced
	//    - User's scrapeInterval, metricsPath, and custom labels are preserved
	scKey := types.NamespacedName{Name: scName, Namespace: addoncfg.InstallNamespace}
	reconciledSC := &cooprometheusv1alpha1.ScrapeConfig{}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, scKey, reconciledSC); err != nil {
			return false
		}
		schemeEnforced := reconciledSC.Spec.Scheme != nil && *reconciledSC.Spec.Scheme == cooprometheusv1.SchemeHTTPS
		backupLabelSet := reconciledSC.Labels[addoncfg.BackupLabelKey] == addoncfg.BackupLabelValue
		staticConfigsEnforced := len(reconciledSC.Spec.StaticConfigs) == 1 &&
			len(reconciledSC.Spec.StaticConfigs[0].Targets) == 1 &&
			reconciledSC.Spec.StaticConfigs[0].Targets[0] == "not-configurable"

		intervalPreserved := reconciledSC.Spec.ScrapeInterval != nil && *reconciledSC.Spec.ScrapeInterval == "45s"
		pathPreserved := reconciledSC.Spec.MetricsPath != nil && *reconciledSC.Spec.MetricsPath == "/metrics/custom"
		userLabelPreserved := reconciledSC.Labels["user.io/app"] == "custom-service"

		return schemeEnforced && backupLabelSet && staticConfigsEnforced &&
			intervalPreserved && pathPreserved && userLabelPreserved
	}, 10*time.Second, 100*time.Millisecond, "expected SSA to enforce invariants while preserving user fields on ScrapeConfig")

	// 4. Verify that CMAO placement configs were updated to include the user-defined ScrapeConfig
	expectedSCConfig := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    "monitoring.rhobs",
			Resource: cooprometheusv1alpha1.ScrapeConfigName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Namespace: addoncfg.InstallNamespace,
			Name:      scName,
		},
	}
	updatedCMAO := &addonv1beta1.ClusterManagementAddOn{}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, types.NamespacedName{Name: addoncfg.Name}, updatedCMAO); err != nil {
			return false
		}
		for _, placement := range updatedCMAO.Spec.InstallStrategy.Placements {
			if placement.Name == "global" && slices.Contains(placement.Configs, expectedSCConfig) {
				return true
			}
		}
		return false
	}, 10*time.Second, 100*time.Millisecond, "expected CMAO placement configs to include user-defined ScrapeConfig")

	// 5. User mutation test: user updates scrapeInterval to 60s
	require.NoError(t, testEnv.K8sClient.Get(ctx, scKey, reconciledSC))
	reconciledSC.Spec.ScrapeInterval = ptr.To(cooprometheusv1.Duration("60s"))
	require.NoError(t, testEnv.K8sClient.Update(ctx, reconciledSC))

	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, scKey, reconciledSC); err != nil {
			return false
		}
		return reconciledSC.Spec.ScrapeInterval != nil && *reconciledSC.Spec.ScrapeInterval == "60s" &&
			reconciledSC.Spec.Scheme != nil && *reconciledSC.Spec.Scheme == cooprometheusv1.SchemeHTTPS
	}, 10*time.Second, 100*time.Millisecond, "expected user scrapeInterval update to persist with invariants intact")
}

// TestResourceCreator_MultiPlacementOrphanGarbageCollection verifies placement-based lifecycle and garbage collection:
// removing a placement cascades deletion to single-placement orphans, retains multi-placement agents with surviving
// references, ignores unmanaged CRs, and achieves steady-state quiescence without resurrection loops.
func TestResourceCreator_MultiPlacementOrphanGarbageCollection(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)
	startResourceCreatorManager(ctx, t, testEnv)

	// 1. Initial provisioning via AODC and CMAO with three placements: "global", "regional", and "edge"
	aodc := &addonv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonv1beta1.CustomizedVariable{
				{
					Name:  addon.KeyPlatformMetricsCollection,
					Value: string(addon.PrometheusAgentV1alpha1),
				},
				{
					Name:  addon.KeyMetricsHubHostname,
					Value: "thanos.example.com",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
		},
		Spec: addonv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonv1beta1.InstallStrategy{
				Type: "Placements",
				Placements: []addonv1beta1.PlacementStrategy{
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "global",
							Namespace: addoncfg.InstallNamespace,
						},
					},
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "regional",
							Namespace: addoncfg.InstallNamespace,
						},
					},
					{
						PlacementRef: addonv1beta1.PlacementRef{
							Name:      "edge",
							Namespace: addoncfg.InstallNamespace,
						},
					},
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, cmao))

	// Wait for default global PrometheusAgent to be created
	defaultAgentKey := types.NamespacedName{
		Name:      defaultAgentName,
		Namespace: addoncfg.InstallNamespace,
	}
	require.Eventually(t, func() bool {
		return testEnv.K8sClient.Get(ctx, defaultAgentKey, &cooprometheusv1alpha1.PrometheusAgent{}) == nil
	}, 10*time.Second, 100*time.Millisecond, "expected default global PrometheusAgent to be created")

	// 2. Deploy user-defined and unmanaged PrometheusAgents:
	//    - regionalAgent: MCOA-managed, single placement targeting "regional"
	//    - multiAgent: MCOA-managed, multi-placement targeting "regional" AND "edge"
	//    - unmanagedAgent: third-party unmanaged agent targeting "regional" without MCOA part-of label
	regionalAgentName := "user-regional-collector"
	regionalAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      regionalAgentName,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
				addoncfg.ComponentK8sLabelKey: mconfig.PlatformMetricsCollectorApp,
			},
			Annotations: map[string]string{
				addoncfg.PlacementAnnotationKey: fmt.Sprintf("%s/regional", addoncfg.InstallNamespace),
			},
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				LogLevel: "info",
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, regionalAgent))

	multiAgentName := "user-multi-collector"
	multiAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      multiAgentName,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
				addoncfg.ComponentK8sLabelKey: mconfig.PlatformMetricsCollectorApp,
			},
			Annotations: map[string]string{
				addoncfg.PlacementAnnotationKey: fmt.Sprintf("%s/regional,%s/edge", addoncfg.InstallNamespace, addoncfg.InstallNamespace),
			},
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				LogLevel: "debug",
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, multiAgent))

	unmanagedAgentName := "unmanaged-collector"
	unmanagedAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      unmanagedAgentName,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				"custom.vendor.io/managed": "true",
			},
			Annotations: map[string]string{
				addoncfg.PlacementAnnotationKey: fmt.Sprintf("%s/regional", addoncfg.InstallNamespace),
			},
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				LogLevel: "warn",
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, unmanagedAgent))

	// 3. Verify that the manager reconciles both managed agents into the CMAO placement configs
	regionalAgentConfig := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
			Resource: cooprometheusv1alpha1.PrometheusAgentName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Namespace: addoncfg.InstallNamespace,
			Name:      regionalAgentName,
		},
	}
	multiAgentConfig := addonv1beta1.AddOnConfig{
		ConfigGroupResource: addonv1beta1.ConfigGroupResource{
			Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
			Resource: cooprometheusv1alpha1.PrometheusAgentName,
		},
		ConfigReferent: addonv1beta1.ConfigReferent{
			Namespace: addoncfg.InstallNamespace,
			Name:      multiAgentName,
		},
	}

	cmaoKey := types.NamespacedName{Name: addoncfg.Name}
	currentCMAO := &addonv1beta1.ClusterManagementAddOn{}
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, cmaoKey, currentCMAO); err != nil {
			return false
		}
		var regionalHasBoth, edgeHasMulti bool
		for _, p := range currentCMAO.Spec.InstallStrategy.Placements {
			if p.Name == "regional" {
				if slices.Contains(p.Configs, regionalAgentConfig) && slices.Contains(p.Configs, multiAgentConfig) {
					regionalHasBoth = true
				}
			}
			if p.Name == "edge" {
				if slices.Contains(p.Configs, multiAgentConfig) {
					edgeHasMulti = true
				}
			}
		}
		return regionalHasBoth && edgeHasMulti
	}, 10*time.Second, 100*time.Millisecond, "expected CMAO placement configs to include user-defined regional and multi-placement agents")

	// 4. Mutation 1: Remove "regional" placement from CMAO.
	//    Expected Garbage Collection cascade:
	//    - regionalAgent: only targeted "regional" -> must be deleted by DeleteOrphanResources.
	//    - multiAgent: targeted "regional" AND "edge", "edge" remains -> must be preserved.
	//    - unmanagedAgent: targeted "regional", but unmanaged -> must be preserved.
	//    - defaultAgent: targets "global" -> must be preserved.
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, cmaoKey, currentCMAO); err != nil {
			return false
		}
		filtered := make([]addonv1beta1.PlacementStrategy, 0, len(currentCMAO.Spec.InstallStrategy.Placements))
		for _, p := range currentCMAO.Spec.InstallStrategy.Placements {
			if p.Name != "regional" {
				filtered = append(filtered, p)
			}
		}
		currentCMAO.Spec.InstallStrategy.Placements = filtered
		return testEnv.K8sClient.Update(ctx, currentCMAO) == nil
	}, 10*time.Second, 100*time.Millisecond, "failed to update CMAO to remove regional placement")

	regionalKey := types.NamespacedName{Name: regionalAgentName, Namespace: addoncfg.InstallNamespace}
	multiKey := types.NamespacedName{Name: multiAgentName, Namespace: addoncfg.InstallNamespace}
	unmanagedKey := types.NamespacedName{Name: unmanagedAgentName, Namespace: addoncfg.InstallNamespace}

	// Assert orphaned regional agent is deleted
	require.Eventually(t, func() bool {
		err := testEnv.K8sClient.Get(ctx, regionalKey, &cooprometheusv1alpha1.PrometheusAgent{})
		return apierrors.IsNotFound(err)
	}, 10*time.Second, 100*time.Millisecond, "expected orphaned regional agent to be garbage collected")

	// Assert multiAgent is preserved because edge placement still exists
	require.NoError(t, testEnv.K8sClient.Get(ctx, multiKey, &cooprometheusv1alpha1.PrometheusAgent{}),
		"expected multi-placement agent to be preserved while edge placement remains")

	// Assert unmanagedAgent is untouched
	require.NoError(t, testEnv.K8sClient.Get(ctx, unmanagedKey, &cooprometheusv1alpha1.PrometheusAgent{}),
		"expected unmanaged agent to never be touched by MCOA garbage collection")

	// Assert default global agent is preserved
	require.NoError(t, testEnv.K8sClient.Get(ctx, defaultAgentKey, &cooprometheusv1alpha1.PrometheusAgent{}),
		"expected default global agent to be preserved")

	// Assert edge placement configs in CMAO retain multiAgent
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, cmaoKey, currentCMAO); err != nil {
			return false
		}
		for _, p := range currentCMAO.Spec.InstallStrategy.Placements {
			if p.Name == "edge" && slices.Contains(p.Configs, multiAgentConfig) {
				return true
			}
		}
		return false
	}, 10*time.Second, 100*time.Millisecond, "expected edge placement configs to retain multi-placement agent")

	// 5. Mutation 2: Remove "edge" placement from CMAO, leaving only "global".
	//    Expected Garbage Collection cascade:
	//    - multiAgent: now has no remaining active placements -> must be deleted by DeleteOrphanResources.
	//    - unmanagedAgent: still unmanaged -> must be preserved.
	//    - defaultAgent: targets "global" -> must be preserved.
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, cmaoKey, currentCMAO); err != nil {
			return false
		}
		filtered := make([]addonv1beta1.PlacementStrategy, 0, len(currentCMAO.Spec.InstallStrategy.Placements))
		for _, p := range currentCMAO.Spec.InstallStrategy.Placements {
			if p.Name != "edge" {
				filtered = append(filtered, p)
			}
		}
		currentCMAO.Spec.InstallStrategy.Placements = filtered
		return testEnv.K8sClient.Update(ctx, currentCMAO) == nil
	}, 10*time.Second, 100*time.Millisecond, "failed to update CMAO to remove edge placement")

	// Assert multiAgent is now garbage collected
	require.Eventually(t, func() bool {
		err := testEnv.K8sClient.Get(ctx, multiKey, &cooprometheusv1alpha1.PrometheusAgent{})
		return apierrors.IsNotFound(err)
	}, 10*time.Second, 100*time.Millisecond, "expected multi-placement agent to be garbage collected after edge placement removal")

	// Assert unmanagedAgent remains intact
	require.NoError(t, testEnv.K8sClient.Get(ctx, unmanagedKey, &cooprometheusv1alpha1.PrometheusAgent{}),
		"expected unmanaged agent to remain intact after all referenced placements removed")

	// Assert default global agent remains intact
	require.NoError(t, testEnv.K8sClient.Get(ctx, defaultAgentKey, &cooprometheusv1alpha1.PrometheusAgent{}),
		"expected default global agent to remain intact")

	// Assert CMAO retains only global placement
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, cmaoKey, currentCMAO); err != nil {
			return false
		}
		return len(currentCMAO.Spec.InstallStrategy.Placements) == 1 &&
			currentCMAO.Spec.InstallStrategy.Placements[0].Name == "global"
	}, 10*time.Second, 100*time.Millisecond, "expected CMAO to retain only global placement")

	// 6. Assert Quiescence: Ensure no resurrection loops or churn over a steady-state observation period
	require.Never(t, func() bool {
		recreatedRegional := testEnv.K8sClient.Get(ctx, regionalKey, &cooprometheusv1alpha1.PrometheusAgent{}) == nil
		recreatedMulti := testEnv.K8sClient.Get(ctx, multiKey, &cooprometheusv1alpha1.PrometheusAgent{}) == nil
		deletedUnmanaged := apierrors.IsNotFound(testEnv.K8sClient.Get(ctx, unmanagedKey, &cooprometheusv1alpha1.PrometheusAgent{}))
		deletedDefault := apierrors.IsNotFound(testEnv.K8sClient.Get(ctx, defaultAgentKey, &cooprometheusv1alpha1.PrometheusAgent{}))
		return recreatedRegional || recreatedMulti || deletedUnmanaged || deletedDefault
	}, 1*time.Second, 100*time.Millisecond, "expected system to achieve steady-state quiescence with no resurrection loops")
}
