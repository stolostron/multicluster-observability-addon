package defaulthubstack

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	chandlers "github.com/stolostron/multicluster-observability-addon/internal/coo/handlers"
	cooresource "github.com/stolostron/multicluster-observability-addon/internal/coo/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var managedByPredicate = predicate.Funcs{
	CreateFunc:  func(event.CreateEvent) bool { return false },
	UpdateFunc:  func(event.UpdateEvent) bool { return false },
	GenericFunc: func(event.GenericEvent) bool { return false },
	DeleteFunc: func(e event.DeleteEvent) bool {
		return e.Object.GetLabels()[addoncfg.ManagedByK8sLabelKey] == cooresource.ManagedByLabelValue
	},
}

var mcoaAODCPredicate = predicate.And(
	predicate.GenerationChangedPredicate{},
	predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == addoncfg.InstallNamespace && obj.GetName() == addoncfg.Name
	}),
)

// DefaultHubStackReconciler watches hub-only COO resources (dashboards, datasources,
// UIPlugin) and re-reconciles them without triggering ManifestWork updates
// for all managed clusters.
type DefaultHubStackReconciler struct {
	client.Client
	APIReader   client.Reader
	Log         logr.Logger
	Scheme      *runtime.Scheme
	ctrl        controller.Controller
	cache       cache.Cache
	mapper      meta.RESTMapper
	mu          sync.RWMutex
	watchedGVKs map[schema.GroupVersionKind]bool
}

func (r *DefaultHubStackReconciler) enqueueAODC() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: addoncfg.InstallNamespace,
				Name:      addoncfg.Name,
			},
		}}
	})
}

func SetupWithManager(mgr ctrl.Manager, logger logr.Logger) error {
	r := &DefaultHubStackReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Log:       logger.WithName("coo-hub"),
		Scheme:    mgr.GetScheme(),
		cache:     mgr.GetCache(),
		mapper:    mgr.GetRESTMapper(),
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		Named("default-hub-stack").
		For(&addonv1beta1.AddOnDeploymentConfig{}, builder.WithPredicates(mcoaAODCPredicate)).
		Watches(&corev1.ConfigMap{}, r.enqueueAODC(), builder.WithPredicates(chandlers.CardinalityRulesConfigMapPredicate()), builder.OnlyMetadata).
		Watches(&operatorsv1alpha1.Subscription{}, r.enqueueAODC(), builder.WithPredicates(chandlers.CooSubscriptionPredicate())).
		Build(r)
	if err != nil {
		return err
	}
	r.ctrl = c

	return nil
}

func (r *DefaultHubStackReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	r.Log.V(2).Info("reconciliation triggered")

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

	reader := client.Reader(r.Client)
	if r.APIReader != nil {
		reader = r.APIReader
	}
	hasCardinalityRules := chandlers.HasCardinalityRules(ctx, reader)

	canManageCOO, err := chandlers.CanManageCOOOnTheHub(ctx, r.Client, r.Log)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check COO installation: %w", err)
	}

	hubReconciler := &cooresource.HubResourceReconciler{
		Client: r.Client,
		Logger: r.Log,
		Opts:   opts,
	}
	if err := hubReconciler.Reconcile(ctx, hasCardinalityRules, canManageCOO); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile COO hub resources: %w", err)
	}

	features := cooresource.EvaluateHubStackFeatures(opts)
	allWatchesBound := r.registerDynamicWatches(features)
	if !allWatchesBound {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *DefaultHubStackReconciler) registerDynamicWatches(features cooresource.HubStackFeatures) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.watchedGVKs == nil {
		r.watchedGVKs = make(map[schema.GroupVersionKind]bool)
	}

	enqueue := r.enqueueAODC()

	watchTargets := []struct {
		obj      client.Object
		gvk      schema.GroupVersionKind
		pred     predicate.Predicate
		required bool
	}{
		{
			obj:      &persesv1.PersesDashboard{},
			gvk:      schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDashboard"},
			pred:     managedByPredicate,
			required: features.PersesEnabled,
		},
		{
			obj:      &persesv1.PersesDatasource{},
			gvk:      schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha1", Kind: "PersesDatasource"},
			pred:     managedByPredicate,
			required: features.PersesEnabled,
		},
		{
			obj:      &uiplugin.UIPlugin{},
			gvk:      schema.GroupVersionKind{Group: "observability.openshift.io", Version: "v1alpha1", Kind: "UIPlugin"},
			pred:     managedByPredicate,
			required: features.PersesEnabled || features.IncidentDetectionEnabled,
		},
	}

	allBound := true
	for _, w := range watchTargets {
		if r.watchedGVKs[w.gvk] {
			continue
		}

		if r.mapper == nil {
			if w.required {
				allBound = false
			}
			continue
		}

		if _, err := r.mapper.RESTMapping(w.gvk.GroupKind(), w.gvk.Version); err != nil {
			r.Log.V(2).Info("CRD not yet available", "kind", w.gvk.Kind)
			if w.required {
				allBound = false
			}
			continue
		}

		if r.ctrl == nil || r.cache == nil {
			if w.required {
				allBound = false
			}
			continue
		}

		if err := r.ctrl.Watch(source.Kind(r.cache, w.obj, enqueue, w.pred)); err != nil {
			r.Log.Error(err, "failed to register dynamic watch", "kind", w.gvk.Kind)
			if w.required {
				allBound = false
			}
			continue
		}

		r.watchedGVKs[w.gvk] = true
		r.Log.Info("registered dynamic watch", "kind", w.gvk.Kind)
	}

	return allBound
}
