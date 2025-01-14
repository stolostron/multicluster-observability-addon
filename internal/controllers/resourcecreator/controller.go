package resourcecreator

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	lhandlers "github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	lmanifests "github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func validateAODC(namespace, name string) bool {
	if namespace != addon.InstallNamespace {
		return false
	}
	if name != addon.Name {
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

	// TODO(JoaoBraveCoding) Delete flow

	// TODO(JoaoBraveCoding) Fetch the ManagedClusterAddOn instance to get the placements

	objects := []client.Object{}
	switch {
	case opts.Platform.Logs.ManagedStack:
		// Get ManagedClusterAddOn for buildOptions
		key = client.ObjectKey{Namespace: "local-cluster", Name: addon.Name}
		mcAddon := &addonv1alpha1.ManagedClusterAddOn{}
		if err := r.Get(ctx, key, mcAddon); err != nil {
			return ctrl.Result{}, err
		}

		loggingOpts, err := lhandlers.BuildDefaultOptions(ctx, r.Client, mcAddon, opts.Platform.Logs, opts.UserWorkloads.Logs, true, opts.HubHostname)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Currently there is no difference between the necessary fields to create a
		// CLF instance and the fields that we want to enforce on the default-stack CLF
		// so there is no need to customize BuildSSAClusterLogForwarder to return a
		// slightly different CLF if there is already an instance on the cluster
		clf, err := lmanifests.BuildSSAClusterLogForwarder(loggingOpts)
		if err != nil {
			return ctrl.Result{}, err
		}
		objects = append(objects, clf)

		// TODO(JoaoBraveCoding) update the name
		key = client.ObjectKey{Namespace: addon.InstallNamespace, Name: addon.DefaultStackPrefix}
		existingLS := &lokiv1.LokiStack{}
		err = r.Get(ctx, key, existingLS)
		if err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		// Build the loki instances
		ls, err := lmanifests.BuildSSALokiStack(loggingOpts, existingLS)
		if err != nil {
			return ctrl.Result{}, err
		}
		objects = append(objects, ls)
	}

	// SSA the objects rendered
	for _, obj := range objects {
		if err := r.Client.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(addon.Name)); err != nil {
			klog.Error(err, "failed to configure resource")
			continue
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonv1alpha1.AddOnDeploymentConfig{}, mcoaAODCPredicate, builder.OnlyMetadata).
		Watches(&loggingv1.ClusterLogForwarder{}, r.enqueueDefaultResources()).
		Watches(&lokiv1.LokiStack{}, r.enqueueDefaultResources()).
		Complete(r)
}

func (r *ResourceCreatorReconciler) enqueueDefaultResources() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		// Always re-trigger a reconciliation of the AODC
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
