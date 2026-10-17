package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	clusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	rsnamespace "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/namespace"
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

// newLoggingOptionsBuilder is newTestOptionsBuilder with a logger that appends every
// Info-level line to the returned slice, rendered by funcr as `"key"="value"` pairs.
func newLoggingOptionsBuilder(t *testing.T, objs ...runtime.Object) (*OptionsBuilder, *[]string) {
	t.Helper()
	lines := &[]string{}
	ob := newTestOptionsBuilder(t, objs...)
	ob.Logger = funcr.New(func(_, args string) {
		*lines = append(*lines, args)
	}, funcr.Options{})
	return ob, lines
}

// resetIntervalState empties the package-level interval caches before and after a test,
// so it starts like a freshly started operator and leaves nothing behind for the others.
func resetIntervalState(t *testing.T) {
	t.Helper()
	reset := func() {
		lastValidMu.Lock()
		defer lastValidMu.Unlock()
		clear(lastValidIntervals)
		clear(lastOrderingState)
	}
	reset()
	t.Cleanup(reset)
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
	assert.Contains(t, updated.Data["prometheusRuleConfig"], "- Max OverAll")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], "- P99")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], "- P95")
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

	// With no cache, aggregators fall back to defaults
	assert.Equal(t, rightsizing.DefaultCpuAggregator, configData.PrometheusRuleConfig.CpuAggregator)
	assert.Equal(t, rightsizing.DefaultMemoryAggregator, configData.PrometheusRuleConfig.MemoryAggregator)

	// Write-back should store default aggregators
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
	assert.Contains(t, updated.Data["prometheusRuleConfig"], "- P90")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], "- P80")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], "- Max OverAll")
}

func TestBackfillConfigKeys_AddsMissingKeys(t *testing.T) {
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

	err := ob.backfillConfigKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"cpuAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"memoryAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"recommendationPercentage":110`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"],
		`"evaluationInterval":"`+rightsizing.DefaultEvaluationInterval+`"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"],
		`"evaluationInterval1d":"`+rightsizing.DefaultEvaluationInterval1d+`"`)
}

func TestBackfillConfigKeys_SkipsWhenKeysExist(t *testing.T) {
	original := `{"recommendationPercentage":110,"cpuAggregator":["P90"],"memoryAggregator":["P99"],"evaluationInterval":"5m","evaluationInterval1d":"15m"}`
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

	err := ob.backfillConfigKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Equal(t, original, updated.Data["prometheusRuleConfig"],
		"should not modify ConfigMap when all keys already exist")
}

func TestBackfillConfigKeys_PreservesExistingFields(t *testing.T) {
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

	err := ob.backfillConfigKeys(t.Context(), cm)
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

func TestBackfillConfigKeys_OnlyOneMissing(t *testing.T) {
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

	err := ob.backfillConfigKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"memoryAggregator"`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `"P90"`,
		"existing cpuAggregator values should be preserved")
}

// TestBackfillConfigKeys_OnlyOneIntervalMissing covers a ConfigMap that already carries
// one of the two interval keys: the user's value must survive, and only the other key is
// added, with its default.
func TestBackfillConfigKeys_OnlyOneIntervalMissing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		present string // the interval the user already set, as a JSON member
		added   string // the JSON member backfill has to add
	}{
		{
			name:    "evaluationInterval1d is missing",
			present: `"evaluationInterval":"10m"`,
			added:   `"evaluationInterval1d":"` + rightsizing.DefaultEvaluationInterval1d + `"`,
		},
		{
			name:    "evaluationInterval is missing",
			present: `"evaluationInterval1d":"1h"`,
			added:   `"evaluationInterval":"` + rightsizing.DefaultEvaluationInterval + `"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rightsizing.NamespaceConfigMapName,
					Namespace: addoncfg.InstallNamespace,
				},
				Data: map[string]string{
					"prometheusRuleConfig": `{"recommendationPercentage":110,"cpuAggregator":["P90"],"memoryAggregator":["P99"],` + tc.present + `}`,
				},
			}
			ob := newTestOptionsBuilder(t, cm)

			err := ob.backfillConfigKeys(t.Context(), cm)
			require.NoError(t, err)

			var updated corev1.ConfigMap
			require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
				Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
			}, &updated))

			assert.Contains(t, updated.Data["prometheusRuleConfig"], tc.present,
				"the existing interval value should be preserved")
			assert.Contains(t, updated.Data["prometheusRuleConfig"], tc.added)
		})
	}
}

func TestBackfillConfigKeys_YAMLFromMCO217(t *testing.T) {
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

	err := ob.backfillConfigKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.Contains(t, updated.Data["prometheusRuleConfig"], `cpuAggregator:`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `memoryAggregator:`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"],
		"evaluationInterval: "+rightsizing.DefaultEvaluationInterval+"\n")
	assert.Contains(t, updated.Data["prometheusRuleConfig"],
		"evaluationInterval1d: "+rightsizing.DefaultEvaluationInterval1d+"\n")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `recommendationPercentage: 110`)
	assert.Contains(t, updated.Data["prometheusRuleConfig"], `openshift.*`,
		"existing YAML fields should be preserved after YAML rewrite")
}

func TestBackfillConfigKeys_YAMLPreservesExistingAggregators(t *testing.T) {
	yamlConfig := `namespaceFilterCriteria:
  exclusionCriteria:
  - openshift.*
recommendationPercentage: 110
cpuAggregator:
- P90
memoryAggregator:
- Max OverAll
- P99
evaluationInterval: 5m
evaluationInterval1d: 15m
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

	err := ob.backfillConfigKeys(t.Context(), cm)
	require.NoError(t, err)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))

	assert.YAMLEq(t, yamlConfig, updated.Data["prometheusRuleConfig"],
		"should not modify YAML ConfigMap when all keys already exist")
}

func TestValidateAndSanitizeConfig_InvalidEvaluationIntervalFallsBackToDefault(t *testing.T) {
	// Clear interval cache to simulate process restart (no prior valid value).
	lastValidMu.Lock()
	delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval"))
	lastValidMu.Unlock()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"cpuAggregator":["Max OverAll"],"memoryAggregator":["Max OverAll"],"evaluationInterval":"badvalue","evaluationInterval1d":"15m"}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)
	ctx := t.Context()

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
	assert.True(t, reverted, "invalid evaluationInterval should trigger revert")
	assert.Equal(t, rightsizing.DefaultEvaluationInterval, configData.PrometheusRuleConfig.EvaluationInterval,
		"should fall back to default when no cached value exists")

	// Write-back should remove the bad value.
	ob.revertConfigMap(ctx, rightsizing.NamespaceConfigMapName, configData)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))
	assert.NotContains(t, updated.Data["prometheusRuleConfig"], "badvalue")
	assert.Contains(t, updated.Data["prometheusRuleConfig"], rightsizing.DefaultEvaluationInterval)
}

func TestValidateAndSanitizeConfig_InvalidEvaluationIntervalRestoresCachedValue(t *testing.T) {
	// Pre-seed the cache with a previously valid custom interval.
	cacheValidInterval(rightsizing.NamespaceConfigMapName, "evaluationInterval", "10m")

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"cpuAggregator":["Max OverAll"],"memoryAggregator":["Max OverAll"],"evaluationInterval":"0m","evaluationInterval1d":"15m"}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
	assert.True(t, reverted, "zero-value duration should trigger revert")
	assert.Equal(t, "10m", configData.PrometheusRuleConfig.EvaluationInterval,
		"should restore previously cached valid value, not hardcoded default")
}

func TestValidateAndSanitizeConfig_ZeroValueDurationIsInvalid(t *testing.T) {
	// Clear interval cache so this test always exercises the default-fallback path
	// regardless of which other tests ran first. Sibling tests seed "10m" into the
	// cache using the same ConfigMap name, which would otherwise cause this test to
	// restore "10m" instead of the hardcoded default — passing either way but testing
	// the wrong path.
	lastValidMu.Lock()
	delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval"))
	delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval1d"))
	lastValidMu.Unlock()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"cpuAggregator":["Max OverAll"],"memoryAggregator":["Max OverAll"],"evaluationInterval":"0s","evaluationInterval1d":"0h"}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
	assert.True(t, reverted, "zero-value durations must be rejected")
	// With cold cache, both fields fall back to the hardcoded defaults.
	assert.Equal(t, rightsizing.DefaultEvaluationInterval, configData.PrometheusRuleConfig.EvaluationInterval,
		"zero-value interval should be replaced by default, not just any non-zero value")
	assert.Equal(t, rightsizing.DefaultEvaluationInterval1d, configData.PrometheusRuleConfig.EvaluationInterval1d,
		"zero-value interval1d should be replaced by default")
}

// TestValidateAndSanitizeConfig_OutOfRangeDurationsAreRejected covers durations that are
// syntactically valid Prometheus durations but operationally dangerous: "1ms" would make
// every spoke re-evaluate the rule groups a thousand times a second, and "1y" expands the
// subquery lookback to over a million points per series.
func TestValidateAndSanitizeConfig_OutOfRangeDurationsAreRejected(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		evaluationInterval   string
		evaluationInterval1d string
	}{
		{"below minimum", "1ms", "1s"},
		{"above maximum", "1y", "2w"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lastValidMu.Lock()
			delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval"))
			delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval1d"))
			lastValidMu.Unlock()

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rightsizing.NamespaceConfigMapName,
					Namespace: addoncfg.InstallNamespace,
					Labels:    rightsizing.RSLabels(),
				},
				Data: map[string]string{
					"prometheusRuleConfig": fmt.Sprintf(
						`{"cpuAggregator":["Max OverAll"],"memoryAggregator":["Max OverAll"],`+
							`"evaluationInterval":%q,"evaluationInterval1d":%q}`,
						tc.evaluationInterval, tc.evaluationInterval1d),
				},
			}
			ob := newTestOptionsBuilder(t, cm)

			configData, err := rightsizing.ParseConfigMapData(cm.Data)
			require.NoError(t, err)

			reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
			assert.True(t, reverted, "out-of-range durations must be rejected")
			assert.Equal(t, rightsizing.DefaultEvaluationInterval, configData.PrometheusRuleConfig.EvaluationInterval)
			assert.Equal(t, rightsizing.DefaultEvaluationInterval1d, configData.PrometheusRuleConfig.EvaluationInterval1d)

			// The sanitized value is what reaches the generated PrometheusRule.
			rule, err := rsnamespace.GeneratePrometheusRule(configData)
			require.NoError(t, err)
			assert.Equal(t, rightsizing.DefaultEvaluationInterval, string(*rule.Spec.Groups[0].Interval))
			assert.NotContains(t, rule.Spec.Groups[0].Rules[0].Expr.String(), tc.evaluationInterval)
		})
	}
}

// TestValidateAndSanitizeConfig_IntervalCeilingsAreIndependent pins the asymmetry between
// the two fields. "2h" is too coarse for the rolling group — the daily rules would see
// only 12 samples per [1d] window — but it is a legitimate cadence for re-running the
// daily rules themselves, so only the first field may be rejected.
func TestValidateAndSanitizeConfig_IntervalCeilingsAreIndependent(t *testing.T) {
	lastValidMu.Lock()
	delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval"))
	delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval1d"))
	lastValidMu.Unlock()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"cpuAggregator":["Max OverAll"],"memoryAggregator":["Max OverAll"],` +
				`"evaluationInterval":"2h","evaluationInterval1d":"2h"}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
	assert.True(t, reverted, "the rolling interval is above its ceiling and must be rejected")
	assert.Equal(t, rightsizing.DefaultEvaluationInterval, configData.PrometheusRuleConfig.EvaluationInterval,
		"2h exceeds MaxEvaluationInterval and should fall back to the default")
	assert.Equal(t, "2h", configData.PrometheusRuleConfig.EvaluationInterval1d,
		"2h is within MaxEvaluationInterval1d and must be preserved")
}

// TestValidateAndSanitizeConfig_WastefulIntervalOrderingWarnsButKeepsConfig covers the
// case a user hits by raising only evaluationInterval: 30m is legal, but it is coarser
// than the untouched 15m default for evaluationInterval1d, so the daily rules recompute
// twice per new sample. That is wasteful, not wrong, and must not be reverted — otherwise
// a single legal edit would be silently undone because of a field the user never set.
func TestValidateAndSanitizeConfig_WastefulIntervalOrderingWarnsButKeepsConfig(t *testing.T) {
	lastValidMu.Lock()
	delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval"))
	delete(lastValidIntervals, cacheKey(rightsizing.NamespaceConfigMapName, "evaluationInterval1d"))
	lastValidMu.Unlock()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"cpuAggregator":["Max OverAll"],"memoryAggregator":["Max OverAll"],` +
				`"evaluationInterval":"30m","evaluationInterval1d":"15m"}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName)
	assert.False(t, reverted, "a wasteful but correct ordering must not trigger a revert")
	assert.Equal(t, "30m", configData.PrometheusRuleConfig.EvaluationInterval,
		"the user's legal evaluationInterval must be preserved")
	assert.Equal(t, "15m", configData.PrometheusRuleConfig.EvaluationInterval1d)

	// The ordering is surfaced as a diagnosable factor rather than a rejection.
	factor, warn := rightsizing.RedundantEvaluationFactor("30m", "15m")
	assert.True(t, warn)
	assert.Equal(t, 2, factor)

	// The user's value is what reaches the generated PrometheusRule.
	rule, err := rsnamespace.GeneratePrometheusRule(configData)
	require.NoError(t, err)
	assert.Equal(t, "30m", string(*rule.Spec.Groups[0].Interval))
	assert.Contains(t, rule.Spec.Groups[0].Rules[0].Expr.String(), "[30m:]")
}

// TestValidateAndSanitizeConfig_ClearingIntervalDoesNotResurrectOldValue is the key
// regression test for the "cache resurrects deleted interval" bug.
//
// Sequence that was broken:
//  1. evaluationInterval: "10m"  → cached as "10m"
//  2. evaluationInterval: ""     → old code: skip cache; "10m" stays   ← BUG
//     new code: delete cache entry; "10m" is gone               ← FIXED
//  3. evaluationInterval: "bad"  → invalid; cache miss → default "5m"
//
// The old code restored "10m" at step 3, silently reinstating a value the
// user had explicitly removed.
func TestValidateAndSanitizeConfig_ClearingIntervalDoesNotResurrectOldValue(t *testing.T) {
	cmName := rightsizing.NamespaceConfigMapName

	// Step 1: prime the cache with a custom value.
	cacheValidInterval(cmName, "evaluationInterval", "10m")
	cached, ok := getCachedInterval(cmName, "evaluationInterval")
	require.True(t, ok)
	require.Equal(t, "10m", cached, "pre-condition: cache must hold the custom value")

	// Step 2: user removes the key (empty string = "use default").
	// This must evict the cache entry, not leave "10m" behind.
	cacheValidInterval(cmName, "evaluationInterval", "")
	_, ok = getCachedInterval(cmName, "evaluationInterval")
	assert.False(t, ok, "clearing the interval must evict the cache entry")

	// Step 3: user now makes a typo.
	// With an empty cache the handler must use the hardcoded default, NOT resurrect "10m".
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": `{"cpuAggregator":["Max OverAll"],"memoryAggregator":["Max OverAll"],"evaluationInterval":"badvalue","evaluationInterval1d":"15m"}`,
		},
	}
	ob := newTestOptionsBuilder(t, cm)
	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	reverted := ob.validateAndSanitizeConfig(&configData, cmName)
	assert.True(t, reverted)
	assert.Equal(t, rightsizing.DefaultEvaluationInterval, configData.PrometheusRuleConfig.EvaluationInterval,
		"after clearing then mistyping, must fall back to the default, not resurrect the stale 10m")
}

// newIntervalConfigData returns the default right-sizing config with the two evaluation
// intervals replaced.
func newIntervalConfigData(evaluationInterval, evaluationInterval1d string) rightsizing.RSConfigMapData {
	configData := rightsizing.RSConfigMapData{
		PrometheusRuleConfig:   rightsizing.GetDefaultRSPrometheusRuleConfig(),
		PlacementConfiguration: rightsizing.GetDefaultRSPlacement(),
	}
	configData.PrometheusRuleConfig.EvaluationInterval = evaluationInterval
	configData.PrometheusRuleConfig.EvaluationInterval1d = evaluationInterval1d
	return configData
}

// newIntervalConfigMap returns a right-sizing ConfigMap the way MCOA writes it, with the
// two evaluation intervals replaced.
func newIntervalConfigMap(name, evaluationInterval, evaluationInterval1d string) *corev1.ConfigMap {
	configData := newIntervalConfigData(evaluationInterval, evaluationInterval1d)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig":   rightsizing.FormatYAML(configData.PrometheusRuleConfig),
			"placementConfiguration": rightsizing.FormatYAML(configData.PlacementConfiguration),
		},
	}
}

// TestValidateAndSanitizeConfig_InvalidIntervalRestoresItsOwnLastValidValue drives the
// interval cache the way reconciles do, through validateAndSanitizeConfig alone: a valid
// config is seen first, then an edit that breaks one or both intervals. Each broken field
// must come back as its own last valid value, and a valid edit made next to it must stay.
func TestValidateAndSanitizeConfig_InvalidIntervalRestoresItsOwnLastValidValue(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		editRolling, editDaily string
		wantRolling, wantDaily string
	}{
		{"only evaluationInterval is invalid", "2h", "1h", "10m", "1h"},
		{"only evaluationInterval1d is invalid", "20m", "2d", "20m", "30m"},
		{"evaluationInterval1d above its 6h ceiling is invalid", "20m", "24h", "20m", "30m"},
		{"evaluationInterval below its 5m floor is invalid", "4m", "1h", "10m", "1h"},
		{"evaluationInterval1d below its 15m floor is invalid", "20m", "5m", "20m", "30m"},
		{"both are invalid", "bogus", "0m", "10m", "30m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetIntervalState(t)
			ob := newTestOptionsBuilder(t)

			configData := newIntervalConfigData("10m", "30m")
			require.False(t, ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName),
				"pre-condition: the first config is valid")

			configData.PrometheusRuleConfig.EvaluationInterval = tc.editRolling
			configData.PrometheusRuleConfig.EvaluationInterval1d = tc.editDaily
			assert.True(t, ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName),
				"an invalid interval should trigger revert")
			assert.Equal(t, tc.wantRolling, configData.PrometheusRuleConfig.EvaluationInterval)
			assert.Equal(t, tc.wantDaily, configData.PrometheusRuleConfig.EvaluationInterval1d)
		})
	}
}

// orderingWarnings returns the wasteful-ordering warnings among the captured log lines.
func orderingWarnings(lines []string) []string {
	var warnings []string
	for _, line := range lines {
		if strings.Contains(line, "evaluationInterval is coarser than evaluationInterval1d") {
			warnings = append(warnings, line)
		}
	}
	return warnings
}

// TestValidateAndSanitizeConfig_WastefulIntervalOrderingIsLoggedOncePerChange pins when
// the wasteful-ordering warning is written. Build runs for every managed cluster on every
// reconcile and a wasteful ordering is never reverted, so the warning has to appear when
// the interval pair becomes wasteful and then stay quiet until the pair changes.
func TestValidateAndSanitizeConfig_WastefulIntervalOrderingIsLoggedOncePerChange(t *testing.T) {
	// validate does what one reconcile does for a ConfigMap holding the given intervals.
	validate := func(t *testing.T, ob *OptionsBuilder, configMapName, rolling, daily string) {
		t.Helper()
		configData := newIntervalConfigData(rolling, daily)
		require.False(t, ob.validateAndSanitizeConfig(&configData, configMapName),
			"both intervals are valid, so nothing may be reverted")
	}

	t.Run("logged once however often the same pair is validated", func(t *testing.T) {
		resetIntervalState(t)
		ob, lines := newLoggingOptionsBuilder(t)

		for range 3 {
			validate(t, ob, rightsizing.NamespaceConfigMapName, "1h", "20m")
		}

		warnings := orderingWarnings(*lines)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], `"configMap"="rs-namespace-config"`)
		assert.Contains(t, warnings[0], `"evaluationInterval"="1h"`)
		assert.Contains(t, warnings[0], `"evaluationInterval1d"="20m"`)
		assert.Contains(t, warnings[0], `"redundantEvaluationsPerSample"=3`)
	})

	t.Run("not logged for a well-ordered pair", func(t *testing.T) {
		resetIntervalState(t)
		ob, lines := newLoggingOptionsBuilder(t)

		validate(t, ob, rightsizing.NamespaceConfigMapName, "5m", "15m")
		validate(t, ob, rightsizing.NamespaceConfigMapName, "15m", "15m")
		validate(t, ob, rightsizing.NamespaceConfigMapName, "", "")

		assert.Empty(t, orderingWarnings(*lines))
	})

	t.Run("logged again when the wasteful pair changes or comes back", func(t *testing.T) {
		resetIntervalState(t)
		ob, lines := newLoggingOptionsBuilder(t)

		validate(t, ob, rightsizing.NamespaceConfigMapName, "30m", "15m") // wasteful: warn
		validate(t, ob, rightsizing.NamespaceConfigMapName, "1h", "15m")  // another wasteful pair: warn
		validate(t, ob, rightsizing.NamespaceConfigMapName, "1h", "1h")   // fixed: quiet
		validate(t, ob, rightsizing.NamespaceConfigMapName, "1h", "15m")  // reintroduced: warn

		assert.Len(t, orderingWarnings(*lines), 3)
	})

	t.Run("an empty evaluationInterval1d is reported as the default that runs", func(t *testing.T) {
		resetIntervalState(t)
		ob, lines := newLoggingOptionsBuilder(t)

		// evaluationInterval1d is empty, so the daily groups run at the 15m default, which
		// is finer than an evaluationInterval of 1h. An empty evaluationInterval never
		// triggers the warning: its 5m default is finer than any valid evaluationInterval1d.
		validate(t, ob, rightsizing.NamespaceConfigMapName, "1h", "")
		// Spelling the default out is the same configuration, not a change.
		validate(t, ob, rightsizing.NamespaceConfigMapName, "1h", rightsizing.DefaultEvaluationInterval1d)

		warnings := orderingWarnings(*lines)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], `"evaluationInterval"="1h"`)
		assert.Contains(t, warnings[0], `"evaluationInterval1d"="15m"`)
		assert.Contains(t, warnings[0], `"redundantEvaluationsPerSample"=4`)
	})

	t.Run("tracked per ConfigMap", func(t *testing.T) {
		resetIntervalState(t)
		ob, lines := newLoggingOptionsBuilder(t)

		validate(t, ob, rightsizing.NamespaceConfigMapName, "30m", "15m")
		validate(t, ob, rightsizing.VirtualizationConfigMapName, "30m", "15m")

		warnings := orderingWarnings(*lines)
		require.Len(t, warnings, 2)
		assert.Contains(t, warnings[0], `"configMap"="rs-namespace-config"`)
		assert.Contains(t, warnings[1], `"configMap"="rs-virt-config"`)
	})
}

// assertGroupIntervals checks every rule group of a generated PrometheusRule: the rolling
// groups run at wantRolling and use it as their subquery range, the daily groups run at
// wantDaily.
func assertGroupIntervals(t *testing.T, rule *monitoringv1.PrometheusRule, wantRolling, wantDaily string) {
	t.Helper()
	require.Len(t, rule.Spec.Groups, 4, "groups in %s", rule.Name)
	for _, group := range rule.Spec.Groups {
		require.NotNil(t, group.Interval, "interval of group %s", group.Name)
		switch {
		case strings.Contains(group.Name, "-5m.rule"):
			assert.Equal(t, wantRolling, string(*group.Interval), "interval of group %s", group.Name)
			for _, r := range group.Rules {
				assert.Contains(t, r.Expr.String(), "["+wantRolling+":]", "subquery range of %s", r.Record)
			}
		case strings.Contains(group.Name, "-1d.rule"):
			assert.Equal(t, wantDaily, string(*group.Interval), "interval of group %s", group.Name)
		default:
			t.Errorf("unexpected rule group %q in %s", group.Name, rule.Name)
		}
	}
}

// intervalPair is the evaluationInterval / evaluationInterval1d pair of one ConfigMap.
type intervalPair struct{ rolling, daily string }

// TestBuild_EvaluationIntervals follows the two interval keys through a whole reconcile:
// ConfigMap on the hub, validation and write-back, generated PrometheusRules. The
// namespace and the virtualization ConfigMap hold different values, so a result that
// leaks from one feature into the other fails the test.
func TestBuild_EvaluationIntervals(t *testing.T) {
	defaults := intervalPair{rightsizing.DefaultEvaluationInterval, rightsizing.DefaultEvaluationInterval1d}
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spoke-1",
			Labels: map[string]string{
				clusterinfov1beta1.LabelKubeVendor: string(clusterinfov1beta1.KubeVendorOpenShift),
			},
		},
	}

	// setIntervals creates or updates a ConfigMap on the hub so it holds the given pair.
	setIntervals := func(t *testing.T, ob *OptionsBuilder, name string, pair intervalPair) {
		t.Helper()
		cm := newIntervalConfigMap(name, pair.rolling, pair.daily)
		var existing corev1.ConfigMap
		err := ob.Client.Get(t.Context(), types.NamespacedName{Name: name, Namespace: addoncfg.InstallNamespace}, &existing)
		if apierrors.IsNotFound(err) {
			require.NoError(t, ob.Client.Create(t.Context(), cm))
			return
		}
		require.NoError(t, err)
		existing.Data = cm.Data
		require.NoError(t, ob.Client.Update(t.Context(), &existing))
	}

	// assertBuild runs Build with both features enabled, then checks, per feature, the
	// intervals of the generated rule groups and the pair left in its ConfigMap.
	assertBuild := func(t *testing.T, ob *OptionsBuilder, wantNamespace, wantVirtualization intervalPair) {
		t.Helper()
		opts, err := ob.Build(t.Context(), cluster, newPlatformOpts(true, true))
		require.NoError(t, err)

		for _, feature := range []struct {
			configMapName string
			component     ComponentOptions
			want          intervalPair
		}{
			{rightsizing.NamespaceConfigMapName, opts.NamespaceRightSizing, wantNamespace},
			{rightsizing.VirtualizationConfigMapName, opts.VirtualizationRightSizing, wantVirtualization},
		} {
			require.True(t, feature.component.Enabled, "feature of %s", feature.configMapName)
			require.Len(t, feature.component.PrometheusRules, 1, "rules of %s", feature.configMapName)
			assertGroupIntervals(t, feature.component.PrometheusRules[0], feature.want.rolling, feature.want.daily)

			var cm corev1.ConfigMap
			require.NoError(t, ob.Client.Get(t.Context(), types.NamespacedName{
				Name: feature.configMapName, Namespace: addoncfg.InstallNamespace,
			}, &cm))
			stored, err := rightsizing.ParseConfigMapData(cm.Data)
			require.NoError(t, err)
			assert.Equal(t, feature.want.rolling, stored.PrometheusRuleConfig.EvaluationInterval,
				"evaluationInterval stored in %s", feature.configMapName)
			assert.Equal(t, feature.want.daily, stored.PrometheusRuleConfig.EvaluationInterval1d,
				"evaluationInterval1d stored in %s", feature.configMapName)
		}
	}

	t.Run("missing ConfigMaps are created with the defaults", func(t *testing.T) {
		resetIntervalState(t)
		ob := newTestOptionsBuilder(t)

		assertBuild(t, ob, defaults, defaults)
	})

	t.Run("custom intervals are used as they are", func(t *testing.T) {
		resetIntervalState(t)
		ob := newTestOptionsBuilder(t)
		setIntervals(t, ob, rightsizing.NamespaceConfigMapName, intervalPair{"10m", "1h"})
		setIntervals(t, ob, rightsizing.VirtualizationConfigMapName, intervalPair{"20m", "2h"})

		assertBuild(t, ob, intervalPair{"10m", "1h"}, intervalPair{"20m", "2h"})
	})

	t.Run("an invalid interval is replaced by its default and written back", func(t *testing.T) {
		resetIntervalState(t)
		ob := newTestOptionsBuilder(t)
		// Both keys are invalid in one ConfigMap, only evaluationInterval1d in the other.
		setIntervals(t, ob, rightsizing.NamespaceConfigMapName, intervalPair{"2h", "abc"})
		setIntervals(t, ob, rightsizing.VirtualizationConfigMapName, intervalPair{"20m", "2d"})

		assertBuild(t, ob, defaults, intervalPair{"20m", rightsizing.DefaultEvaluationInterval1d})
	})

	t.Run("evaluationInterval1d is kept at its 6h ceiling and rejected above it", func(t *testing.T) {
		resetIntervalState(t)
		ob := newTestOptionsBuilder(t)
		// 24h, one run per aggregation window, leaves no gaps but too few hub samples per day.
		setIntervals(t, ob, rightsizing.NamespaceConfigMapName, intervalPair{"5m", "6h"})
		setIntervals(t, ob, rightsizing.VirtualizationConfigMapName, intervalPair{"5m", "24h"})

		assertBuild(t, ob, intervalPair{"5m", "6h"}, intervalPair{"5m", rightsizing.DefaultEvaluationInterval1d})
	})

	t.Run("intervals are kept at their floors and rejected below them", func(t *testing.T) {
		resetIntervalState(t)
		ob := newTestOptionsBuilder(t)
		// 300s and 900s are the floors spelled differently from the 5m and 15m defaults, so
		// keeping them cannot be mistaken for a revert to the defaults. 5m is the rolling
		// floor, which is still below the daily one.
		setIntervals(t, ob, rightsizing.NamespaceConfigMapName, intervalPair{"300s", "900s"})
		setIntervals(t, ob, rightsizing.VirtualizationConfigMapName, intervalPair{"4m", "5m"})

		assertBuild(t, ob, intervalPair{"300s", "900s"}, defaults)
	})

	t.Run("an invalid edit keeps the intervals of the last valid config", func(t *testing.T) {
		resetIntervalState(t)
		ob := newTestOptionsBuilder(t)
		setIntervals(t, ob, rightsizing.NamespaceConfigMapName, intervalPair{"10m", "1h"})
		setIntervals(t, ob, rightsizing.VirtualizationConfigMapName, intervalPair{"20m", "2h"})
		assertBuild(t, ob, intervalPair{"10m", "1h"}, intervalPair{"20m", "2h"})

		// The user now breaks both keys in both ConfigMaps.
		setIntervals(t, ob, rightsizing.NamespaceConfigMapName, intervalPair{"2h", "abc"})
		setIntervals(t, ob, rightsizing.VirtualizationConfigMapName, intervalPair{"0m", "2d"})

		assertBuild(t, ob, intervalPair{"10m", "1h"}, intervalPair{"20m", "2h"})
	})
}

// TestRevertConfigMap_IntervalRevertKeepsOtherFields checks that writing back a rejected
// interval leaves the rest of the user's configuration alone: the write-back rewrites the
// whole prometheusRuleConfig key, so every other field has to survive the round trip, and
// placementConfiguration, a separate key, must not be touched at all.
func TestRevertConfigMap_IntervalRevertKeepsOtherFields(t *testing.T) {
	resetIntervalState(t)

	ruleConfig := `namespaceFilterCriteria:
  inclusionCriteria:
  - team-.*
labelFilterCriteria:
- labelName: label_env
  inclusionCriteria:
  - prod
recommendationPercentage: 125
cpuAggregator:
- P90
memoryAggregator:
- Max OverAll
- P99
evaluationInterval: 2h
evaluationInterval1d: 30m
`
	placement := `{"spec":{"predicates":[{"requiredClusterSelector":{"labelSelector":{"matchLabels":{"env":"prod"}}}}]}}`
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig":   ruleConfig,
			"placementConfiguration": placement,
		},
	}
	ob := newTestOptionsBuilder(t, cm)
	ctx := t.Context()

	before, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err)

	configData := before
	require.True(t, ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName),
		"2h is above the rolling ceiling and must be rejected")
	// Apart from the interval, the fixture has to be a configuration the rule builder
	// accepts; otherwise the round trip would be checked on a ConfigMap no hub can use.
	_, err = rsnamespace.GeneratePrometheusRule(configData)
	require.NoError(t, err, "the sanitized fixture must generate rules")
	ob.revertConfigMap(ctx, rightsizing.NamespaceConfigMapName, configData)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))
	assert.Equal(t, placement, updated.Data["placementConfiguration"],
		"placementConfiguration is a separate key and must not be rewritten")

	after, err := rightsizing.ParseConfigMapData(updated.Data)
	require.NoError(t, err)

	// Only the rejected field may differ.
	want := before.PrometheusRuleConfig
	want.EvaluationInterval = rightsizing.DefaultEvaluationInterval
	assert.Equal(t, want, after.PrometheusRuleConfig)
}

// TestValidateAndSanitizeConfig_YAMLNumberIntervalIsRejected covers a user who writes an
// interval without a unit. YAML reads it as a number, which the ConfigMap parser turns
// into the string "5"; that must be rejected like any other malformed value, not fail
// the reconcile, and the write-back must replace it with a proper duration.
func TestValidateAndSanitizeConfig_YAMLNumberIntervalIsRejected(t *testing.T) {
	resetIntervalState(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.NamespaceConfigMapName,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: map[string]string{
			"prometheusRuleConfig": "cpuAggregator:\n- Max OverAll\nmemoryAggregator:\n- Max OverAll\n" +
				"evaluationInterval: 5\nevaluationInterval1d: 30\n",
		},
	}
	ob := newTestOptionsBuilder(t, cm)
	ctx := t.Context()

	configData, err := rightsizing.ParseConfigMapData(cm.Data)
	require.NoError(t, err, "a bare number must not make the ConfigMap unparseable")
	require.Equal(t, "5", configData.PrometheusRuleConfig.EvaluationInterval)
	require.Equal(t, "30", configData.PrometheusRuleConfig.EvaluationInterval1d)

	assert.True(t, ob.validateAndSanitizeConfig(&configData, rightsizing.NamespaceConfigMapName))
	assert.Equal(t, rightsizing.DefaultEvaluationInterval, configData.PrometheusRuleConfig.EvaluationInterval)
	assert.Equal(t, rightsizing.DefaultEvaluationInterval1d, configData.PrometheusRuleConfig.EvaluationInterval1d)

	ob.revertConfigMap(ctx, rightsizing.NamespaceConfigMapName, configData)

	var updated corev1.ConfigMap
	require.NoError(t, ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &updated))
	assert.Contains(t, updated.Data["prometheusRuleConfig"],
		"evaluationInterval: "+rightsizing.DefaultEvaluationInterval+"\n")
	assert.Contains(t, updated.Data["prometheusRuleConfig"],
		"evaluationInterval1d: "+rightsizing.DefaultEvaluationInterval1d+"\n")
}
