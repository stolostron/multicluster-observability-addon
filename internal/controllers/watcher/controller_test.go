package watcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestGetReconcileRequestsFromManifestWorks(t *testing.T) {
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
			expectedReconcileRequests: []reconcile.Request{},
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
			}

			cliObj := tc.object.(client.Object)
			rqs := r.getReconcileRequestsFromManifestWorks(context.TODO(), cliObj)
			assert.Equal(t, tc.expectedReconcileRequests, rqs)
		})
	}
}
