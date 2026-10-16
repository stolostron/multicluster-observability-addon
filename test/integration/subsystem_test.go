package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	ocinfrav1 "github.com/openshift/api/config/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	addon "github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	addonctrl "github.com/stolostron/multicluster-observability-addon/internal/controllers/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/resourcecreator"
	"github.com/stolostron/multicluster-observability-addon/internal/controllers/watcher"
	metricsconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type subsystemTrigger struct {
	ClusterName string
	AddonName   string
}

// triggerSpyAddonManager wraps the real OCM AddonManager, recording every Trigger invocation
// before delegating directly to the underlying real AddonManager to drive actual reconciliations.
type triggerSpyAddonManager struct {
	addonmanager.AddonManager
	mu       sync.Mutex
	triggers []subsystemTrigger
}

func (s *triggerSpyAddonManager) Trigger(clusterName, addonName string) {
	s.mu.Lock()
	s.triggers = append(s.triggers, subsystemTrigger{
		ClusterName: clusterName,
		AddonName:   addonName,
	})
	s.mu.Unlock()
	s.AddonManager.Trigger(clusterName, addonName)
}

func (s *triggerSpyAddonManager) getTriggers() []subsystemTrigger {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.triggers)
}

func (s *triggerSpyAddonManager) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.triggers)
}

func (s *triggerSpyAddonManager) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggers = nil
}

// startSubsystemManager initializes and starts:
//  1. The real OCM AddonManager (via addonctrl.NewAddonManager) wrapped with a trigger spy.
//  2. A shared controller-runtime Manager hosting ResourceCreatorReconciler and WatcherReconciler.
func startSubsystemManager(ctx context.Context, t *testing.T, testEnv *TestEnv) (ctrl.Manager, *triggerSpyAddonManager) {
	t.Helper()

	testLogger := testr.New(t)
	ctrl.SetLogger(testLogger)

	httpClient, err := rest.HTTPClientFor(testEnv.Cfg)
	require.NoError(t, err)

	mapper, err := apiutil.NewDynamicRESTMapper(testEnv.Cfg, httpClient)
	require.NoError(t, err)

	realAddonMgr, err := addonctrl.NewAddonManager(ctx, testEnv.Cfg, testEnv.Scheme, testLogger, httpClient, mapper)
	require.NoError(t, err)

	addonSpy := &triggerSpyAddonManager{AddonManager: realAddonMgr}

	disableNameValidation := true
	mgr, err := ctrl.NewManager(testEnv.Cfg, ctrl.Options{
		Scheme: testEnv.Scheme,
		Cache:  addoncfg.GetCacheOptions(),
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
	require.NoError(t, watcher.SetupWithManager(mgr, addonSpy, testLogger))

	mgrCtx, cancelMgr := context.WithCancel(ctx)
	mgrErrCh := make(chan error, 1)
	t.Cleanup(func() {
		cancelMgr()
		select {
		case mgrErr := <-mgrErrCh:
			assert.NoError(t, mgrErr, "controller manager exited with unexpected error")
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for controller manager to shutdown")
		}
	})

	go func() {
		mgrErrCh <- mgr.Start(mgrCtx)
	}()

	require.NoError(t, realAddonMgr.Start(mgrCtx))

	require.True(t, mgr.GetCache().WaitForCacheSync(ctx), "failed waiting for manager caches to sync")
	return mgr, addonSpy
}

// extractSecretFromManifestWork searches ManifestWork.Spec.Workload.Manifests for a Secret with the given name.
func extractSecretFromManifestWork(mw *workv1.ManifestWork, secretName string) (*corev1.Secret, bool) {
	for _, manifest := range mw.Spec.Workload.Manifests {
		var secret corev1.Secret
		if err := json.Unmarshal(manifest.Raw, &secret); err == nil {
			if secret.Kind == "Secret" && secret.Name == secretName {
				return &secret, true
			}
		}
	}
	return nil, false
}

// extractManifestFromManifestWork searches ManifestWork.Spec.Workload.Manifests for a resource with the given kind and name.
func extractManifestFromManifestWork(mw *workv1.ManifestWork, kind, name string) ([]byte, bool) {
	for _, manifest := range mw.Spec.Workload.Manifests {
		var meta struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(manifest.Raw, &meta); err == nil {
			if meta.Kind == kind && meta.Metadata.Name == name {
				return manifest.Raw, true
			}
		}
	}
	return nil, false
}

// TestSubsystem_PlatformMetricsDeploymentAndSecretRotation serves as the Tier 2 Subsystem integration test.
//
// Architectural Scope:
// This test validates the full end-to-end multi-controller integration loop across MCOA:
//  1. Real concurrent execution of ResourceCreator, real AddonManager, and Watcher.
//  2. Closed-Loop Reactive Lifecycle:
//     a. AODC + CMAO are applied -> ResourceCreator reconciles and produces default PrometheusAgent on the Hub.
//     b. ManagedClusterAddOn config references point to the generated PrometheusAgent.
//     c. The real AddonManager reconciles ManagedClusterAddOn, renders the Helm chart, and creates the real
//     ManifestWork on the spoke cluster namespace.
//     d. The real Watcher inspects the generated ManifestWork and dynamically caches reverse-dependencies
//     for all embedded upstream Secrets.
//     e. Mutating an upstream Secret on the hub fires a Watcher event -> Watcher triggers the real AddonManager
//     -> the real AddonManager re-renders Helm manifests -> the real ManifestWork is updated on the API server.
//  3. Steady-State Quiescence Assertion:
//     Proves absence of runaway event storms, ping-pong reconciliations, or resource version drift across
//     the entire subsystem once steady-state converges.
func TestSubsystem_PlatformMetricsDeploymentAndSecretRotation(t *testing.T) {
	testEnv := SetupTestEnv(t)
	ctx := t.Context()

	// 1. Setup Namespaces
	setupTestNamespace(ctx, t, testEnv.K8sClient, addoncfg.InstallNamespace)

	spokeCluster := "managed-cluster-prod-1"
	setupTestNamespace(ctx, t, testEnv.K8sClient, spokeCluster)

	// 2. Setup Prerequisites required for real AddonManager manifest rendering:
	// - OpenShift ClusterVersion CR named "version" (required by getClusterID)
	clusterVersion := &ocinfrav1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
		Spec: ocinfrav1.ClusterVersionSpec{
			ClusterID: "c0ffeebabe-1234-5678-9abc-def012345678",
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, clusterVersion))

	// - Upstream mTLS and Accessor Secrets in addon install namespace
	hubCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metricsconfig.HubCASecretName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string][]byte{
			metricsconfig.MTLSCASecretKey: []byte("initial-ca-cert-payload"),
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, hubCASecret))

	clientCertSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metricsconfig.ClientCertSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string][]byte{
			metricsconfig.MTLSCertSecretKey:    []byte("client-cert-payload"),
			metricsconfig.MTLSCertKeySecretKey: []byte("client-key-payload"),
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, clientCertSecret))

	accessorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metricsconfig.AlertmanagerAccessorSecretName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string][]byte{
			"token": []byte("initial-token"),
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, accessorSecret))

	// - Images list ConfigMap for metrics agent image overrides
	imagesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metricsconfig.ImagesConfigMapObjKey.Name,
			Namespace: metricsconfig.ImagesConfigMapObjKey.Namespace,
		},
		Data: map[string]string{
			"obo_prometheus_rhel9_operator": "quay.io/prometheus/obo-operator",
			"prometheus_config_reloader":    "quay.io/prometheus/config-reloader",
			"kube_rbac_proxy":               "quay.io/kube/rbac-proxy",
			"kube_state_metrics":            "quay.io/kube/kube-state-metrics",
			"node_exporter":                 "quay.io/kube/node-exporter",
			"prometheus":                    "quay.io/prometheus/prometheus",
			"endpoint_monitoring_operator":  "quay.io/stolostron/endpoint-monitoring-operator",
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, imagesCM))

	// - ManagedCluster representing spokeCluster
	managedCluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: spokeCluster,
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, managedCluster))

	// - AddOnDeploymentConfig and ClusterManagementAddOn for ResourceCreator and AddonManager
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
					Value: "thanos-receiver.apps.hub.example.com",
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, aodc))

	aodcHash, err := addonutils.GetAddOnDeploymentConfigSpecHash(aodc)
	require.NoError(t, err)

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

	// - ManagedClusterAddOn for spokeCluster
	mcAddon := &addonv1beta1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: spokeCluster,
		},
		Spec: addonv1beta1.ManagedClusterAddOnSpec{},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, mcAddon))

	// Pre-seed ManagedClusterAddOn Status with RegistrationApplied condition
	// (satisfies OCM addon-deploy-controller gatekeeper) and config references
	mcAddon.Status.Conditions = []metav1.Condition{
		{
			Type:               addonv1beta1.ManagedClusterAddOnRegistrationApplied,
			Status:             metav1.ConditionTrue,
			Reason:             "RegistrationApplied",
			Message:            "Registration applied for testing",
			LastTransitionTime: metav1.Now(),
		},
	}
	mcAddon.Status.ConfigReferences = []addonv1beta1.ConfigReference{
		{
			ConfigGroupResource: addonv1beta1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			DesiredConfig: &addonv1beta1.ConfigSpecHash{
				ConfigReferent: addonv1beta1.ConfigReferent{
					Namespace: addoncfg.InstallNamespace,
					Name:      addoncfg.Name,
				},
				SpecHash: aodcHash,
			},
		},
		{
			ConfigGroupResource: addonv1beta1.ConfigGroupResource{
				Group:    cooprometheusv1alpha1.SchemeGroupVersion.Group,
				Resource: cooprometheusv1alpha1.PrometheusAgentName,
			},
			DesiredConfig: &addonv1beta1.ConfigSpecHash{
				ConfigReferent: addonv1beta1.ConfigReferent{
					Namespace: addoncfg.InstallNamespace,
					Name:      defaultAgentName,
				},
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Status().Update(ctx, mcAddon))

	// 3. Start Manager with ResourceCreator, real AddonManager, and Watcher
	_, addonSpy := startSubsystemManager(ctx, t, testEnv)

	// 4. Verify convergence: ResourceCreator provisions PrometheusAgent via SSA,
	// OCM addon-config-controller detects the new config and triggers AddonManager,
	// which generates the real ManifestWork on the spoke cluster namespace.
	mwKey := types.NamespacedName{
		Name:      fmt.Sprintf("addon-%s-deploy-0", addoncfg.Name),
		Namespace: spokeCluster,
	}
	var mw workv1.ManifestWork
	var agentRaw []byte
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, mwKey, &mw); err != nil {
			return false
		}
		var foundAgent bool
		agentRaw, foundAgent = extractManifestFromManifestWork(&mw, cooprometheusv1alpha1.PrometheusAgentsKind, metricsconfig.PlatformMetricsCollectorApp)
		return foundAgent
	}, 30*time.Second, 100*time.Millisecond, "expected real ManifestWork to be generated by real AddonManager containing PrometheusAgent")

	trimmedHubID := metricsconfig.GetTrimmedClusterID(string(clusterVersion.Spec.ClusterID))
	targetHubCAName := metricsconfig.GetHubMtlsCASecretName(trimmedHubID)

	var deployedAgent cooprometheusv1alpha1.PrometheusAgent
	require.NoError(t, json.Unmarshal(agentRaw, &deployedAgent))
	assert.Contains(t, deployedAgent.Spec.Secrets, targetHubCAName, "expected PrometheusAgent to reference rewritten Hub CA Secret")

	var foundRW bool
	for _, rw := range deployedAgent.Spec.RemoteWrite {
		if rw.Name != nil && *rw.Name == metricsconfig.RemoteWriteCfgName {
			foundRW = true
			require.NotNil(t, rw.TLSConfig, "expected remote write spec to have TLSConfig")
			assert.Equal(t, fmt.Sprintf("/etc/prometheus/secrets/%s/%s", targetHubCAName, metricsconfig.MTLSCASecretKey), rw.TLSConfig.CAFile)
			break
		}
	}
	assert.True(t, foundRW, "expected PrometheusAgent to contain %s remote write spec", metricsconfig.RemoteWriteCfgName)

	// Verify the real ManifestWork contains the embedded Hub CA Secret with original resource annotation
	embeddedCA, found := extractSecretFromManifestWork(&mw, targetHubCAName)
	require.True(t, found, "expected ManifestWork to contain embedded Hub CA Secret %s", targetHubCAName)
	assert.Equal(t, fmt.Sprintf("%s/%s", addoncfg.InstallNamespace, metricsconfig.HubCASecretName),
		embeddedCA.Annotations[addoncfg.AnnotationOriginalResource], "expected correct original-resource annotation")
	assert.Equal(t, []byte("initial-ca-cert-payload"), embeddedCA.Data[metricsconfig.MTLSCASecretKey])

	// 5. Reset spy trigger counter before mutating Secret
	addonSpy.reset()

	// 6. Mutate upstream Hub CA Secret
	var curSecret corev1.Secret
	require.NoError(t, testEnv.K8sClient.Get(ctx, client.ObjectKeyFromObject(hubCASecret), &curSecret))
	curSecret.Data[metricsconfig.MTLSCASecretKey] = []byte("updated-ca-cert-payload")
	require.NoError(t, testEnv.K8sClient.Update(ctx, &curSecret))

	// 7. Closed-Loop Assertion:
	// Verify Watcher detects the Secret update, triggers real AddonManager,
	// and real AddonManager updates the ManifestWork with the new CA payload.
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, mwKey, &mw); err != nil {
			return false
		}
		updatedSecret, ok := extractSecretFromManifestWork(&mw, targetHubCAName)
		if !ok {
			return false
		}
		return string(updatedSecret.Data[metricsconfig.MTLSCASecretKey]) == "updated-ca-cert-payload"
	}, 30*time.Second, 100*time.Millisecond, "expected ManifestWork to be updated with new secret data following trigger")

	// Verify that Watcher triggered reconciliation for the spokeCluster
	hasTrigger := slices.ContainsFunc(addonSpy.getTriggers(), func(tr subsystemTrigger) bool {
		return tr.ClusterName == spokeCluster && tr.AddonName == addoncfg.Name
	})
	assert.True(t, hasTrigger, "expected Watcher trigger for spokeCluster and addonName")

	// 8. Deterministic Quiescence Assertion:
	// Observe steady state after convergence to verify no runaway trigger loop occurs.
	settled := addonSpy.count()
	assert.GreaterOrEqual(t, settled, 1, "expected at least 1 trigger for the secret mutation")
	assert.Never(t, func() bool {
		return addonSpy.count() > settled
	}, 3*time.Second, 100*time.Millisecond, "expected no further triggers after convergence (runaway reconciliation)")

	// 9. User-Defined PrometheusAgent Spoke Rendering:
	// Create a user-defined PrometheusAgent (with part-of label and placement-ref annotation, but no CMAO OwnerReference).
	// Verify that ResourceCreator deletes the shadowed default agent, reconciles the user-defined agent via SSA
	// without adding a CMAO OwnerReference, and that the real AddonManager renders it into the spoke ManifestWork.
	userAgentName := "user-custom-platform-agent"
	userAgent := &cooprometheusv1alpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userAgentName,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				addoncfg.ComponentK8sLabelKey: metricsconfig.PlatformMetricsCollectorApp,
				addoncfg.PartOfK8sLabelKey:    addoncfg.Name,
			},
			Annotations: map[string]string{
				addoncfg.PlacementAnnotationKey: fmt.Sprintf("%s/%s", addoncfg.InstallNamespace, "global"),
			},
		},
		Spec: cooprometheusv1alpha1.PrometheusAgentSpec{
			CommonPrometheusFields: cooprometheusv1.CommonPrometheusFields{
				ScrapeInterval: "45s",
			},
		},
	}
	require.NoError(t, testEnv.K8sClient.Create(ctx, userAgent))

	// Wait for ResourceCreator to delete the shadowed default agent and reconcile userAgent via SSA
	var reconciledUserAgent cooprometheusv1alpha1.PrometheusAgent
	require.Eventually(t, func() bool {
		var defAgent cooprometheusv1alpha1.PrometheusAgent
		err := testEnv.K8sClient.Get(ctx, types.NamespacedName{
			Name:      defaultAgentName,
			Namespace: addoncfg.InstallNamespace,
		}, &defAgent)
		if !apierrors.IsNotFound(err) {
			return false
		}
		if err := testEnv.K8sClient.Get(ctx, client.ObjectKeyFromObject(userAgent), &reconciledUserAgent); err != nil {
			return false
		}
		return len(reconciledUserAgent.Spec.RemoteWrite) > 0
	}, 10*time.Second, 100*time.Millisecond, "expected shadowed default agent to be deleted and userAgent reconciled via SSA")
	assert.Empty(t, reconciledUserAgent.OwnerReferences, "expected user-defined PrometheusAgent to not acquire a CMAO OwnerReference")

	// Update ManagedClusterAddOn.Status.ConfigReferences to point to the user-defined PrometheusAgent
	var curMCAddon addonv1beta1.ManagedClusterAddOn
	require.NoError(t, testEnv.K8sClient.Get(ctx, client.ObjectKeyFromObject(mcAddon), &curMCAddon))
	for i := range curMCAddon.Status.ConfigReferences {
		if curMCAddon.Status.ConfigReferences[i].Resource == cooprometheusv1alpha1.PrometheusAgentName {
			curMCAddon.Status.ConfigReferences[i].DesiredConfig.Name = userAgentName
		}
	}
	require.NoError(t, testEnv.K8sClient.Status().Update(ctx, &curMCAddon))

	// Verify real AddonManager renders the user-defined PrometheusAgent into the spoke ManifestWork
	require.Eventually(t, func() bool {
		if err := testEnv.K8sClient.Get(ctx, mwKey, &mw); err != nil {
			return false
		}
		raw, found := extractManifestFromManifestWork(&mw, cooprometheusv1alpha1.PrometheusAgentsKind, metricsconfig.PlatformMetricsCollectorApp)
		if !found {
			return false
		}
		var agent cooprometheusv1alpha1.PrometheusAgent
		if err := json.Unmarshal(raw, &agent); err != nil {
			return false
		}
		return agent.Spec.ScrapeInterval == "45s" && slices.Contains(agent.Spec.Secrets, targetHubCAName)
	}, 15*time.Second, 100*time.Millisecond, "expected real AddonManager to render user-defined PrometheusAgent (without CMAO OwnerReference) into spoke ManifestWork")
}
