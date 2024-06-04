package watcher

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workapiv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

func NewWatcherManager(logger logr.Logger, scheme *runtime.Scheme, addonManager *addonmanager.AddonManager) (*WatcherManager, error) {
	l := logger.WithName("mcoa-watcher")

	ctrl.SetLogger(l)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("unable to start manager: %w", err)
	}

	if err = (&WatcherReconciler{
		Client:        mgr.GetClient(),
		Log:           l.WithName("controllers").WithName("mcoa-watcher"),
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
	(*r.addonnManager).Trigger(req.Namespace, req.Name)

	r.Log.V(2).Info("reconciliation triggered", "cluster", req.Namespace)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonapiv1alpha1.ManagedClusterAddOn{}, noReconcilePred).
		Watches(&corev1.Secret{}, r.enqueueForSecret(), builder.OnlyMetadata).
		Complete(r)
}

func (r *WatcherReconciler) enqueueForSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		key := client.ObjectKey{Name: addon.Name, Namespace: obj.GetNamespace()}
		mcaddon := &addonapiv1alpha1.ManagedClusterAddOn{}
		if err := r.Client.Get(ctx, key, mcaddon); err != nil {
			if apierrors.IsNotFound(err) {
				return r.getReconcileRequestsFromManifestWorks(ctx, obj)
			}
			r.Log.Error(err, "Error getting managedclusteraddon resources in event handler")
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

// getReconcileRequestsFromManifestWorks gets reconcile.Request for secrets referenced in ManifestWorks.
func (r *WatcherReconciler) getReconcileRequestsFromManifestWorks(ctx context.Context, obj client.Object) []reconcile.Request {
	rqs := []reconcile.Request{}
	mws := &workapiv1.ManifestWorkList{}
	if err := r.Client.List(ctx, mws, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"open-cluster-management.io/addon-name": "multicluster-observability-addon",
		}),
	}); err != nil {
		for _, mw := range mws.Items {
			for _, m := range mw.Spec.Workload.Manifests {
				if equality.Semantic.DeepEqual(m.Object, obj) {
					rqs = append(rqs,
						// Trigger a reconcile request for the addon in the ManifestWork namespace
						reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      addon.Name,
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
