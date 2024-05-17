package watcher

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
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

// WatcherReconciler reconciles the ManagedClusterAddon to annotate the ManiestWorks resource
type WatcherReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	addonnManager *addonmanager.AddonManager
}

var (
	noNameErr      = errors.New("no name for reconciliation request")
	noNamespaceErr = errors.New("no namespace for reconciliation request")
)

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name == "" {
		return ctrl.Result{}, noNameErr
	}
	if req.Namespace == "" {
		return ctrl.Result{}, noNamespaceErr
	}
	(*r.addonnManager).Trigger(req.Namespace, req.Name)

	r.Log.V(2).Info("reconciliation triggered", "cluster", req.Namespace)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonapiv1alpha1.ManagedClusterAddOn{}, noReconcilePred).
		Watches(&corev1.ConfigMap{}, r.enqueueForClusterSpecificResource(), builder.OnlyMetadata).
		Watches(&corev1.Secret{}, r.enqueueForClusterSpecificResource(), builder.OnlyMetadata).
		Watches(&loggingv1.ClusterLogForwarder{}, r.enqueueForClusterWideResource(), builder.OnlyMetadata).
		Watches(&otelv1alpha1.OpenTelemetryCollector{}, r.enqueueForClusterWideResource(), builder.OnlyMetadata).
		Complete(r)
}

func (r *WatcherReconciler) enqueueForClusterSpecificResource() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		key := client.ObjectKey{Name: "multicluster-observability-addon", Namespace: obj.GetNamespace()}
		addon := &addonapiv1alpha1.ManagedClusterAddOn{}
		if err := r.Client.Get(ctx, key, addon); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
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

		if len(addonList.Items) == 0 {
			r.Log.V(2).Info("no managedclusteraddon found")
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
