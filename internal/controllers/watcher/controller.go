package watcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
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

type ReferenceCache struct {
	sync.RWMutex
	// configToMWNs maps a Configuration Resource Key (Group/Kind/Namespace/Name)
	// to a set of ManifestWork Namespaces that reference it.
	configToMWNs map[string]map[string]struct{}
	// mwKeyToConfigs maps a ManifestWork Key (Namespace/Name) to the set of
	// Configuration Resource Keys it references. Used for efficient cleanup.
	mwKeyToConfigs map[string]map[string]struct{}
}

func NewReferenceCache() *ReferenceCache {
	return &ReferenceCache{
		configToMWNs:   make(map[string]map[string]struct{}),
		mwKeyToConfigs: make(map[string]map[string]struct{}),
	}
}

func (c *ReferenceCache) Add(mwNamespace, mwName string, configKeys []string) {
	c.Lock()
	defer c.Unlock()

	mwKey := fmt.Sprintf("%s/%s", mwNamespace, mwName)
	newConfigs := make(map[string]struct{})
	for _, k := range configKeys {
		newConfigs[k] = struct{}{}
	}

	// Remove old references if any (handle update)
	if oldConfigs, exists := c.mwKeyToConfigs[mwKey]; exists {
		for configKey := range oldConfigs {
			if _, keep := newConfigs[configKey]; !keep {
				c.removeRef(mwNamespace, configKey)
			}
		}
	}

	// Add new references
	c.mwKeyToConfigs[mwKey] = newConfigs
	for _, configKey := range configKeys {
		if _, ok := c.configToMWNs[configKey]; !ok {
			c.configToMWNs[configKey] = make(map[string]struct{})
		}
		c.configToMWNs[configKey][mwNamespace] = struct{}{}
	}
}

func (c *ReferenceCache) Remove(mwNamespace, mwName string) {
	c.Lock()
	defer c.Unlock()

	mwKey := fmt.Sprintf("%s/%s", mwNamespace, mwName)
	if oldConfigs, exists := c.mwKeyToConfigs[mwKey]; exists {
		for configKey := range oldConfigs {
			c.removeRef(mwNamespace, configKey)
		}
		delete(c.mwKeyToConfigs, mwKey)
	}
}

func (c *ReferenceCache) removeRef(mwNamespace, configKey string) {
	if nss, ok := c.configToMWNs[configKey]; ok {
		delete(nss, mwNamespace)
		if len(nss) == 0 {
			delete(c.configToMWNs, configKey)
		}
	}
}

func (c *ReferenceCache) GetNamespaces(configKey string) []string {
	c.RLock()
	defer c.RUnlock()

	var result []string
	if nss, ok := c.configToMWNs[configKey]; ok {
		for ns := range nss {
			result = append(result, ns)
		}
	}
	return result
}

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
		Cache:         NewReferenceCache(),
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
	Cache         *ReferenceCache
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
		Watches(&workv1.ManifestWork{}, r.enqueueForManifestWork(), builder.WithPredicates(manifestWorkPredicate)).
		Watches(&corev1.Secret{}, r.enqueueForConfigResource(), builder.OnlyMetadata).
		Watches(&corev1.ConfigMap{}, r.enqueueForConfigResource(), builder.OnlyMetadata).
		Watches(&corev1.ConfigMap{}, r.enqueueForAllManagedClusters(), imagesConfigMapPredicate, builder.OnlyMetadata).
		Watches(&hyperv1.HostedCluster{}, r.enqueueForLocalCluster(), hostedClusterPredicate).
		Watches(&prometheusv1.ServiceMonitor{}, r.enqueueForLocalCluster(), hypershiftServiceMonitorsPredicate(r.Log), builder.OnlyMetadata).
		Complete(r)
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

// enqueueForManifestWork updates the cache when a ManifestWork is created/updated/deleted
func (r *WatcherReconciler) enqueueForManifestWork() handler.EventHandler {
	return handler.Funcs{
		CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			r.updateCache(e.Object)
		},
		UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			r.updateCache(e.ObjectNew)
		},
		DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			r.Cache.Remove(e.Object.GetNamespace(), e.Object.GetName())
		},
	}
}

func (r *WatcherReconciler) updateCache(obj client.Object) {
	mw, ok := obj.(*workv1.ManifestWork)
	if !ok {
		return
	}

	keys := []string{}
	decode := serializer.NewCodecFactory(r.Scheme).UniversalDeserializer().Decode

	for _, m := range mw.Spec.Workload.Manifests {
		obj, gvk, err := decode(m.Raw, nil, nil)
		if err != nil {
			continue
		}

		clientObj, ok := obj.(client.Object)
		if !ok {
			continue
		}

		switch clientObj.(type) {
		case *corev1.Secret, *corev1.ConfigMap:
			key := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Kind, clientObj.GetNamespace(), clientObj.GetName())
			keys = append(keys, key)
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

var manifestWorkPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj.GetLabels()[addoncfg.LabelOCMAddonName] == addoncfg.Name
})


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

var imagesConfigMapPredicate = builder.WithPredicates(predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectNew.GetName() == mconfig.ImagesConfigMapObjKey.Name && e.ObjectNew.GetNamespace() == mconfig.ImagesConfigMapObjKey.Namespace {
			oldCM, okOld := e.ObjectOld.(*corev1.ConfigMap)
			newCM, okNew := e.ObjectNew.(*corev1.ConfigMap)
			if !okOld || !okNew {
				return false
			}
			return !equality.Semantic.DeepEqual(oldCM.Data, newCM.Data)
		}
		return false
	},
	CreateFunc: func(e event.CreateEvent) bool {
		return e.Object.GetName() == mconfig.ImagesConfigMapObjKey.Name && e.Object.GetNamespace() == mconfig.ImagesConfigMapObjKey.Namespace
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return e.Object.GetName() == mconfig.ImagesConfigMapObjKey.Name && e.Object.GetNamespace() == mconfig.ImagesConfigMapObjKey.Namespace
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
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
