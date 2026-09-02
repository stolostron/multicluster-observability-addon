package defaulthubstack

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	chandlers "github.com/stolostron/multicluster-observability-addon/internal/coo/handlers"
	cooresource "github.com/stolostron/multicluster-observability-addon/internal/coo/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var managedByPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj.GetLabels()[addoncfg.ManagedByK8sLabelKey] == cooresource.ManagedByLabelValue
})

var mcoaAODCPredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj.GetNamespace() == addoncfg.InstallNamespace && obj.GetName() == addoncfg.Name
})

// DefaultHubStackReconciler watches hub-only COO resources (dashboards, datasources,
// UIPlugin) and re-reconciles them without triggering ManifestWork updates
// for all managed clusters.
type DefaultHubStackReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func SetupWithManager(mgr ctrl.Manager, logger logr.Logger) error {
	r := &DefaultHubStackReconciler{
		Client: mgr.GetClient(),
		Log:    logger.WithName("coo-hub"),
		Scheme: mgr.GetScheme(),
	}

	enqueue := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: addoncfg.InstallNamespace,
				Name:      addoncfg.Name,
			},
		}}
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("default-hub-stack").
		For(&addonv1beta1.AddOnDeploymentConfig{}, builder.WithPredicates(mcoaAODCPredicate)).
		Watches(&persesv1.PersesDashboard{}, enqueue, builder.WithPredicates(managedByPredicate)).
		Watches(&persesv1.PersesDatasource{}, enqueue, builder.WithPredicates(managedByPredicate)).
		Watches(&uiplugin.UIPlugin{}, enqueue, builder.WithPredicates(managedByPredicate)).
		Complete(r)
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

	return ctrl.Result{}, nil
}
