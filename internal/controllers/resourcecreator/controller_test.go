package resourcecreator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestValidateAODC(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		objName   string
		expected  bool
	}{
		{
			name:      "valid namespace and name",
			namespace: addoncfg.InstallNamespace,
			objName:   addoncfg.Name,
			expected:  true,
		},
		{
			name:      "invalid namespace",
			namespace: "wrong-ns",
			objName:   addoncfg.Name,
			expected:  false,
		},
		{
			name:      "invalid name",
			namespace: addoncfg.InstallNamespace,
			objName:   "wrong-name",
			expected:  false,
		},
		{
			name:      "invalid both",
			namespace: "wrong-ns",
			objName:   "wrong-name",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateAODC(tt.namespace, tt.objName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCMAOPlacementsChanged(t *testing.T) {
	tests := []struct {
		name     string
		oldObj   client.Object
		newObj   client.Object
		expected bool
	}{
		{
			name: "placements unchanged",
			oldObj: &addonv1alpha1.ClusterManagementAddOn{
				Spec: addonv1alpha1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1alpha1.InstallStrategy{
						Placements: []addonv1alpha1.PlacementStrategy{
							{PlacementRef: addonv1alpha1.PlacementRef{Name: "p1"}},
						},
					},
				},
			},
			newObj: &addonv1alpha1.ClusterManagementAddOn{
				Spec: addonv1alpha1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1alpha1.InstallStrategy{
						Placements: []addonv1alpha1.PlacementStrategy{
							{PlacementRef: addonv1alpha1.PlacementRef{Name: "p1"}},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "placements changed",
			oldObj: &addonv1alpha1.ClusterManagementAddOn{
				Spec: addonv1alpha1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1alpha1.InstallStrategy{
						Placements: []addonv1alpha1.PlacementStrategy{
							{PlacementRef: addonv1alpha1.PlacementRef{Name: "p1"}},
						},
					},
				},
			},
			newObj: &addonv1alpha1.ClusterManagementAddOn{
				Spec: addonv1alpha1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1alpha1.InstallStrategy{
						Placements: []addonv1alpha1.PlacementStrategy{
							{PlacementRef: addonv1alpha1.PlacementRef{Name: "p2"}},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmaoPlacementsChanged(tt.oldObj, tt.newObj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMCOAAODCRequest(t *testing.T) {
	expected := []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      addoncfg.Name,
				Namespace: addoncfg.InstallNamespace,
			},
		},
	}
	assert.Equal(t, expected, mcoaAODCRequest())
}

func TestEnqueueFunctions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = addonv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &ResourceCreatorReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	t.Run("enqueueAODC", func(t *testing.T) {
		h := reconciler.enqueueAODC()
		q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		h.Create(context.Background(), event.CreateEvent{Object: &corev1.ConfigMap{}}, q)

		var actual []reconcile.Request
		for q.Len() > 0 {
			item, _ := q.Get()
			actual = append(actual, item)
			q.Done(item)
		}
		assert.Equal(t, mcoaAODCRequest(), actual)
	})

	t.Run("enqueueForMCOAOwnedResources", func(t *testing.T) {
		h := reconciler.enqueueForMCOAOwnedResources()

		t.Run("owned resource", func(t *testing.T) {
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "addon.open-cluster-management.io/v1alpha1",
							Kind:       "ClusterManagementAddOn",
							Name:       addoncfg.Name,
						},
					},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: obj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("unowned resource", func(t *testing.T) {
			obj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: obj}, q)
			assert.Equal(t, 0, q.Len())
		})
	})

	t.Run("enqueueForMCOControlledResources", func(t *testing.T) {
		h := reconciler.enqueueForMCOControlledResources()

		t.Run("controlled by MCO", func(t *testing.T) {
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "observability.open-cluster-management.io/v1beta1",
							Kind:       "MultiClusterObservability",
							Controller: func() *bool { b := true; return &b }(),
						},
					},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: obj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("not controlled by MCO", func(t *testing.T) {
			obj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: obj}, q)
			assert.Equal(t, 0, q.Len())
		})
	})
}
