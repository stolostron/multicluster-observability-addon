package defaulthubstack

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	clusterlifecycleconstants "github.com/stolostron/cluster-lifecycle-api/constants"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	chandlers "github.com/stolostron/multicluster-observability-addon/internal/coo/handlers"
	cooresource "github.com/stolostron/multicluster-observability-addon/internal/coo/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// HubCleanupFinalizer is held on the hub cluster's ManagedClusterAddOn for as long as
// HubResourceReconciler owns resources on the hub.
//
// Hub COO (Subscription, OperatorGroup, namespace), the Perses dashboards and datasources,
// the UIPlugin and the analytics namespace are deliberately kept out of the ManifestWork
// lifecycle: the chart renders them as deletion-orphan stubs and HubResourceReconciler owns
// them via server-side apply. Nothing deletes them when the Work Agent tears the hub
// ManifestWork down, so this controller has to do it itself.
//
// MCO's teardown (waitForMCOATeardown) deletes the ClusterManagementAddOn and then blocks
// until every MCOA ManagedClusterAddOn has disappeared before it undeploys our RBAC and
// Deployment. Hanging the cleanup off a finalizer on the hub ManagedClusterAddOn therefore
// guarantees we run while the addon manager is still alive and still has permissions.
const HubCleanupFinalizer = "observability.open-cluster-management.io/hub-resource-cleanup"

var managedByPredicate = predicate.Funcs{
	CreateFunc:  func(event.CreateEvent) bool { return false },
	UpdateFunc:  func(event.UpdateEvent) bool { return false },
	GenericFunc: func(event.GenericEvent) bool { return false },
	DeleteFunc: func(e event.DeleteEvent) bool {
		return e.Object.GetLabels()[addoncfg.ManagedByK8sLabelKey] == cooresource.ManagedByLabelValue
	},
}

var mcoaAODCPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj.GetNamespace() == addoncfg.InstallNamespace && obj.GetName() == addoncfg.Name
})

// mcoaAddonPredicate keeps the ManagedClusterAddOn watch to the MCOA addon, and to the
// events that can change whether the finalizer must be added or released. Status churn
// must not enqueue a hub reconcile once per managed cluster.
var mcoaAddonPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool { return e.Object.GetName() == addoncfg.Name },
	UpdateFunc: func(e event.UpdateEvent) bool {
		return e.ObjectNew.GetName() == addoncfg.Name &&
			e.ObjectOld.GetDeletionTimestamp().IsZero() != e.ObjectNew.GetDeletionTimestamp().IsZero()
	},
	DeleteFunc:  func(e event.DeleteEvent) bool { return e.Object.GetName() == addoncfg.Name },
	GenericFunc: func(event.GenericEvent) bool { return false },
}

// mcoaCMAPredicate reacts to the MCOA ClusterManagementAddOn entering deletion. That is the
// first thing MCO does when it tears MCOA down, and it is the only uninstall signal on a hub
// that is not itself a managed cluster (no ManagedClusterAddOn, so no finalizer to hang the
// cleanup off).
var mcoaCMAPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool { return e.Object.GetName() == addoncfg.Name },
	UpdateFunc: func(e event.UpdateEvent) bool {
		return e.ObjectNew.GetName() == addoncfg.Name &&
			e.ObjectOld.GetDeletionTimestamp().IsZero() != e.ObjectNew.GetDeletionTimestamp().IsZero()
	},
	DeleteFunc:  func(e event.DeleteEvent) bool { return e.Object.GetName() == addoncfg.Name },
	GenericFunc: func(event.GenericEvent) bool { return false },
}

// hubRequest is the single reconcile request this controller works off.
func hubRequest() []reconcile.Request {
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: addoncfg.InstallNamespace,
			Name:      addoncfg.Name,
		},
	}}
}

// enqueueHubRequest maps any watched object to the single hub reconcile request.
func enqueueHubRequest() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return hubRequest()
	})
}

// DefaultHubStackReconciler watches hub-only COO resources (dashboards, datasources,
// UIPlugin) and re-reconciles them without triggering ManifestWork updates
// for all managed clusters.
type DefaultHubStackReconciler struct {
	client.Client
	Log         logr.Logger
	Scheme      *runtime.Scheme
	ctrl        controller.Controller
	cache       cache.Cache
	mapper      meta.RESTMapper
	watchedGVKs map[schema.GroupVersionKind]bool
}

func SetupWithManager(mgr ctrl.Manager, logger logr.Logger) error {
	r := &DefaultHubStackReconciler{
		Client: mgr.GetClient(),
		Log:    logger.WithName("coo-hub"),
		Scheme: mgr.GetScheme(),
		cache:  mgr.GetCache(),
		mapper: mgr.GetRESTMapper(),
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		Named("default-hub-stack").
		For(&addonv1beta1.AddOnDeploymentConfig{}, builder.WithPredicates(mcoaAODCPredicate)).
		Watches(&addonv1beta1.ManagedClusterAddOn{}, r.enqueueForHubAddon(), builder.WithPredicates(mcoaAddonPredicate)).
		Watches(&addonv1beta1.ClusterManagementAddOn{}, enqueueHubRequest(), builder.WithPredicates(mcoaCMAPredicate)).
		Build(r)
	if err != nil {
		return err
	}
	r.ctrl = c

	return nil
}

// enqueueForHubAddon enqueues the single hub reconcile request for MCOA ManagedClusterAddOns
// that live in the hub cluster's namespace. Addons on regular spokes are ignored: they carry
// no hub resources.
func (r *DefaultHubStackReconciler) enqueueForHubAddon() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		cluster := &clusterv1.ManagedCluster{}
		if err := r.Get(ctx, client.ObjectKey{Name: obj.GetNamespace()}, cluster); err != nil {
			if !errors.IsNotFound(err) {
				r.Log.Error(err, "failed to get ManagedCluster for addon", "cluster", obj.GetNamespace())
			}
			return nil
		}
		if cluster.Labels[clusterlifecycleconstants.SelfManagedClusterLabelKey] != "true" {
			return nil
		}
		return hubRequest()
	})
}

func (r *DefaultHubStackReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	r.Log.V(2).Info("reconciliation triggered")

	hubAddon, err := r.getHubAddon(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// MCO deletes the ClusterManagementAddOn first and only then waits for the
	// ManagedClusterAddOns to disappear, so its absence is what marks a real uninstall.
	// Tear the hub resources down and stay down: the AddOnDeploymentConfig still carries
	// the old options at this point, and re-applying them would just leave the resources
	// orphaned once MCO removes our Deployment.
	uninstalling, err := r.addonIsUninstalling(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if uninstalling {
		return ctrl.Result{}, r.teardownHubResources(ctx, hubAddon)
	}

	if hubAddon != nil {
		// The addon can also be removed from the hub alone, by a placement change, while
		// MCOA stays enabled. HubResourceReconciler keeps managing the hub resources off
		// the AddOnDeploymentConfig in that case, so release the finalizer without
		// deleting anything.
		if !hubAddon.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, r.releaseFinalizer(ctx, hubAddon)
		}
		// Claim the addon before creating anything, so we never own hub resources that we
		// have no deletion hook for.
		if err := r.ensureFinalizer(ctx, hubAddon); err != nil {
			return ctrl.Result{}, err
		}
	}

	var opts addon.Options
	aodc := &addonv1beta1.AddOnDeploymentConfig{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: addoncfg.InstallNamespace, Name: addoncfg.Name}, aodc); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get AddOnDeploymentConfig: %w", err)
		}
		r.Log.Info("AddOnDeploymentConfig not found, reconciling with empty options to clean up hub resources")
	} else {
		var buildErr error
		opts, buildErr = addon.BuildOptions(aodc)
		if buildErr != nil {
			return ctrl.Result{}, fmt.Errorf("failed to build addon options: %w", buildErr)
		}
	}

	hasCardinalityRules := chandlers.HasCardinalityRules(ctx, r.Client)

	installCOO, err := chandlers.InstallOfCOOOnTheHubIsNeeded(ctx, r.Client, r.Log)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check COO installation: %w", err)
	}

	hubReconciler := &cooresource.HubResourceReconciler{
		Client: r.Client,
		Logger: r.Log,
		Opts:   opts,
	}
	if err := hubReconciler.Reconcile(ctx, hasCardinalityRules, installCOO); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile COO hub resources: %w", err)
	}

	r.registerDynamicWatches()

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// getHubAddon returns the MCOA ManagedClusterAddOn of the hub cluster, or nil when the hub
// is not a managed cluster or the addon is not installed on it.
func (r *DefaultHubStackReconciler) getHubAddon(ctx context.Context) (*addonv1beta1.ManagedClusterAddOn, error) {
	clusters := &clusterv1.ManagedClusterList{}
	opts := client.MatchingLabels{clusterlifecycleconstants.SelfManagedClusterLabelKey: "true"}
	if err := r.List(ctx, clusters, opts); err != nil {
		return nil, fmt.Errorf("failed to list hub ManagedClusters: %w", err)
	}

	for i := range clusters.Items {
		addonKey := client.ObjectKey{Namespace: clusters.Items[i].Name, Name: addoncfg.Name}
		mcao := &addonv1beta1.ManagedClusterAddOn{}
		if err := r.Get(ctx, addonKey, mcao); err != nil {
			if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			return nil, fmt.Errorf("failed to get ManagedClusterAddOn %s: %w", addonKey, err)
		}
		return mcao, nil
	}

	return nil, nil
}

// teardownHubResources removes everything HubResourceReconciler owns on the hub and then
// releases the finalizer, so the addon framework can finish deleting the ManagedClusterAddOn
// and MCO can proceed with the rest of the MCOA teardown.
func (r *DefaultHubStackReconciler) teardownHubResources(ctx context.Context, hubAddon *addonv1beta1.ManagedClusterAddOn) error {
	hubReconciler := &cooresource.HubResourceReconciler{
		Client: r.Client,
		Logger: r.Log,
		Opts:   addon.Options{},
	}
	if err := hubReconciler.Reconcile(ctx, false, false); err != nil {
		return fmt.Errorf("failed to clean up COO hub resources: %w", err)
	}
	r.Log.Info("cleaned up hub resources for MCOA uninstall")

	if hubAddon == nil {
		return nil
	}
	return r.releaseFinalizer(ctx, hubAddon)
}

// addonIsUninstalling reports whether the MCOA ClusterManagementAddOn is gone or on its way out.
func (r *DefaultHubStackReconciler) addonIsUninstalling(ctx context.Context) (bool, error) {
	cma := &addonv1beta1.ClusterManagementAddOn{}
	if err := r.Get(ctx, client.ObjectKey{Name: addoncfg.Name}, cma); err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("failed to get ClusterManagementAddOn %s: %w", addoncfg.Name, err)
	}
	return !cma.DeletionTimestamp.IsZero(), nil
}

func (r *DefaultHubStackReconciler) ensureFinalizer(ctx context.Context, hubAddon *addonv1beta1.ManagedClusterAddOn) error {
	if controllerutil.ContainsFinalizer(hubAddon, HubCleanupFinalizer) {
		return nil
	}

	if err := r.patchFinalizer(ctx, hubAddon, controllerutil.AddFinalizer); err != nil {
		return fmt.Errorf("failed to add %s finalizer to ManagedClusterAddOn %s/%s: %w",
			HubCleanupFinalizer, hubAddon.Namespace, hubAddon.Name, err)
	}

	r.Log.Info("added hub cleanup finalizer", "cluster", hubAddon.Namespace)
	return nil
}

func (r *DefaultHubStackReconciler) releaseFinalizer(ctx context.Context, hubAddon *addonv1beta1.ManagedClusterAddOn) error {
	if !controllerutil.ContainsFinalizer(hubAddon, HubCleanupFinalizer) {
		return nil
	}

	if err := r.patchFinalizer(ctx, hubAddon, controllerutil.RemoveFinalizer); err != nil {
		return fmt.Errorf("failed to remove %s finalizer from ManagedClusterAddOn %s/%s: %w",
			HubCleanupFinalizer, hubAddon.Namespace, hubAddon.Name, err)
	}

	r.Log.Info("released hub cleanup finalizer", "cluster", hubAddon.Namespace)
	return nil
}

// patchFinalizer re-reads the addon before mutating it: the addon framework holds its own
// finalizers on this object and updates it concurrently.
func (r *DefaultHubStackReconciler) patchFinalizer(
	ctx context.Context,
	hubAddon *addonv1beta1.ManagedClusterAddOn,
	mutate func(client.Object, string) bool,
) error {
	key := client.ObjectKeyFromObject(hubAddon)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &addonv1beta1.ManagedClusterAddOn{}
		if err := r.Get(ctx, key, latest); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !mutate(latest, HubCleanupFinalizer) {
			return nil
		}
		return r.Update(ctx, latest)
	})
}

func (r *DefaultHubStackReconciler) registerDynamicWatches() {
	if r.watchedGVKs == nil {
		r.watchedGVKs = make(map[schema.GroupVersionKind]bool)
	}

	enqueue := enqueueHubRequest()

	watchTargets := []struct {
		obj client.Object
		gvk schema.GroupVersionKind
	}{
		{&persesv1.PersesDashboard{}, schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDashboard"}},
		{&persesv1.PersesDatasource{}, schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDatasource"}},
		{&uiplugin.UIPlugin{}, schema.GroupVersionKind{Group: "observability.openshift.io", Version: "v1alpha1", Kind: "UIPlugin"}},
	}

	for _, w := range watchTargets {
		if r.watchedGVKs[w.gvk] {
			continue
		}

		if _, err := r.mapper.RESTMapping(w.gvk.GroupKind(), w.gvk.Version); err != nil {
			r.Log.V(2).Info("CRD not yet available, will retry on next reconcile", "kind", w.gvk.Kind)
			continue
		}

		if err := r.ctrl.Watch(source.Kind(r.cache, w.obj, enqueue, managedByPredicate)); err != nil {
			r.Log.Error(err, "failed to register dynamic watch", "kind", w.gvk.Kind)
			continue
		}

		r.watchedGVKs[w.gvk] = true
		r.Log.Info("registered dynamic watch", "kind", w.gvk.Kind)
	}
}
