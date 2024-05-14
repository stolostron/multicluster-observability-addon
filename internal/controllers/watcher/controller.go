package watcher

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/handlers"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
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

func NewWatcherManager(logger logr.Logger, scheme *runtime.Scheme) (*WatcherManager, error) {
	l := logger.WithName("mcoa-watcher")

	ctrl.SetLogger(l)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("unable to start manager: %w", err)
	}

	if err = (&WatcherReconciler{
		Client: mgr.GetClient(),
		Log:    l.WithName("controllers").WithName("mcoa-watcher"),
		Scheme: mgr.GetScheme(),
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

// WatcherReconciler reconciles the ManagedClusterAddon to annotate the ManiestWorks resource
type WatcherReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if err := handlers.UpdateAnnotationOnManifestWorks(ctx, r.Log, req, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonapiv1alpha1.ManagedClusterAddOn{}, noReconcilePred).
		Watches(&corev1.ConfigMap{}, r.enqueueForClusterSpecificResource(), builder.OnlyMetadata).
		Watches(&corev1.Secret{}, r.enqueueForClusterSpecificResource(), builder.OnlyMetadata).
		Watches(&loggingv1.ClusterLogForwarder{}, r.enqueueForClusterWideResource(), builder.OnlyMetadata).
		Complete(r)
}

func (r *WatcherReconciler) enqueueForClusterSpecificResource() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		key := client.ObjectKey{Name: "multicluster-observability-addon", Namespace: obj.GetNamespace()}
		addon := &addonapiv1alpha1.ManagedClusterAddOn{}
		if err := r.Client.Get(ctx, key, addon); err != nil && !apierrors.IsNotFound(err) {
			r.Log.Error(err, "Error getting managedclusteraddon resources in event handler")
			return nil
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      addon.Name,
					Namespace: addon.Namespace,
				},
			},
		}
	})
}

func (r *WatcherReconciler) enqueueForClusterWideResource() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		addonList := &addonapiv1alpha1.ManagedClusterAddOnList{}
		if err := r.Client.List(ctx, addonList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				"open-cluster-management.io/addon-name": "multicluster-observability-addon",
			}),
		}); err != nil {
			r.Log.Error(err, "Error listing managedclusteraddon resources in event handler")
			return nil
		}

		clustersetValue, clustersetExists := obj.GetAnnotations()["cluster.open-cluster-management.io/clusterset"]
		var clustersInClusterSet map[string]struct{}
		if clustersetExists {
			clusterList := &clusterv1.ManagedClusterList{}
			if err := r.Client.List(ctx, clusterList, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labels.Set{
					"cluster.open-cluster-management.io/clusterset": clustersetValue,
				}),
			}); err != nil {
				r.Log.Error(err, "Error listing managedcluster resources in event handler")
				return nil
			}
			clustersInClusterSet = make(map[string]struct{}, len(clusterList.Items))
			for _, cluster := range clusterList.Items {
				clustersInClusterSet[cluster.Name] = struct{}{}
			}
		}

		var requests []reconcile.Request
		for _, addon := range addonList.Items {
			_, installed := clustersInClusterSet[addon.Namespace]
			if clustersetExists && !installed {
				continue
			}

			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: addon.Namespace,
					Name:      addon.Name,
				},
			})
		}
		return requests
	})
}
