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
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func setupTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, clusterv1beta1.Install(scheme))
	require.NoError(t, addonv1beta1.Install(scheme))
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
					Delegated:             true,
					NamespaceEnabled:      nsEnabled,
					VirtualizationEnabled: virtEnabled,
				},
			},
		},
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

func TestRSConfigMapPredicate(t *testing.T) {
	pred := RSConfigMapPredicate()

	rsNsCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}}
	rsVirtCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}}
	unrelatedCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "other-config", Namespace: addoncfg.InstallNamespace,
	}}
	wrongNsCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: rightsizing.NamespaceConfigMapName, Namespace: "other-namespace",
	}}

	// Create: accepts RS ConfigMaps, rejects others
	assert.True(t, pred.CreateFunc(event.CreateEvent{Object: rsNsCM}))
	assert.True(t, pred.CreateFunc(event.CreateEvent{Object: rsVirtCM}))
	assert.False(t, pred.CreateFunc(event.CreateEvent{Object: unrelatedCM}))
	assert.False(t, pred.CreateFunc(event.CreateEvent{Object: wrongNsCM}))

	// Update: accepts RS ConfigMaps, rejects others
	assert.True(t, pred.UpdateFunc(event.UpdateEvent{ObjectNew: rsNsCM}))
	assert.False(t, pred.UpdateFunc(event.UpdateEvent{ObjectNew: unrelatedCM}))

	// Delete: always rejected (prevents race during MCO cleanup)
	assert.False(t, pred.DeleteFunc(event.DeleteEvent{Object: rsNsCM}))
	assert.False(t, pred.DeleteFunc(event.DeleteEvent{Object: unrelatedCM}))

	// Generic: always rejected
	assert.False(t, pred.GenericFunc(event.GenericEvent{Object: rsNsCM}))
}

func TestReconcileRSResources_CleanupNamespace(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM, virtCM)
	ctx := context.TODO()

	opts := newPlatformOpts(false, true)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "namespace configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	require.NoError(t, err, "virtualization configmap should still exist")
}

func TestReconcileRSResources_CleanupVirtualization(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM, virtCM)
	ctx := context.TODO()

	opts := newPlatformOpts(true, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	require.NoError(t, err, "namespace configmap should still exist")
}

func TestReconcileRSResources_CleanupBoth(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM, virtCM)
	ctx := context.TODO()

	opts := newPlatformOpts(false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "namespace configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization configmap should be deleted")
}

func TestReconcileRSResources_CleanupIdempotent(t *testing.T) {
	ob := newTestOptionsBuilder(t)
	ctx := context.TODO()

	opts := newPlatformOpts(false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)
}

// TestReconcileRSResources_SkipsWhenNotDelegated verifies ConfigMaps survive when RS is not delegated to MCOA.
func TestReconcileRSResources_SkipsWhenNotDelegated(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM, virtCM)
	ctx := context.TODO()

	opts := addon.Options{
		Platform: addon.PlatformOptions{
			Enabled: true,
			AnalyticsOptions: addon.AnalyticsOptions{
				RightSizing: addon.RightSizingOptions{
					Delegated:             false,
					NamespaceEnabled:      false,
					VirtualizationEnabled: false,
				},
			},
		},
	}
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	require.NoError(t, err, "namespace configmap must survive when not delegated")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	require.NoError(t, err, "virtualization configmap must survive when not delegated")
}

func TestClusterMatchesPlacement_EmptyPredicates(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
	}
	placement := rightsizing.GetDefaultRSPlacement()
	assert.True(t, clusterMatchesPlacement(cluster, placement))
}

func TestClusterMatchesPlacement_LabelMatch(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cluster1",
			Labels: map[string]string{"env": "prod", "region": "us-east"},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{{
				RequiredClusterSelector: clusterv1beta1.ClusterSelector{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "prod"},
					},
				},
			}},
		},
	}
	assert.True(t, clusterMatchesPlacement(cluster, placement))
}

func TestClusterMatchesPlacement_LabelNoMatch(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cluster1",
			Labels: map[string]string{"env": "staging"},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{{
				RequiredClusterSelector: clusterv1beta1.ClusterSelector{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "prod"},
					},
				},
			}},
		},
	}
	assert.False(t, clusterMatchesPlacement(cluster, placement))
}

func TestClusterMatchesPlacement_LabelExpressions(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cluster1",
			Labels: map[string]string{"env": "prod"},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{{
				RequiredClusterSelector: clusterv1beta1.ClusterSelector{
					LabelSelector: metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "env",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"prod", "staging"},
						}},
					},
				},
			}},
		},
	}
	assert.True(t, clusterMatchesPlacement(cluster, placement))
}

func TestClusterMatchesPlacement_ClaimMatch(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: "platform.open-cluster-management.io", Value: "AWS"},
			},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{{
				RequiredClusterSelector: clusterv1beta1.ClusterSelector{
					ClaimSelector: clusterv1beta1.ClusterClaimSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "platform.open-cluster-management.io",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"AWS", "GCP"},
						}},
					},
				},
			}},
		},
	}
	assert.True(t, clusterMatchesPlacement(cluster, placement))
}

func TestClusterMatchesPlacement_ClaimNoMatch(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: "platform.open-cluster-management.io", Value: "Azure"},
			},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{{
				RequiredClusterSelector: clusterv1beta1.ClusterSelector{
					ClaimSelector: clusterv1beta1.ClusterClaimSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "platform.open-cluster-management.io",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"AWS", "GCP"},
						}},
					},
				},
			}},
		},
	}
	assert.False(t, clusterMatchesPlacement(cluster, placement))
}

func TestClusterMatchesPlacement_PredicatesORed(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cluster1",
			Labels: map[string]string{"region": "eu-west"},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{
				{
					RequiredClusterSelector: clusterv1beta1.ClusterSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"region": "us-east"},
						},
					},
				},
				{
					RequiredClusterSelector: clusterv1beta1.ClusterSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{"region": "eu-west"},
						},
					},
				},
			},
		},
	}
	assert.True(t, clusterMatchesPlacement(cluster, placement), "second predicate should match (ORed)")
}

func TestClusterMatchesPlacement_ClaimDoesNotExist(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: "platform.open-cluster-management.io", Value: "AWS"},
			},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{{
				RequiredClusterSelector: clusterv1beta1.ClusterSelector{
					ClaimSelector: clusterv1beta1.ClusterClaimSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "customclaim",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						}},
					},
				},
			}},
		},
	}
	assert.True(t, clusterMatchesPlacement(cluster, placement))
}

func TestClusterMatchesPlacement_CombinedLabelAndClaim(t *testing.T) {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cluster1",
			Labels: map[string]string{"env": "prod"},
		},
		Status: clusterv1.ManagedClusterStatus{
			ClusterClaims: []clusterv1.ManagedClusterClaim{
				{Name: "platform.open-cluster-management.io", Value: "AWS"},
			},
		},
	}

	placement := clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{{
				RequiredClusterSelector: clusterv1beta1.ClusterSelector{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "prod"},
					},
					ClaimSelector: clusterv1beta1.ClusterClaimSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "platform.open-cluster-management.io",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"AWS"},
						}},
					},
				},
			}},
		},
	}
	assert.True(t, clusterMatchesPlacement(cluster, placement), "both label and claim match (ANDed)")

	cluster.Labels["env"] = "staging"
	assert.False(t, clusterMatchesPlacement(cluster, placement), "label no longer matches")
}

func TestRevertConfigMap_WritesBackValidConfig(t *testing.T) {
	// Simulate a ConfigMap with invalid memoryAggregator "M90".
	// After validation + revert, the ConfigMap must reflect the cached (valid) values.
	invalidConfig := `{"namespaceFilterCriteria":{"exclusionCriteria":["openshift.*"]},"recommendationPercentage":110,"cpuAggregator":["Max OverAll","P99","P95"],"memoryAggregator":["Max OverAll","P99","M90"]}`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": invalidConfig,
		},
	}
	ob := newTestOptionsBuilder(t, cm)
	ctx := t.Context()

	// Seed the cache with a previous valid config
	cacheValidAggregator(rightsizing.NamespaceConfigMapName, "cpu", []string{"Max OverAll", "P99", "P95"})
	cacheValidAggregator(rightsizing.NamespaceConfigMapName, "memory", []string{"Max OverAll", "P99", "P95"})

	// Parse, validate, and revert
	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
	assert.True(t, reverted, "should signal revert needed when invalid values present")

	// The in-memory config should have the cached valid values, not the invalid ones
	assert.Equal(t, []string{"Max OverAll", "P99", "P95"}, configData.PrometheusRuleConfig.MemoryAggregator)
	assert.Equal(t, []string{"Max OverAll", "P99", "P95"}, configData.PrometheusRuleConfig.CpuAggregator)

	// Now write-back
	ob.revertConfigMap(ctx, rightsizing.NamespaceConfigMapName, configData)

	// Verify the ConfigMap was updated
	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.NotEqual(t, invalidConfig, updated.Data["prometheusRuleConfig"],
		"ConfigMap should have been reverted (invalid M90 removed)")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"memoryAggregator":["Max OverAll","P99","P95"]`)
	assert.NotContains(t, updated.Data["prometheusRuleConfig"], "M90",
		"invalid value M90 should not be in reverted ConfigMap")
}

func TestRevertConfigMap_NoRevertWhenAllValid(t *testing.T) {
	validConfig := `{"namespaceFilterCriteria":{"exclusionCriteria":["openshift.*"]},"recommendationPercentage":110,"cpuAggregator":["Max OverAll","P99","P95"],"memoryAggregator":["Max OverAll","P99"]}`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": validConfig,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
	assert.False(t, reverted, "should not signal revert when all values are valid")
}

func TestRevertConfigMap_FallsBackToDefaultsWhenNoCachedConfig(t *testing.T) {
	// Clear cache to simulate process restart
	lastValidMu.Lock()
	delete(lastValidAggregators, cacheKey(rightsizing.VirtualizationConfigMapName, "cpu"))
	delete(lastValidAggregators, cacheKey(rightsizing.VirtualizationConfigMapName, "memory"))
	lastValidMu.Unlock()

	invalidConfig := `{"recommendationPercentage":110,"cpuAggregator":["foo"],"memoryAggregator":["bar"]}`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.VirtualizationConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": invalidConfig,
		},
	}
	ob := newTestOptionsBuilder(t, cm)
	ctx := t.Context()

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.VirtualizationConfigMapName)
	assert.True(t, reverted, "should signal revert when invalid values present")

	// With no cache, aggregators are set to nil (downstream resolvers use defaults)
	assert.Nil(t, configData.PrometheusRuleConfig.CpuAggregator)
	assert.Nil(t, configData.PrometheusRuleConfig.MemoryAggregator)

	// Write-back should store nil aggregators (defaults will be used)
	ob.revertConfigMap(ctx, rightsizing.VirtualizationConfigMapName, configData)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.NotContains(t, updated.Data["prometheusRuleConfig"], "foo")
	assert.NotContains(t, updated.Data["prometheusRuleConfig"], "bar")
}

func TestRevertConfigMap_InvalidCpuDoesNotAffectValidMemory(t *testing.T) {
	// Pre-cache valid CPU
	cacheValidAggregator("test-mixed-cm", "cpu", []string{"P90", "P80"})

	invalidConfig := `{"recommendationPercentage":110,"cpuAggregator":["P90","M80"],"memoryAggregator":["Max OverAll","P99"]}`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mixed-cm",
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": invalidConfig,
		},
	}
	ob := newTestOptionsBuilder(t, cm)
	ctx := t.Context()

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, "test-mixed-cm")
	assert.True(t, reverted, "CPU has invalid entry so revert is needed")

	// CPU should be reverted to cached; memory should keep the valid new values
	assert.Equal(t, []string{"P90", "P80"}, configData.PrometheusRuleConfig.CpuAggregator)
	assert.Equal(t, []string{"Max OverAll", "P99"}, configData.PrometheusRuleConfig.MemoryAggregator)

	ob.revertConfigMap(ctx, "test-mixed-cm", configData)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(ctx, types.NamespacedName{
		Name: "test-mixed-cm", Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.NotContains(t, updated.Data["prometheusRuleConfig"], "M80")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"P90"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"P80"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"Max OverAll"`)
}

func TestBackfillAggregatorKeys_AddsMissingKeys(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"namespaceFilterCriteria":{"exclusionCriteria":["openshift.*"]},"recommendationPercentage":110}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	err := ob.backfillAggregatorKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"cpuAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"memoryAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"recommendationPercentage":110`)
}

func TestBackfillAggregatorKeys_SkipsWhenKeysExist(t *testing.T) {
	original := `{"recommendationPercentage":110,"cpuAggregator":["P90"],"memoryAggregator":["P99"]}`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{
			"prometheusRuleConfig": original,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	err := ob.backfillAggregatorKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Equal(t, original, updated.Data["prometheusRuleConfig"],
		"should not modify ConfigMap when both keys already exist")
}

func TestBackfillAggregatorKeys_PreservesExistingFields(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"recommendationPercentage":120,"namespaceFilterCriteria":{"exclusionCriteria":["kube-.*"]}}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	err := ob.backfillAggregatorKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"recommendationPercentage":120`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `kube-.*`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"cpuAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"memoryAggregator"`)
}

func TestBackfillAggregatorKeys_OnlyOneMissing(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"recommendationPercentage":110,"cpuAggregator":["P90","P75"]}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	err := ob.backfillAggregatorKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"memoryAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"P90"`,
		"existing cpuAggregator values should be preserved")
}

func TestBackfillAggregatorKeys_YAMLFromMCO217(t *testing.T) {
	// MCO 2.17 writes prometheusRuleConfig as YAML via yaml.Marshal.
	// On upgrade the ConfigMap is preserved; backfill must parse YAML so
	// aggregator keys become visible (json.Unmarshal would silently no-op).
	yamlConfig := `namespaceFilterCriteria:
  inclusionCriteria: []
  exclusionCriteria:
  - openshift.*
labelFilterCriteria: []
recommendationPercentage: 110
`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{
			"prometheusRuleConfig": yamlConfig,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	err := ob.backfillAggregatorKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"cpuAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"memoryAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"recommendationPercentage":110`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `openshift.*`,
		"existing YAML fields should be preserved after JSON rewrite")
}

func TestBackfillAggregatorKeys_YAMLPreservesExistingAggregators(t *testing.T) {
	yamlConfig := `namespaceFilterCriteria:
  exclusionCriteria:
  - openshift.*
recommendationPercentage: 110
cpuAggregator:
- P90
memoryAggregator:
- Max OverAll
- P99
`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{
			"prometheusRuleConfig": yamlConfig,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	err := ob.backfillAggregatorKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.YAMLEq(t, yamlConfig, updated.Data["prometheusRuleConfig"],
		"should not modify YAML ConfigMap when both aggregator keys already exist")
}
