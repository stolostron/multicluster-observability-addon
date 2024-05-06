package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	noReconcilePred = builder.WithPredicates(predicate.Funcs{
		UpdateFunc:  func(ue event.UpdateEvent) bool { return false },
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	})
)

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
		Complete(r)
}

func (r *WatcherReconciler) enqueueForClusterSpecificResource() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		key := client.ObjectKey{Name: "multicluster-observability-addon", Namespace: obj.GetNamespace()}
		addon := &addonapiv1alpha1.ManagedClusterAddOn{}
		if err := r.Client.Get(ctx, key, addon); err != nil && !apierrors.IsNotFound(err){
			r.Log.Error(err, "Error getting ManagedClusterAddon resources in event handler")
			return nil
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      addon.Name,
					Namespace: addon.Namespace,
				},
			}}
	})
}
