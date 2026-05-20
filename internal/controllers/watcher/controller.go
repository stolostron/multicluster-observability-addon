package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	rshandlers "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/handlers"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	localClusterNamespace = "local-cluster"
)

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, addonManager addonmanager.AddonManager, logger logr.Logger) error {
	l := logger.WithName("watcher")
	r := &WatcherReconciler{
		Client:       mgr.GetClient(),
		Log:          l.WithName("controller"),
		Scheme:       mgr.GetScheme(),
		addonManager: addonManager,
		Cache:        NewReferenceCache(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("watcher").
		Watches(&workv1.ManifestWork{}, r.enqueueForManifestWork(), builder.WithPredicates(manifestWorkPredicate)).
		Watches(&corev1.Secret{}, r.enqueueForConfigResource(), builder.OnlyMetadata).
		Watches(&corev1.ConfigMap{}, r.enqueueForConfigResource(), builder.OnlyMetadata).
		Watches(&corev1.ConfigMap{}, r.enqueueForAllManagedClusters(), builder.WithPredicates(predicate.Or(imagesConfigMapPredicate, rshandlers.RSConfigMapPredicate())), builder.OnlyMetadata).
		Watches(&hyperv1.HostedCluster{}, r.enqueueForLocalCluster(), hostedClusterPredicate).
		Watches(&prometheusv1.ServiceMonitor{}, r.enqueueForLocalCluster(), hypershiftServiceMonitorsPredicate(r.Log), builder.OnlyMetadata).
		Complete(r)
}

// WatcherReconciler triggers reconciliation in the AddonManager when something changes in an upstream resource
type WatcherReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	addonManager addonmanager.AddonManager
	Cache        *ReferenceCache
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.V(2).Info("reconciliation triggered", "request", req.String())
	r.addonManager.Trigger(req.Namespace, req.Name)

	return ctrl.Result{}, nil
}

// getConfigResourceKey generates a key for a given client.Object.
// The key format is "<Group>/<Kind>/<Namespace>/<Name>".
func (r *WatcherReconciler) getConfigResourceKey(obj client.Object) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		// GVK might be missing for objects from the informer.
		// Try to look it up from the scheme.
		gvks, _, err := r.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) > 0 {
			gvk = gvks[0]
		}
	}

	return fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName())
}

func (r *WatcherReconciler) enqueueForManifestWork() handler.EventHandler {
	return handler.Funcs{
		CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if mw, ok := e.Object.(*workv1.ManifestWork); ok {
				r.updateCache(mw)
			}
		},
		UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			oldMw, okOld := e.ObjectOld.(*workv1.ManifestWork)
			newMw, okNew := e.ObjectNew.(*workv1.ManifestWork)
			if !okOld || !okNew {
				return
			}
			if oldMw.Generation != newMw.Generation {
				r.updateCache(newMw)
			}
		},
		DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			r.Cache.Remove(e.Object.GetNamespace(), e.Object.GetName())
		},
	}
}

var manifestWorkPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return e.Object.GetLabels()[addoncfg.LabelOCMAddonName] == addoncfg.Name
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldMw, okOld := e.ObjectOld.(*workv1.ManifestWork)
		newMw, okNew := e.ObjectNew.(*workv1.ManifestWork)
		if !okOld || !okNew {
			return false
		}
		return oldMw.Generation != newMw.Generation && newMw.Labels[addoncfg.LabelOCMAddonName] == addoncfg.Name
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return e.Object.GetLabels()[addoncfg.LabelOCMAddonName] == addoncfg.Name
	},
}

func (r *WatcherReconciler) updateCache(mw *workv1.ManifestWork) {
	keys := map[string]struct{}{}

	for _, m := range mw.Spec.Workload.Manifests {
		minimalObj := &metav1.PartialObjectMetadata{}
		if err := json.Unmarshal(m.Raw, minimalObj); err != nil {
			r.Log.V(3).Error(err, "failed to unmarshal manifest to PartialObjectMetadata")
			continue
		}

		gvk := minimalObj.GroupVersionKind()

		switch gvk.GroupKind() {
		case corev1.SchemeGroupVersion.WithKind("Secret").GroupKind(), corev1.SchemeGroupVersion.WithKind("ConfigMap").GroupKind():
			originalResource, ok := minimalObj.GetAnnotations()[addoncfg.AnnotationOriginalResource]
			if !ok {
				r.Log.V(3).Info("configuration resource is missing the original-resource annotation, it is not added to the cache")
				continue
			}

			parts := strings.Split(originalResource, "/")
			if len(parts) != 2 {
				r.Log.V(3).Info("original-resource annotation is malformed, expected format 'namespace/name'", "annotation", originalResource)
				continue
			}

			namespace := parts[0]
			name := parts[1]

			if namespace == "" || name == "" {
				r.Log.V(3).Info("original-resource annotation contains empty namespace or name")
				continue
			}

			key := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Kind, namespace, name)
			keys[key] = struct{}{}
		}
	}

	r.Cache.Add(mw.Namespace, mw.Name, keys)
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

func (r *WatcherReconciler) enqueueForAllManagedClusters() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		r.Log.V(2).Info("Enqueue for all managed clusters", "gvk", obj.GetObjectKind().GroupVersionKind().String(), "name", obj.GetName(), "namespace", obj.GetNamespace())

		mwList := &workv1.ManifestWorkList{}
		if err := r.List(ctx, mwList, client.MatchingLabels{addoncfg.LabelOCMAddonName: addoncfg.Name}); err != nil {
			r.Log.Error(err, "error listing ManifestWorks to trigger reconciliation for all clusters")
			return nil
		}

		namespaces := make(map[string]struct{})
		for _, mw := range mwList.Items {
			namespaces[mw.Namespace] = struct{}{}
		}

		requests := make([]reconcile.Request, 0, len(namespaces))
		for ns := range namespaces {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      addoncfg.Name,
					Namespace: ns,
				},
			})
		}
		r.Log.V(2).Info("enqueuing reconciliation for all managed clusters", "count", len(requests))
		return requests
	})
}

func (r *WatcherReconciler) enqueueForConfigResource() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		rqs := []reconcile.Request{}
		namespaces := r.Cache.GetNamespaces(r.getConfigResourceKey(obj))
		if len(namespaces) == 0 {
			return []reconcile.Request{}
		}

		r.Log.V(2).Info("Enqueue for config resource event", "gvk", obj.GetObjectKind().GroupVersionKind().String(), "name", obj.GetName(), "namespace", obj.GetNamespace(), "clustersCount", len(namespaces))

		for _, ns := range namespaces {
			rqs = append(rqs,
				// Trigger a reconcile request for the addon in the ManifestWork namespace
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      addoncfg.Name,
						Namespace: ns,
					},
				},
			)
		}
		return rqs
	})
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

var imagesConfigMapPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		return e.ObjectNew.GetName() == mconfig.ImagesConfigMapObjKey.Name &&
			e.ObjectNew.GetNamespace() == mconfig.ImagesConfigMapObjKey.Namespace
	},
	CreateFunc: func(e event.CreateEvent) bool {
		return e.Object.GetName() == mconfig.ImagesConfigMapObjKey.Name &&
			e.Object.GetNamespace() == mconfig.ImagesConfigMapObjKey.Namespace
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return e.Object.GetName() == mconfig.ImagesConfigMapObjKey.Name &&
			e.Object.GetNamespace() == mconfig.ImagesConfigMapObjKey.Namespace
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

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

	return false
}
