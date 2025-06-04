package watcher

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	localClusterNamespace = "local-cluster"
)

var (
	managedClusterAddonKind                       = "ManagedClusterAddOn"
	errMessageGettingManagedClusterAddonResources = "Error getting managedclusteraddon resources in event handler"
	errMessageListingManifestWorkResources        = "Error listing manifestwork resources in event handler"
	errMessageDecodingManifestIntoObject          = "Error decoding manifest to client.object"
	errCastingObject                              = errors.New("object is not a client.Object")
)

var noReconcilePred = builder.WithPredicates(predicate.Funcs{
	UpdateFunc:  func(ue event.UpdateEvent) bool { return false },
	CreateFunc:  func(e event.CreateEvent) bool { return false },
	DeleteFunc:  func(e event.DeleteEvent) bool { return false },
	GenericFunc: func(e event.GenericEvent) bool { return false },
})

type WatcherManager struct {
	mgr    *ctrl.Manager
	logger logr.Logger
}

func NewWatcherManager(addonManager *addonmanager.AddonManager, scheme *runtime.Scheme, logger logr.Logger) (*WatcherManager, error) {
	l := logger.WithName("watcher")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme, Logger: l.WithName("manager")})
	if err != nil {
		return nil, fmt.Errorf("unable to start manager: %w", err)
	}

	if err = (&WatcherReconciler{
		Client:        mgr.GetClient(),
		Log:           l.WithName("controller"),
		Scheme:        mgr.GetScheme(),
		addonnManager: addonManager,
	}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create mcoa-watcher controller: %w", err)
	}

	if err = mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up health check: %w", err)
	}
	if err = mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up ready check: %w", err)
	}

	wm := WatcherManager{
		mgr:    &mgr,
		logger: l,
	}

	return &wm, nil
}

func (wm *WatcherManager) Start(ctx context.Context) {
	wm.logger.Info("Starting watcher manager")
	go func() {
		err := (*wm.mgr).Start(ctx)
		if err != nil {
			wm.logger.Error(err, "there was an error while running the reconciliation watcher")
		}
	}()
}

// WatcherReconciler triggers reconciliation in the AddonManager when something changes in an upstream resource
type WatcherReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	addonnManager *addonmanager.AddonManager
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.V(2).Info("reconciliation triggered", "request", req.String())
	(*r.addonnManager).Trigger(req.Namespace, req.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonv1alpha1.ManagedClusterAddOn{}, noReconcilePred, builder.OnlyMetadata).
		Watches(&corev1.Secret{}, r.enqueueForConfigResource(), builder.OnlyMetadata).
		Watches(&corev1.ConfigMap{}, r.enqueueForConfigResource(), builder.OnlyMetadata).
		Watches(&hyperv1.HostedCluster{}, r.enqueueForLocalCluster(), hostedClusterPredicate, builder.OnlyMetadata).
		Watches(&prometheusv1.ServiceMonitor{}, r.enqueueForLocalCluster(), hypershiftServiceMonitorsPredicate(r.Log), builder.OnlyMetadata).
		Complete(r)
}

func (r *WatcherReconciler) enqueueForLocalCluster() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		r.Log.V(2).Info("Enqueue for local cluster event", "gvk", obj.GetObjectKind().GroupVersionKind().String(), "name", obj.GetName(), "namespace", obj.GetNamespace())
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      addoncfg.Name,
					Namespace: localClusterNamespace,
				},
			},
		}
	})
}

func (r *WatcherReconciler) enqueueForConfigResource() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		key := client.ObjectKey{Name: addoncfg.Name, Namespace: obj.GetNamespace()}
		mcaddon := &metav1.PartialObjectMetadata{}
		mcaddon.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   addonv1alpha1.GroupVersion.Group,
			Version: addonv1alpha1.GroupVersion.Version,
			Kind:    managedClusterAddonKind,
		})
		if err := r.Get(ctx, key, mcaddon); err != nil {
			if apierrors.IsNotFound(err) {
				return r.getReconcileRequestsFromManifestWorks(ctx, obj)
			}
			r.Log.Error(err, errMessageGettingManagedClusterAddonResources)
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      mcaddon.Name,
					Namespace: mcaddon.Namespace,
				},
			},
		}
	})
}

// getReconcileRequestsFromManifestWorks gets reconcile.Request for resources referenced in ManifestWorks.
func (r *WatcherReconciler) getReconcileRequestsFromManifestWorks(ctx context.Context, newObj client.Object) []reconcile.Request {
	rqs := []reconcile.Request{}
	newObjKind := newObj.GetObjectKind()
	newObjKey := client.ObjectKeyFromObject(newObj)

	mws := &workv1.ManifestWorkList{}
	labelSelector := labels.SelectorFromSet(labels.Set{
		addoncfg.LabelOCMAddonName: addoncfg.Name,
	})
	if err := r.List(ctx, mws, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		r.Log.Error(err, errMessageListingManifestWorkResources)
		return nil
	}

	for _, mw := range mws.Items {
		for _, m := range mw.Spec.Workload.Manifests {
			obj, err := r.manifestToObject(m)
			if err != nil {
				r.Log.Error(err, errMessageDecodingManifestIntoObject)
				continue
			}
			objKind := obj.GetObjectKind()
			objKey := client.ObjectKeyFromObject(obj)
			if objKind.GroupVersionKind().String() == newObjKind.GroupVersionKind().String() && objKey == newObjKey {
				// Only trigger a reconcile request if the object has changed
				if !equality.Semantic.DeepEqual(newObj, obj) {
					rqs = append(rqs,
						// Trigger a reconcile request for the addon in the ManifestWork namespace
						reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      addoncfg.Name,
								Namespace: mw.Namespace,
							},
						},
					)
				}
			}
		}
	}
	return rqs
}

func (r *WatcherReconciler) manifestToObject(m workv1.Manifest) (client.Object, error) {
	decode := serializer.NewCodecFactory(r.Scheme).UniversalDeserializer().Decode

	obj, _, err := decode(m.Raw, nil, nil)
	if err != nil {
		return nil, err
	}

	clientObj, ok := obj.(client.Object)
	if !ok {
		return nil, errCastingObject
	}

	return clientObj, nil
}

var hostedClusterPredicate = builder.WithPredicates(predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		newHC := e.ObjectNew.(*hyperv1.HostedCluster)
		oldHC := e.ObjectOld.(*hyperv1.HostedCluster)
		return newHC.Spec.ClusterID != oldHC.Spec.ClusterID
	},
	CreateFunc:  func(e event.CreateEvent) bool { return true },
	DeleteFunc:  func(e event.DeleteEvent) bool { return true },
	GenericFunc: func(e event.GenericEvent) bool { return false },
})

func hypershiftServiceMonitorsPredicate(logger logr.Logger) builder.Predicates {
	return builder.WithPredicates(predicate.Funcs{
		UpdateFunc:  func(e event.UpdateEvent) bool { return isHypershiftServiceMonitor(logger, e.ObjectNew) },
		CreateFunc:  func(e event.CreateEvent) bool { return isHypershiftServiceMonitor(logger, e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return isHypershiftServiceMonitor(logger, e.Object) },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	})
}

// isHypershiftServiceMonitor returns true when the serviceMonitor is deployed by hypershift for etcd or the apiserver
// This is used for metrics to ensure our own serviceMonitor, based on the original one deployed by hypershift remains in sync.
func isHypershiftServiceMonitor(logger logr.Logger, obj client.Object) bool {
	if obj.GetName() == mconfig.HypershiftEtcdServiceMonitorName || obj.GetName() == mconfig.HypershiftApiServerServiceMonitorName {
		for _, owner := range obj.GetOwnerReferences() {
			gv, err := schema.ParseGroupVersion(owner.APIVersion)
			if err != nil {
				logger.V(1).Info("failed to parse groupVersion", "error", err.Error())
				continue
			}

			if gv.Group == hyperv1.GroupVersion.Group {
				return true
			}
		}
	}

	if obj.GetName() == mconfig.AcmEtcdServiceMonitorName || obj.GetName() == mconfig.AcmApiServerServiceMonitorName {
		return true
	}

	return false
}
