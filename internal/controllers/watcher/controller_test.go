package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	addoncommon "github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestEnqueueForConfigResource(t *testing.T) {
	existingSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "foo",
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	newSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "foo",
		},
		Data: map[string][]byte{
			"foo": []byte("baz"),
		},
	}

	newSecretNoGVK := newSecret.DeepCopy()
	newSecretNoGVK.SetGroupVersionKind(schema.GroupVersionKind{})

	existingConfigmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "bar",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}
	newConfigmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "bar",
		},
		Data: map[string]string{
			"foo": "baz",
		},
	}

	for _, tc := range []struct {
		name                      string
		object                    runtime.Object
		manifests                 []workv1.Manifest
		expectedReconcileRequests []reconcile.Request
	}{
		{
			name:   "reconcile secret in manifests",
			object: newSecret,
			manifests: []workv1.Manifest{
				{
					RawExtension: runtime.RawExtension{
						Object: existingSecret,
					},
				},
			},
			expectedReconcileRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "multicluster-observability-addon",
						Namespace: "test-namespace",
					},
				},
			},
		},
		{
			name:   "reconcile secret with empty GVK (simulating Informer)",
			object: newSecretNoGVK,
			manifests: []workv1.Manifest{
				{
					RawExtension: runtime.RawExtension{
						Object: existingSecret,
					},
				},
			},
			expectedReconcileRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "multicluster-observability-addon",
						Namespace: "test-namespace",
					},
				},
			},
		},
		{
			name:   "reconcile configmap in manifests",
			object: newConfigmap,
			manifests: []workv1.Manifest{
				{
					RawExtension: runtime.RawExtension{
						Object: existingConfigmap,
					},
				},
			},
			expectedReconcileRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "multicluster-observability-addon",
						Namespace: "test-namespace",
					},
				},
			},
		},
		{
			name:   "dont reconcile if the resource doesn't have updates",
			object: existingConfigmap,
			manifests: []workv1.Manifest{
				{
					RawExtension: runtime.RawExtension{
						Object: existingConfigmap,
					},
				},
			},
			expectedReconcileRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "multicluster-observability-addon",
						Namespace: "test-namespace",
					},
				},
			},
		},
		{
			name:   "dont reconcile if resource not in manifests",
			object: existingSecret,
			manifests: []workv1.Manifest{
				{
					RawExtension: runtime.RawExtension{
						Object: existingConfigmap,
					},
				},
			},
			expectedReconcileRequests: []reconcile.Request{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifestWork := &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-manifestwork",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"open-cluster-management.io/addon-name": "multicluster-observability-addon",
					},
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: tc.manifests,
					},
				},
			}

			// Create a fake client
			s := scheme.Scheme
			_ = workv1.Install(s)
			cl := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(manifestWork).
				Build()

			r := &WatcherReconciler{
				Client: cl,
				Scheme: s,
				Cache:  NewReferenceCache(),
			}

			// Populate cache
			keys := map[string]struct{}{}
			decode := serializer.NewCodecFactory(r.Scheme).UniversalDeserializer().Decode
			for i, m := range tc.manifests {
				if m.Raw == nil && m.Object != nil {
					raw, err := json.Marshal(m.Object)
					if err != nil {
						t.Errorf("failed to marshal object: %v", err)
						continue
					}
					tc.manifests[i].Raw = raw
					m.Raw = raw
				}
				obj, _, err := decode(m.Raw, nil, nil)
				if err != nil {
					t.Errorf("failed to decode manifest: %v", err)
					continue
				}
				clientObj, ok := obj.(client.Object)
				if !ok {
					continue
				}
				keys[r.getConfigResourceKey(clientObj)] = struct{}{}
			}
			r.Cache.Add(manifestWork.Namespace, manifestWork.Name, keys)

			cliObj := tc.object.(client.Object)

			h := r.enqueueForConfigResource()
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

			h.Create(context.Background(), event.CreateEvent{Object: cliObj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.ElementsMatch(t, tc.expectedReconcileRequests, actual)
		})
	}
}

func TestIsHypershiftServiceMonitor(t *testing.T) {
	hypershiftOwner := metav1.OwnerReference{APIVersion: hyperv1.GroupVersion.String()}
	nonHypershiftOwner := metav1.OwnerReference{APIVersion: "apps/v1"}
	alphaApiVersion := hyperv1.GroupVersion
	alphaApiVersion.Version = "v1alpha1"
	hypershiftWithOtherAPIVersionOwner := metav1.OwnerReference{APIVersion: alphaApiVersion.String()}

	testCases := []struct {
		name           string
		inputObject    client.Object
		expectedResult bool
	}{
		{
			name:           "hypershift etcd serviceMonitor with correct owner",
			inputObject:    createTestObject(mconfig.HypershiftEtcdServiceMonitorName, []metav1.OwnerReference{hypershiftOwner}),
			expectedResult: true,
		},
		{
			name:           "hypershift apiServer serviceMonitor with correct owner",
			inputObject:    createTestObject(mconfig.HypershiftApiServerServiceMonitorName, []metav1.OwnerReference{hypershiftOwner}),
			expectedResult: true,
		},
		{
			name:           "hypershift serviceMonitor with non-hypershift owner",
			inputObject:    createTestObject(mconfig.HypershiftEtcdServiceMonitorName, []metav1.OwnerReference{nonHypershiftOwner}),
			expectedResult: false,
		},
		{
			name:           "hypershift serviceMonitor with multiple owners, one correct",
			inputObject:    createTestObject(mconfig.HypershiftEtcdServiceMonitorName, []metav1.OwnerReference{nonHypershiftOwner, hypershiftOwner}),
			expectedResult: true,
		},
		{
			name:           "hypershift serviceMonitor with other APIVersion owner",
			inputObject:    createTestObject(mconfig.HypershiftApiServerServiceMonitorName, []metav1.OwnerReference{hypershiftWithOtherAPIVersionOwner}),
			expectedResult: true,
		},
		{
			name:           "unrelated serviceMonitor name",
			inputObject:    createTestObject("random-monitor", []metav1.OwnerReference{hypershiftOwner}),
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, isHypershiftServiceMonitor(logr.Discard(), tc.inputObject))
		})
	}
}

func createTestObject(name string, owners []metav1.OwnerReference) client.Object {
	u := &unstructured.Unstructured{}
	u.SetName(name)
	u.SetOwnerReferences(owners)
	return u
}

func TestMCHNetworkPoliciesPredicate(t *testing.T) {
	createMCH := func(enabled *bool) *unstructured.Unstructured {
		u := addoncommon.NewMultiClusterHub()
		if enabled != nil {
			_ = unstructured.SetNestedField(u.Object, *enabled, "spec", "networkPolicies", "enabled")
		}
		return u
	}

	trueVal := true
	falseVal := false

	t.Run("CreateFunc", func(t *testing.T) {
		assert.True(t, mchNetworkPoliciesPredicate.Create(event.CreateEvent{Object: createMCH(&trueVal)}))
		assert.False(t, mchNetworkPoliciesPredicate.Create(event.CreateEvent{Object: createMCH(&falseVal)}))
		assert.False(t, mchNetworkPoliciesPredicate.Create(event.CreateEvent{Object: createMCH(nil)}))
		assert.False(t, mchNetworkPoliciesPredicate.Create(event.CreateEvent{Object: &corev1.ConfigMap{}}))
		assert.False(t, mchNetworkPoliciesPredicate.Create(event.CreateEvent{}))
	})

	t.Run("UpdateFunc", func(t *testing.T) {
		// Enabled flipped false -> true: must trigger
		assert.True(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: createMCH(&falseVal),
			ObjectNew: createMCH(&trueVal),
		}))

		// Enabled flipped true -> false: must trigger
		assert.True(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: createMCH(&trueVal),
			ObjectNew: createMCH(&falseVal),
		}))

		// Enabled missing -> true: must trigger
		assert.True(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: createMCH(nil),
			ObjectNew: createMCH(&trueVal),
		}))

		// Enabled true -> missing (implicitly false): must trigger
		assert.True(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: createMCH(&trueVal),
			ObjectNew: createMCH(nil),
		}))

		// Unchanged (true -> true): must NOT trigger (prevents spurious reconciles on status updates)
		assert.False(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: createMCH(&trueVal),
			ObjectNew: createMCH(&trueVal),
		}))

		// Unchanged (false -> false): must NOT trigger
		assert.False(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: createMCH(&falseVal),
			ObjectNew: createMCH(&falseVal),
		}))

		// Unchanged (nil -> nil): must NOT trigger
		assert.False(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: createMCH(nil),
			ObjectNew: createMCH(nil),
		}))

		// Non-unstructured objects: safe fallback without panic
		assert.False(t, mchNetworkPoliciesPredicate.Update(event.UpdateEvent{
			ObjectOld: &corev1.ConfigMap{},
			ObjectNew: &corev1.ConfigMap{},
		}))
	})

	t.Run("DeleteFunc and GenericFunc", func(t *testing.T) {
		assert.False(t, mchNetworkPoliciesPredicate.Delete(event.DeleteEvent{Object: createMCH(&trueVal)}))
		assert.False(t, mchNetworkPoliciesPredicate.Generic(event.GenericEvent{Object: createMCH(&trueVal)}))
	})
}

func TestUpdateCache(t *testing.T) {
	s := scheme.Scheme
	_ = workv1.Install(s)
	_ = corev1.AddToScheme(s)

	tests := []struct {
		name         string
		obj          client.Object
		expectedKeys []string
		shouldExist  bool // true if ManifestWork and processed
	}{
		{
			name: "ManifestWork with valid Secret and ConfigMap",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mw1",
					Namespace: "cluster1",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: mustMarshal(&corev1.Secret{
										TypeMeta: metav1.TypeMeta{
											Kind:       "Secret",
											APIVersion: "v1",
										},
										ObjectMeta: metav1.ObjectMeta{
											Name:      "secret1",
											Namespace: "ns1",
											Annotations: map[string]string{
												addoncfg.AnnotationOriginalResource: "source-ns/source-secret",
											},
										},
									}),
								},
							},
							{
								RawExtension: runtime.RawExtension{
									Raw: mustMarshal(&corev1.ConfigMap{
										TypeMeta: metav1.TypeMeta{
											Kind:       "ConfigMap",
											APIVersion: "v1",
										},
										ObjectMeta: metav1.ObjectMeta{
											Name:      "cm1",
											Namespace: "ns1",
											Annotations: map[string]string{
												addoncfg.AnnotationOriginalResource: "source-ns/source-cm",
											},
										},
									}),
								},
							},
						},
					},
				},
			},
			expectedKeys: []string{
				"/Secret/source-ns/source-secret",
				"/ConfigMap/source-ns/source-cm",
			},
			shouldExist: true,
		},
		{
			name: "ManifestWork with missing annotations",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mw2",
					Namespace: "cluster1",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: mustMarshal(&corev1.Secret{
										TypeMeta: metav1.TypeMeta{
											Kind:       "Secret",
											APIVersion: "v1",
										},
										ObjectMeta: metav1.ObjectMeta{
											Name:      "secret2",
											Namespace: "ns1",
										},
									}),
								},
							},
						},
					},
				},
			},
			expectedKeys: []string{},
			shouldExist:  true,
		},
		{
			name: "ManifestWork with invalid annotation format",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mw3",
					Namespace: "cluster1",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: mustMarshal(&corev1.Secret{
										TypeMeta: metav1.TypeMeta{
											Kind:       "Secret",
											APIVersion: "v1",
										},
										ObjectMeta: metav1.ObjectMeta{
											Name:      "secret3",
											Namespace: "ns1",
											Annotations: map[string]string{
												addoncfg.AnnotationOriginalResource: "invalid-format",
											},
										},
									}),
								},
							},
						},
					},
				},
			},
			expectedKeys: []string{},
			shouldExist:  true,
		},
		{
			name: "ManifestWork with non-config resource",
			obj: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mw4",
					Namespace: "cluster1",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: mustMarshal(&corev1.Service{
										TypeMeta: metav1.TypeMeta{
											Kind:       "Service",
											APIVersion: "v1",
										},
										ObjectMeta: metav1.ObjectMeta{
											Name:      "svc1",
											Namespace: "ns1",
											Annotations: map[string]string{
												addoncfg.AnnotationOriginalResource: "source-ns/source-svc",
											},
										},
									}),
								},
							},
						},
					},
				},
			},
			expectedKeys: []string{},
			shouldExist:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &WatcherReconciler{
				Log:    logr.Discard(),
				Scheme: s,
				Cache:  NewReferenceCache(),
			}

			r.updateCache(tt.obj.(*workv1.ManifestWork))

			if !tt.shouldExist {
				// Verify cache is empty
				r.Cache.RLock()
				defer r.Cache.RUnlock()
				assert.Empty(t, r.Cache.mwKeyToConfigs)
				return
			}

			// Verify keys are added
			for _, key := range tt.expectedKeys {
				namespaces := r.Cache.GetNamespaces(key)
				assert.Contains(t, namespaces, tt.obj.GetNamespace())
			}

			// Verify exact match of keys for the MW
			mwKey := fmt.Sprintf("%s/%s", tt.obj.GetNamespace(), tt.obj.GetName())
			r.Cache.RLock()
			configs, exists := r.Cache.mwKeyToConfigs[mwKey]
			r.Cache.RUnlock()

			assert.True(t, exists, "ManifestWork key should exist in cache")
			assert.Len(t, configs, len(tt.expectedKeys), "Number of config keys should match")

			for _, k := range tt.expectedKeys {
				_, ok := configs[k]
				assert.True(t, ok, "Config key %s should be in mwKeyToConfigs", k)
			}
		})
	}
}

func mustMarshal(obj any) []byte {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return b
}

type fakeAddonManager struct {
	addonmanager.AddonManager
	mu       sync.Mutex
	triggers []types.NamespacedName
}

func (f *fakeAddonManager) Trigger(clusterName, addonName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggers = append(f.triggers, types.NamespacedName{
		Namespace: clusterName,
		Name:      addonName,
	})
}

func (f *fakeAddonManager) getTriggers() []types.NamespacedName {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]types.NamespacedName, len(f.triggers))
	copy(out, f.triggers)
	return out
}

func (f *fakeAddonManager) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggers = nil
}

func drainQueue(q workqueue.TypedRateLimitingInterface[reconcile.Request]) []reconcile.Request {
	var requests []reconcile.Request
	for q.Len() > 0 {
		item, _ := q.Get()
		requests = append(requests, item)
		q.Done(item)
	}
	return requests
}

// TestWatcherReconciler_EnqueueForAllManagedClusters verifies that enqueueForAllManagedClusters fans out
// reconciliation requests to all managed clusters with active MCOA ManifestWorks, as used by both the
// global images ConfigMap and MultiClusterHub network policies watches.
func TestWatcherReconciler_EnqueueForAllManagedClusters(t *testing.T) {
	ctx := t.Context()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, workv1.Install(s))

	clusters := []string{"cluster-east", "cluster-west", "cluster-central"}
	var initObjs []client.Object
	for _, cluster := range clusters {
		initObjs = append(initObjs, &workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mcoa-work",
				Namespace: cluster,
				Labels: map[string]string{
					addoncfg.LabelOCMAddonName: addoncfg.Name,
				},
			},
		})
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initObjs...).Build()
	addonMgr := &fakeAddonManager{}
	reconciler := &WatcherReconciler{
		Client:       fakeClient,
		Log:          logr.Discard(),
		Scheme:       s,
		addonManager: addonMgr,
		Cache:        NewReferenceCache(),
	}

	imgConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mconfig.ImagesConfigMapObjKey.Name,
			Namespace: mconfig.ImagesConfigMapObjKey.Namespace,
		},
	}

	h := reconciler.enqueueForAllManagedClusters()
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	h.Update(ctx, event.UpdateEvent{ObjectOld: imgConfigMap, ObjectNew: imgConfigMap}, q)

	requests := drainQueue(q)
	assert.Len(t, requests, len(clusters))

	for _, req := range requests {
		_, err := reconciler.Reconcile(ctx, req)
		require.NoError(t, err)
	}

	triggers := addonMgr.getTriggers()
	assert.Len(t, triggers, len(clusters))
	triggeredClusters := make([]string, 0, len(triggers))
	for _, tr := range triggers {
		triggeredClusters = append(triggeredClusters, tr.Namespace)
	}
	for _, c := range clusters {
		assert.Contains(t, triggeredClusters, c)
	}
}

func TestWatcherReconciler_HypershiftServiceMonitorDiscovery(t *testing.T) {
	ctx := t.Context()

	addonMgr := &fakeAddonManager{}
	reconciler := &WatcherReconciler{
		Log:          logr.Discard(),
		addonManager: addonMgr,
		Cache:        NewReferenceCache(),
	}

	// Positive Test 1: ServiceMonitor "etcd" owned by HostedCluster
	etcdSm := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mconfig.HypershiftEtcdServiceMonitorName,
			Namespace: "clusters-hosted-cluster-1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: hyperv1.GroupVersion.String(),
					Kind:       "HostedCluster",
					Name:       "hosted-cluster-1",
					UID:        types.UID("12345"),
				},
			},
		},
	}

	assert.True(t, isHypershiftServiceMonitor(logr.Discard(), etcdSm), "etcd ServiceMonitor owned by HostedCluster must match")

	// Map to local-cluster request
	h := reconciler.enqueueForLocalCluster()
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	h.Create(ctx, event.CreateEvent{Object: etcdSm}, q)

	requests := drainQueue(q)
	require.Len(t, requests, 1)
	assert.Equal(t, "local-cluster", requests[0].Namespace)
	assert.Equal(t, addoncfg.Name, requests[0].Name)

	_, err := reconciler.Reconcile(ctx, requests[0])
	require.NoError(t, err)

	triggers := addonMgr.getTriggers()
	require.Len(t, triggers, 1)
	assert.Equal(t, "local-cluster", triggers[0].Namespace)
	assert.Equal(t, addoncfg.Name, triggers[0].Name)

	// Positive Test 2: ServiceMonitor "kube-apiserver" owned by HostedCluster
	addonMgr.reset()
	apiserverSm := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mconfig.HypershiftApiServerServiceMonitorName,
			Namespace: "clusters-hosted-cluster-1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: hyperv1.GroupVersion.String(),
					Kind:       "HostedCluster",
					Name:       "hosted-cluster-1",
					UID:        types.UID("12345"),
				},
			},
		},
	}
	assert.True(t, isHypershiftServiceMonitor(logr.Discard(), apiserverSm), "kube-apiserver ServiceMonitor owned by HostedCluster must match")

	// Negative Test 1: ServiceMonitor "etcd" owned by non-hypershift CRD
	unrelatedOwnerSm := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mconfig.HypershiftEtcdServiceMonitorName,
			Namespace: "clusters-hosted-cluster-1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "monitoring.coreos.com/v1",
					Kind:       "Prometheus",
					Name:       "k8s",
					UID:        types.UID("67890"),
				},
			},
		},
	}
	assert.False(t, isHypershiftServiceMonitor(logr.Discard(), unrelatedOwnerSm), "etcd ServiceMonitor with non-HostedCluster owner must NOT match")

	// Negative Test 2: ServiceMonitor not named etcd or kube-apiserver even if owned by HostedCluster
	otherAppSm := &prometheusv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-application-monitor",
			Namespace: "clusters-hosted-cluster-1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: hyperv1.GroupVersion.String(),
					Kind:       "HostedCluster",
					Name:       "hosted-cluster-1",
					UID:        types.UID("12345"),
				},
			},
		},
	}
	assert.False(t, isHypershiftServiceMonitor(logr.Discard(), otherAppSm), "unrelated ServiceMonitor name must NOT match even if owned by HostedCluster")
}
