package resourcecreator

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	mresources "github.com/stolostron/multicluster-observability-addon/internal/metrics/resource"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func validateAODC(namespace, name string) bool {
	if namespace != addon.InstallNamespace || name != addon.Name {
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
	oldCMAO := old.(*addonv1alpha1.ClusterManagementAddOn)
	newCMAO := new.(*addonv1alpha1.ClusterManagementAddOn)
	return !equality.Semantic.DeepEqual(oldCMAO.Spec.InstallStrategy.Placements, newCMAO.Spec.InstallStrategy.Placements)
}

var cmaoPredicate = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool { return e.Object.GetName() == addon.Name },
	UpdateFunc: func(e event.UpdateEvent) bool {
		return e.ObjectNew.GetName() == addon.Name && cmaoPlacementsChanged(e.ObjectOld, e.ObjectNew)
	},
	DeleteFunc:  func(e event.DeleteEvent) bool { return false },
	GenericFunc: func(e event.GenericEvent) bool { return false },
})

type ResourceCreatorManager struct {
	mgr    *ctrl.Manager
	logger logr.Logger
}

func NewResourceCreatorManager(logger logr.Logger, scheme *runtime.Scheme) (*ResourceCreatorManager, error) {
	l := logger.WithName("mcoa-resourcecreator")

	ctrl.SetLogger(l)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: ":8084",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to start manager: %w", err)
	}

	if err = (&ResourceCreatorReconciler{
		Client: mgr.GetClient(),
		Log:    l,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create mcoa-resourcecreator controller: %w", err)
	}

	wm := ResourceCreatorManager{
		mgr:    &mgr,
		logger: l,
	}

	return &wm, nil
}

func (wm *ResourceCreatorManager) Start(ctx context.Context) {
	wm.logger.Info("Starting resourcecreator manager")
	go func() {
		err := (*wm.mgr).Start(ctx)
		if err != nil {
			wm.logger.Error(err, "there was an error while running the reconciliation resourcecreator")
		}
	}()
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
	// Fetch the AddOnDeploymentConfig instance and transform it into the Options struct
	key := client.ObjectKey{Namespace: req.Namespace, Name: req.Name}
	aodc := &addonv1alpha1.AddOnDeploymentConfig{}
	if err := r.Get(ctx, key, aodc); err != nil {
		return ctrl.Result{}, err
	}
	opts, err := addon.BuildOptions(aodc)
	if err != nil {
		return ctrl.Result{}, err
	}

	key = client.ObjectKey{Name: addon.Name}
	cmao := &addonv1alpha1.ClusterManagementAddOn{}
	if err = r.Get(ctx, key, cmao); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile metrics resources
	images, err := mconfig.GetImageOverrides(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get image overrides: %w", err)
	}

	mdefault := mresources.DefaultStackResources{
		Client:          r.Client,
		CMAO:            cmao,
		AddonOptions:    opts,
		Logger:          r.Log,
		PrometheusImage: images.Prometheus,
	}
	if err := mdefault.Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonv1alpha1.AddOnDeploymentConfig{}, mcoaAODCPredicate, builder.OnlyMetadata).
		Watches(&addonv1alpha1.ClusterManagementAddOn{}, handler.EnqueueRequestsFromMapFunc(enqueueAddon), cmaoPredicate).
		Watches(&clusterv1.ManagedCluster{}, r.enqueueAODC()).
		Watches(&loggingv1.ClusterLogForwarder{}, r.enqueueDefaultResources()).
		Watches(&prometheusalpha1.PrometheusAgent{}, r.enqueueDefaultResources()).
		Complete(r)
}

func (r *ResourceCreatorReconciler) enqueueAODC() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      addon.Name,
					Namespace: addon.InstallNamespace,
				},
			},
		}
	})
}

func (r *ResourceCreatorReconciler) enqueueDefaultResources() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		hasOwnerRef, err := controllerutil.HasOwnerReference(obj.GetOwnerReferences(), common.NewMCOAClusterManagementAddOn(), r.Client.Scheme())
		if err != nil {
			r.Log.Error(err, "failed to check owner reference")
			return []reconcile.Request{}
		}

		if !hasOwnerRef {
			return []reconcile.Request{}
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      addon.Name,
					Namespace: addon.InstallNamespace,
				},
			},
		}
	})
}

func enqueueAddon(ctx context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      addon.Name,
				Namespace: addon.InstallNamespace,
			},
		},
	}
}
