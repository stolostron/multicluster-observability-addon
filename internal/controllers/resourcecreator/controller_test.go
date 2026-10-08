package resourcecreator

import (
	"context"
	"errors"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
			oldObj: &addonv1beta1.ClusterManagementAddOn{
				Spec: addonv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1beta1.InstallStrategy{
						Placements: []addonv1beta1.PlacementStrategy{
							{PlacementRef: addonv1beta1.PlacementRef{Name: "p1"}},
						},
					},
				},
			},
			newObj: &addonv1beta1.ClusterManagementAddOn{
				Spec: addonv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1beta1.InstallStrategy{
						Placements: []addonv1beta1.PlacementStrategy{
							{PlacementRef: addonv1beta1.PlacementRef{Name: "p1"}},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "placements changed",
			oldObj: &addonv1beta1.ClusterManagementAddOn{
				Spec: addonv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1beta1.InstallStrategy{
						Placements: []addonv1beta1.PlacementStrategy{
							{PlacementRef: addonv1beta1.PlacementRef{Name: "p1"}},
						},
					},
				},
			},
			newObj: &addonv1beta1.ClusterManagementAddOn{
				Spec: addonv1beta1.ClusterManagementAddOnSpec{
					InstallStrategy: addonv1beta1.InstallStrategy{
						Placements: []addonv1beta1.PlacementStrategy{
							{PlacementRef: addonv1beta1.PlacementRef{Name: "p2"}},
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
	_ = addonv1beta1.Install(scheme)
	_ = cooprometheusv1alpha1.AddToScheme(scheme)
	_ = prometheusv1.AddToScheme(scheme)

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

		t.Run("user-defined resource with part-of label", func(t *testing.T) {
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						addoncfg.PartOfK8sLabelKey: addoncfg.Name,
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

		t.Run("user-defined PrometheusAgent with part-of label", func(t *testing.T) {
			agent := &cooprometheusv1alpha1.PrometheusAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-agent",
					Namespace: addoncfg.InstallNamespace,
					Labels: map[string]string{
						addoncfg.PartOfK8sLabelKey: addoncfg.Name,
					},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: agent}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("unowned resource without label", func(t *testing.T) {
			obj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: obj}, q)
			assert.Equal(t, 0, q.Len())
		})

		t.Run("nil object", func(t *testing.T) {
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: nil}, q)
			assert.Equal(t, 0, q.Len())
		})

		t.Run("update event: label removed triggers reconciliation", func(t *testing.T) {
			oldObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{addoncfg.PartOfK8sLabelKey: addoncfg.Name},
				},
			}
			newObj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Update(context.Background(), event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("update event: label added triggers reconciliation", func(t *testing.T) {
			oldObj := &corev1.ConfigMap{}
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{addoncfg.PartOfK8sLabelKey: addoncfg.Name},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Update(context.Background(), event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("update event: unlabeled resource does not trigger", func(t *testing.T) {
			oldObj := &corev1.ConfigMap{}
			newObj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Update(context.Background(), event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}, q)
			assert.Equal(t, 0, q.Len())
		})

		t.Run("delete event: labeled resource triggers reconciliation", func(t *testing.T) {
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{addoncfg.PartOfK8sLabelKey: addoncfg.Name},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Delete(context.Background(), event.DeleteEvent{Object: obj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("delete event: unlabeled resource does not trigger", func(t *testing.T) {
			obj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Delete(context.Background(), event.DeleteEvent{Object: obj}, q)
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

		t.Run("user-defined resource with part-of label", func(t *testing.T) {
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						addoncfg.PartOfK8sLabelKey: addoncfg.Name,
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

		t.Run("user-defined ScrapeConfig with part-of label", func(t *testing.T) {
			sc := &cooprometheusv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-sc",
					Namespace: addoncfg.InstallNamespace,
					Labels: map[string]string{
						addoncfg.PartOfK8sLabelKey: addoncfg.Name,
					},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: sc}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("user-defined PrometheusRule with part-of label", func(t *testing.T) {
			rule := &prometheusv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-rule",
					Namespace: addoncfg.InstallNamespace,
					Labels: map[string]string{
						addoncfg.PartOfK8sLabelKey: addoncfg.Name,
					},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: rule}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("not controlled by MCO without label", func(t *testing.T) {
			obj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: obj}, q)
			assert.Equal(t, 0, q.Len())
		})

		t.Run("nil object", func(t *testing.T) {
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Create(context.Background(), event.CreateEvent{Object: nil}, q)
			assert.Equal(t, 0, q.Len())
		})

		t.Run("update event: label removed triggers reconciliation", func(t *testing.T) {
			oldObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{addoncfg.PartOfK8sLabelKey: addoncfg.Name},
				},
			}
			newObj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Update(context.Background(), event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("update event: label added triggers reconciliation", func(t *testing.T) {
			oldObj := &corev1.ConfigMap{}
			newObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{addoncfg.PartOfK8sLabelKey: addoncfg.Name},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Update(context.Background(), event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("update event: unlabeled resource does not trigger", func(t *testing.T) {
			oldObj := &corev1.ConfigMap{}
			newObj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Update(context.Background(), event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}, q)
			assert.Equal(t, 0, q.Len())
		})

		t.Run("delete event: labeled resource triggers reconciliation", func(t *testing.T) {
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{addoncfg.PartOfK8sLabelKey: addoncfg.Name},
				},
			}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Delete(context.Background(), event.DeleteEvent{Object: obj}, q)

			var actual []reconcile.Request
			for q.Len() > 0 {
				item, _ := q.Get()
				actual = append(actual, item)
				q.Done(item)
			}
			assert.Equal(t, mcoaAODCRequest(), actual)
		})

		t.Run("delete event: unlabeled resource does not trigger", func(t *testing.T) {
			obj := &corev1.ConfigMap{}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			h.Delete(context.Background(), event.DeleteEvent{Object: obj}, q)
			assert.Equal(t, 0, q.Len())
		})
	})
}

func TestHasCRD(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = lokiv1.AddToScheme(scheme)
	_ = loggingv1.AddToScheme(scheme)

	// Mapper that only knows about ClusterLogForwarder: the Loki Operator is not
	// installed on this hub, so lokistacks is not served.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{loggingv1.GroupVersion})
	mapper.Add(clusterLogForwarderGVK, meta.RESTScopeNamespace)

	reconciler := &ResourceCreatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(mapper).Build(),
		Scheme: scheme,
	}

	assert.True(t, reconciler.hasCRD(clusterLogForwarderGVK))
	assert.False(t, reconciler.hasCRD(lokiStackGVK))
}

// errKindNotServed stands in for what the API server returns for a kind whose
// CRD is not installed.
var errKindNotServed = errors.New("the server could not find the requested resource")

// TestReconcileWithoutLokiStackCRD covers a hub where neither the Loki Operator
// nor the Cluster Logging Operator is installed. Whether that is fatal depends
// entirely on whether the user asked for the default logging stack.
func TestReconcileWithoutLokiStackCRD(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = addonv1beta1.Install(scheme)
	_ = clusterv1.Install(scheme)
	_ = cooprometheusv1alpha1.AddToScheme(scheme)
	_ = prometheusv1.AddToScheme(scheme)
	_ = lokiv1.AddToScheme(scheme)
	_ = loggingv1.AddToScheme(scheme)
	// Enabling the default stack also drives the mcoa-gateway certificates.
	_ = routev1.AddToScheme(scheme)
	_ = certmanagerv1.AddToScheme(scheme)

	newReconciler := func(defaultStack bool) *ResourceCreatorReconciler {
		aodc := &addonv1beta1.AddOnDeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name, Namespace: addoncfg.InstallNamespace},
		}
		if defaultStack {
			aodc.Spec.CustomizedVariables = []addonv1beta1.CustomizedVariable{
				{Name: addon.KeyPlatformLogsDefault, Value: "true"},
			}
		}
		cmao := &addonv1beta1.ClusterManagementAddOn{
			ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name},
			Spec: addonv1beta1.ClusterManagementAddOnSpec{
				InstallStrategy: addonv1beta1.InstallStrategy{
					Type:       "Placements",
					Placements: []addonv1beta1.PlacementStrategy{{PlacementRef: addoncfg.GlobalPlacementRef}},
				},
			},
		}

		// Empty REST mapper: the API server serves neither lokistacks nor
		// clusterlogforwarders. The interceptor makes any call that slips
		// through anyway fail the way a real API server would.
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithRESTMapper(meta.NewDefaultRESTMapper([]schema.GroupVersion{})).
			WithObjects(aodc, cmao).
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					switch list.(type) {
					case *lokiv1.LokiStackList, *loggingv1.ClusterLogForwarderList:
						return errKindNotServed
					}
					return c.List(ctx, list, opts...)
				},
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					switch obj.(type) {
					case *lokiv1.LokiStack, *loggingv1.ClusterLogForwarder:
						return errKindNotServed
					}
					return c.Get(ctx, key, obj, opts...)
				},
			}).
			Build()

		return &ResourceCreatorReconciler{Client: fakeClient, Log: logr.Discard(), Scheme: scheme}
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: addoncfg.Name, Namespace: addoncfg.InstallNamespace},
	}

	t.Run("logging disabled: reconciles cleanly", func(t *testing.T) {
		_, err := newReconciler(false).Reconcile(context.Background(), request)
		assert.NoError(t, err, "a hub without the Loki Operator must still reconcile metrics and right-sizing")
	})

	t.Run("logging enabled: reports the missing CRD", func(t *testing.T) {
		_, err := newReconciler(true).Reconcile(context.Background(), request)
		require.ErrorIs(t, err, errLokiStackCRDMissing)
	})
}
