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
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
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
	require.NoError(t, addonv1alpha1.Install(scheme))
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
	return newPlatformOptsAll(nsEnabled, virtEnabled, false)
}

func newPlatformOptsAll(nsEnabled, virtEnabled, wlEnabled bool) addon.Options {
	return addon.Options{
		Platform: addon.PlatformOptions{
			Enabled: true,
			AnalyticsOptions: addon.AnalyticsOptions{
				RightSizing: addon.RightSizingOptions{
					NamespaceEnabled:      nsEnabled,
					VirtualizationEnabled: virtEnabled,
					WorkloadPodEnabled:    wlEnabled,
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
	rsWlCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: rightsizing.WorkloadConfigMapName, Namespace: addoncfg.InstallNamespace,
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
	assert.True(t, pred.CreateFunc(event.CreateEvent{Object: rsWlCM}))
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

func TestReconcileRSResources_CleanupWorkload(t *testing.T) {
	wlCM := createTestConfigMap(rightsizing.WorkloadConfigMapName)
	wlPlacementCM := createTestConfigMap(rightsizing.WorkloadPlacementCMName)

	ob := newTestOptionsBuilder(t, wlCM, wlPlacementCM)
	ctx := context.TODO()

	opts := newPlatformOptsAll(false, false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.WorkloadConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "workload configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.WorkloadPlacementCMName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "workload placement configmap should be deleted")
}

func TestReconcileRSResources_CleanupIdempotent(t *testing.T) {
	ob := newTestOptionsBuilder(t)
	ctx := context.TODO()

	opts := newPlatformOptsAll(false, false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)
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
