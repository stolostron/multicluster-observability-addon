package resourcecreator

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	rshandlers "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/handlers"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	mresources "github.com/stolostron/multicluster-observability-addon/internal/metrics/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func validateAODC(namespace, name string) bool {
	if namespace != addoncfg.InstallNamespace || name != addoncfg.Name {
		return false
	}
	return true
}

var mcoaAODCPredicate = builder.WithPredicates(predicate.Funcs{
	CreateFunc:  func(e event.CreateEvent) bool { return validateAODC(e.Object.GetNamespace(), e.Object.GetName()) },
	UpdateFunc:  func(e event.UpdateEvent) bool { return validateAODC(e.ObjectOld.GetNamespace(), e.ObjectOld.GetName()) },
	DeleteFunc:  func(e event.DeleteEvent) bool { return validateAODC(e.Object.GetNamespace(), e.Object.GetName()) },
	GenericFunc: func(e event.GenericEvent) bool { return validateAODC(e.Object.GetNamespace(), e.Object.GetName()) },
})

func cmaoPlacementsChanged(old, new client.Object) bool {
	oldCMAO := old.(*addonv1beta1.ClusterManagementAddOn)
	newCMAO := new.(*addonv1beta1.ClusterManagementAddOn)
	return !equality.Semantic.DeepEqual(oldCMAO.Spec.InstallStrategy.Placements, newCMAO.Spec.InstallStrategy.Placements)
}

var cmaoPredicate = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool { return e.Object.GetName() == addoncfg.Name },
	UpdateFunc: func(e event.UpdateEvent) bool {
		return e.ObjectNew.GetName() == addoncfg.Name && cmaoPlacementsChanged(e.ObjectOld, e.ObjectNew)
	},
	DeleteFunc:  func(e event.DeleteEvent) bool { return false },
	GenericFunc: func(e event.GenericEvent) bool { return false },
})

var rsConfigMapPredicate = builder.WithPredicates(rshandlers.RSConfigMapPredicate())

var partOfMCOALabelSelector = labels.SelectorFromSet(labels.Set{
	addoncfg.PartOfK8sLabelKey: addoncfg.Name,
})

var partOfMCOAPredicate = builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return partOfMCOALabelSelector.Matches(labels.Set(obj.GetLabels()))
}))

// SetupWithManager sets up the controller with the Manager.
func SetupWithManager(mgr ctrl.Manager, logger logr.Logger) error {
	l := logger.WithName("resourcecreator")

	r := &ResourceCreatorReconciler{
		Client: mgr.GetClient(),
		Log:    l.WithName("controller"),
		Scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&addonv1beta1.AddOnDeploymentConfig{}, mcoaAODCPredicate).
		// Trigger reconciliations due to changes in Placements
		Watches(&addonv1beta1.ClusterManagementAddOn{}, r.enqueueAODC(), cmaoPredicate).
		// Trigger reconciliations if the pool of ManagedClusters changes
		Watches(&clusterv1.ManagedCluster{}, r.enqueueAODC(), builder.OnlyMetadata).
		// Trigger reconciliations if the metrics configuration resources change
		Watches(&cooprometheusv1alpha1.PrometheusAgent{}, r.enqueueForMCOAOwnedResources()).
		Watches(&cooprometheusv1alpha1.ScrapeConfig{}, r.enqueueForMCOControlledResources(), partOfMCOAPredicate).
		Watches(&prometheusv1.PrometheusRule{}, r.enqueueForMCOControlledResources(), partOfMCOAPredicate).
		// Trigger reconciliations if right-sizing ConfigMaps change
		Watches(&corev1.ConfigMap{}, r.enqueueAODC(), rsConfigMapPredicate).
		Complete(r)
}

// ResourceCreatorReconciler creates resources for default mode according to user configuration
type ResourceCreatorReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ResourceCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.V(2).Info("reconciliation triggered", "request", req.String())

	// Fetch the AddOnDeploymentConfig instance and transform it into the Options struct
	key := client.ObjectKey{Namespace: req.Namespace, Name: req.Name}
	aodc := &addonv1beta1.AddOnDeploymentConfig{}
	if err := r.Get(ctx, key, aodc); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get the AddOnDeploymentConfig: %w", err)
	}
	opts, err := addon.BuildOptions(aodc)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build addon options: %w", err)
	}

	key = client.ObjectKey{Name: addoncfg.Name}
	cmao := &addonv1beta1.ClusterManagementAddOn{}
	if err = r.Get(ctx, key, cmao); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get the ClusterManagementAddOn: %w", err)
	}

	// Reconcile metrics resources
	objs := []common.DefaultConfig{}
	images, err := mconfig.GetImageOverrides(ctx, r.Client, opts.Registries, r.Log)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to get image overrides: %w", err)
	}

	mdefault := mresources.DefaultStackResources{
		Client:             r.Client,
		CMAO:               cmao,
		AddonOptions:       opts,
		Logger:             r.Log,
		KubeRBACProxyImage: images.KubeRBACProxy,
		PrometheusImage:    images.Prometheus,
	}

	mDefaultConfig, err := mdefault.Reconcile(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile metrics resources: %w", err)
	}
	objs = append(objs, mDefaultConfig...)

	// Reconcile right-sizing resources (hub-wide concern).
	// ConfigMap resources are created/updated/deleted here, not per-cluster in handler.go,
	// to avoid race conditions from concurrent Build() calls.
	rsBuilder := &rshandlers.OptionsBuilder{Client: r.Client, Logger: r.Log.WithName("rightsizing")}
	if err := rsBuilder.ReconcileRSResources(ctx, opts); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile right-sizing resources: %w", err)
	}

	if err := common.EnsureAddonConfig(ctx, r.Log, r.Client, objs); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch default configs of the clustermanageraddon: %w", err)
	}

	// Retrieve the updated ClusterManagementAddOn with current default configs
	cmao = &addonv1beta1.ClusterManagementAddOn{}
	if err := r.Get(ctx, types.NamespacedName{Name: addoncfg.Name}, cmao); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get ClusterManagementAddOn: %w", err)
	}
	if err := common.DeleteOrphanResources(ctx, r.Log, r.Client, cmao, &cooprometheusv1alpha1.PrometheusAgentList{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to clean orphan resources: %w", err)
	}

	return ctrl.Result{}, nil
}

func mcoaAODCRequest() []reconcile.Request {
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      addoncfg.Name,
				Namespace: addoncfg.InstallNamespace,
			},
		},
	}
}

func (r *ResourceCreatorReconciler) enqueueAODC() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return mcoaAODCRequest()
	})
}

func (r *ResourceCreatorReconciler) enqueueForMCOAOwnedResources() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		hasOwnerRef, err := controllerutil.HasOwnerReference(obj.GetOwnerReferences(), common.NewMCOAClusterManagementAddOn(), r.Client.Scheme())
		if err != nil {
			r.Log.Error(err, "failed to check owner reference")
			return []reconcile.Request{}
		}

		if !hasOwnerRef {
			return []reconcile.Request{}
		}

		return mcoaAODCRequest()
	})
}

func (r *ResourceCreatorReconciler) enqueueForMCOControlledResources() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		var isControlledByMCO bool
		for _, owner := range obj.GetOwnerReferences() {
			if owner.Controller == nil || !*owner.Controller {
				continue
			}
			gv, err := schema.ParseGroupVersion(owner.APIVersion)
			if err != nil {
				r.Log.V(1).Info("failed to parse groupd version: %s", err.Error())
				continue
			}
			if owner.Kind != "MultiClusterObservability" || gv.Group != "observability.open-cluster-management.io" {
				continue
			}
			isControlledByMCO = true
			break
		}

		if !isControlledByMCO {
			return []reconcile.Request{}
		}

		return mcoaAODCRequest()
	})
}
