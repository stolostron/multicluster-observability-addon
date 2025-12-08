package watcher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
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
			s.AddKnownTypes(workv1.GroupVersion, &workv1.ManifestWork{}, &workv1.ManifestWorkList{})
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
			keys := []string{}
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
				keys = append(keys, r.getConfigResourceKey(clientObj))
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
			name:           "hypershift serviceMonitor with no owner",
			inputObject:    createTestObject(mconfig.AcmEtcdServiceMonitorName, nil),
			expectedResult: true,
		},
		{
			name:           "acm etcd serviceMonitor name",
			inputObject:    createTestObject(mconfig.AcmEtcdServiceMonitorName, nil),
			expectedResult: true,
		},
		{
			name:           "acm apiServer serviceMonitor name",
			inputObject:    createTestObject(mconfig.AcmApiServerServiceMonitorName, nil),
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
