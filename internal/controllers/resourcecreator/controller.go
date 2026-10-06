package resourcecreator

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	routev1 "github.com/openshift/api/route/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	rshandlers "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/handlers"
	"github.com/stolostron/multicluster-observability-addon/internal/mcoagateway"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	mresources "github.com/stolostron/multicluster-observability-addon/internal/metrics/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func isHubManagedClusterAddOn(ctx context.Context, k8s client.Client, obj client.Object) bool {
	// Every cluster's ManagedClusterAddOn is named after the addon. Match the
	// same hub name used when attaching LokiStack (labeled self-managed cluster,
	// or local-cluster when none is labeled).
	if obj.GetName() != addoncfg.Name {
		return false
	}
	hubName, err := common.LookupHubClusterName(ctx, k8s)
	if err != nil {
		return obj.GetNamespace() == addoncfg.HubNamespace
	}
	return obj.GetNamespace() == hubName
}

func hubMCAOPredicate(k8s client.Client) builder.Predicates {
	return builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return isHubManagedClusterAddOn(context.TODO(), k8s, obj)
	}))
}

var rsConfigMapPredicate = builder.WithPredicates(rshandlers.RSConfigMapPredicate())

// isGatewayRoute matches the single Route rendered by the mcoa-gateway Helm
// chart. It isn't MCOA-owned (no controller ref, no part-of label - it's
// applied via the addon-framework's Helm rendering, not SSA'd by this
// controller), so it needs its own targeted predicate rather than
// enqueueForMCOAOwnedResources.
func isGatewayRoute(namespace, name string) bool {
	return namespace == addoncfg.InstallNamespace && name == mcoagateway.GatewayRouteName
}

var mcoaGatewayRoutePredicate = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool { return isGatewayRoute(e.Object.GetNamespace(), e.Object.GetName()) },
	UpdateFunc: func(e event.UpdateEvent) bool {
		return isGatewayRoute(e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
	},
	DeleteFunc:  func(e event.DeleteEvent) bool { return false },
	GenericFunc: func(e event.GenericEvent) bool { return false },
})

func isGatewayServerCertSecret(namespace, name string) bool {
	return namespace == addoncfg.InstallNamespace && name == mcoagateway.DefaultStorageMTLSSecretName
}

var mcoaGatewayServerCertPredicate = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return isGatewayServerCertSecret(e.Object.GetNamespace(), e.Object.GetName())
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		return isGatewayServerCertSecret(e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
	},
	DeleteFunc:  func(e event.DeleteEvent) bool { return false },
	GenericFunc: func(e event.GenericEvent) bool { return false },
})

var partOfMCOALabelSelector = labels.SelectorFromSet(labels.Set{
	addoncfg.PartOfK8sLabelKey: addoncfg.Name,
})

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
		Watches(&cooprometheusv1alpha1.ScrapeConfig{}, r.enqueueForMCOControlledResources()).
		Watches(&prometheusv1.PrometheusRule{}, r.enqueueForMCOControlledResources()).
		// Trigger reconciliations if right-sizing ConfigMaps change
		Watches(&corev1.ConfigMap{}, r.enqueueAODC(), rsConfigMapPredicate).
		// Trigger reconciliations if logging resources change
		Watches(&lokiv1.LokiStack{}, r.enqueueForMCOAOwnedResources()).
		Watches(&loggingv1.ClusterLogForwarder{}, r.enqueueForMCOAOwnedResources()).
		// Trigger reconciliations once the mcoa-gateway Route is admitted (or its
		// host changes), so the gateway server certificate's SAN can be updated
		// to match without waiting for an unrelated reconcile trigger.
		Watches(&routev1.Route{}, r.enqueueAODC(), mcoaGatewayRoutePredicate).
		// Trigger when cert-manager issues/rotates the gateway server secret so
		// CLF placements are published as soon as the Route host is on the SAN.
		Watches(&corev1.Secret{}, r.enqueueAODC(), mcoaGatewayServerCertPredicate).
		// Trigger when the hub ManagedClusterAddOn is created so LokiStack can be attached to it.
		// Hub namespace is resolved the same way as storage attach (not hardcoded to local-cluster).
		Watches(&addonv1beta1.ManagedClusterAddOn{}, r.enqueueAODC(), hubMCAOPredicate(r.Client)).
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
	if err != nil && !apierrors.IsNotFound(err) {
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

	mDefaultConfig, metricsErr := mdefault.Reconcile(ctx)
	if metricsErr != nil {
		r.Log.Error(metricsErr, "failed to reconcile metrics resources, continuing with logging")
	} else {
		objs = append(objs, mDefaultConfig...)
	}

	// Reconcile right-sizing resources (hub-wide concern).
	// ConfigMap resources are created/updated/deleted here, not per-cluster in handler.go,
	// to avoid race conditions from concurrent Build() calls.
	rsBuilder := &rshandlers.OptionsBuilder{Client: r.Client, Logger: r.Log.WithName("rightsizing")}
	if rsErr := rsBuilder.ReconcileRSResources(ctx, opts); rsErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile right-sizing resources: %w", rsErr)
	}

	gatewayEnabled := opts.Platform.Logs.DefaultStack || opts.ThanosOperatorEnabled || aodc.Annotations["mcoa-obs-api"] == "true"
	if gatewayEnabled {
		gatewayHost, err := mcoagateway.GetGatewayRouteHost(ctx, r.Client)
		if err != nil {
			r.Log.V(1).Info("Failed to get MCOA gateway route host, certificate will be built without that SAN for now", "err", err)
		}

		serverCert, err := mcoagateway.BuildServerCertificate(gatewayHost)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to build MCOA gateway server certificate: %w", err)
		}
		certObjs := []client.Object{serverCert}

		if opts.Platform.Logs.DefaultStack {
			lokiClientCert, err := mcoagateway.BuildLokiClientCertificate()
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to build MCOA gateway Loki client certificate: %w", err)
			}
			certObjs = append(certObjs, lokiClientCert)

			managedClusters := &clusterv1.ManagedClusterList{}
			if err := r.List(ctx, managedClusters, &client.ListOptions{}); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get managed cluster list: %w", err)
			}
			// One collector client cert per cluster; secret data is copied to the
			// spoke via the managed collection Helm chart. OU is the cluster name
			// so the gateway can set X-Scope-OrgID from the client certificate.
			for _, cluster := range managedClusters.Items {
				clientCert, err := mcoagateway.BuildCollectionCertificate(cluster.Name)
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to build MCOA gateway collection certificate for %s: %w", cluster.Name, err)
				}
				certObjs = append(certObjs, clientCert)
			}
		}

		for _, obj := range certObjs {
			if err := common.ServerSideApply(ctx, r.Client, obj, cmao); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to apply certificate %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
			}
		}
	}

	// Spoke collection: CLF templates are applied here. Placement configs are
	// appended only after LokiStack is requested on the hub MCAO so spokes do
	// not start forwarding before storage is requested. Metrics configs stay
	// on objs regardless of that wait.
	lDefaultConfig, clfResult, clfErr := r.reconcileLoggingCollection(ctx, cmao, opts)
	if clfErr != nil {
		r.Log.Error(clfErr, "failed to build CLF resources, will requeue and continue to LokiStack")
	}

	// Hub storage component: LokiStack template + MCAO pointer (movable to another cluster later).
	storageResult, storageErr := r.reconcileLoggingStorage(ctx, cmao, opts)

	// Publish CLF placement configs only once storage has been requested on the
	// hub MCAO. A storage error or requeue keeps spokes from forwarding into a
	// stack that is not ready, while metrics configs below are still applied.
	if storageErr == nil && storageResult.IsZero() && clfErr == nil && clfResult.IsZero() {
		objs = append(objs, lDefaultConfig...)
	} else if opts.Platform.Logs.DefaultStack {
		if stripErr := r.stripDefaultStackCLFPlacementConfigs(ctx); stripErr != nil {
			return ctrl.Result{}, stripErr
		}
	}

	if err := common.EnsureAddonConfig(ctx, r.Log, r.Client, objs); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch default configs of the clustermanageraddon: %w", err)
	}

	if err := errors.Join(metricsErr, storageErr, clfErr); err != nil {
		return ctrl.Result{}, err
	}

	if !storageResult.IsZero() {
		return storageResult, nil
	}

	if !clfResult.IsZero() {
		return clfResult, nil
	}

	// Retrieve the updated ClusterManagementAddOn with current default configs
	cmao = &addonv1beta1.ClusterManagementAddOn{}
	if err := r.Get(ctx, types.NamespacedName{Name: addoncfg.Name}, cmao); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get ClusterManagementAddOn: %w", err)
	}
	// Deletes owned PrometheusAgents whose placement-ref annotation no longer references any
	// placement declared on the CMAO. Agents referencing multiple placements are kept as long as
	// at least one of them still exists.
	if err := common.DeleteOrphanResources(ctx, r.Log, r.Client, cmao, &cooprometheusv1alpha1.PrometheusAgentList{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to clean orphan resources: %w", err)
	}
	if err := common.DeleteOrphanResources(ctx, r.Log, r.Client, cmao, &lokiv1.LokiStackList{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to clean orphan logging storage resources: %w", err)
	}
	if err := common.DeleteOrphanResources(ctx, r.Log, r.Client, cmao, &loggingv1.ClusterLogForwarderList{}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to clean orphan logging collection resources: %w", err)
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
		if obj == nil {
			return []reconcile.Request{}
		}

		if partOfMCOALabelSelector.Matches(labels.Set(obj.GetLabels())) {
			return mcoaAODCRequest()
		}

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
		if obj == nil {
			return []reconcile.Request{}
		}

		if partOfMCOALabelSelector.Matches(labels.Set(obj.GetLabels())) {
			return mcoaAODCRequest()
		}

		var isControlledByMCO bool
		for _, owner := range obj.GetOwnerReferences() {
			if owner.Controller == nil || !*owner.Controller {
				continue
			}
			gv, err := schema.ParseGroupVersion(owner.APIVersion)
			if err != nil {
				r.Log.V(1).Info("failed to parse group version", "err", err)
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
