package resourcecreator

import (
	"context"
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	lhandlers "github.com/stolostron/multicluster-observability-addon/internal/logging/handlers"
	"k8s.io/apimachinery/pkg/api/errors"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func applyLoggingObjects(ctx context.Context, k8s client.Client, objs []client.Object, owner client.Object) error {
	for _, obj := range objs {
		if err := common.ServerSideApply(ctx, k8s, obj, owner); err != nil {
			return fmt.Errorf("failed to apply logging resource %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}
	return nil
}

// reconcileLoggingCollection applies spoke collection templates and returns CMAO placement configs.
func (r *ResourceCreatorReconciler) reconcileLoggingCollection(ctx context.Context, cmao *addonv1beta1.ClusterManagementAddOn, opts addon.Options) ([]common.DefaultConfig, error) {
	objs, configs, err := lhandlers.BuildDefaultStackCollectionResources(ctx, r.Client, cmao, opts.Platform.Logs, opts.UserWorkloads.Logs, opts.HubHostname)
	if err != nil {
		return nil, fmt.Errorf("failed to build default stack collection resources: %w", err)
	}
	if err = applyLoggingObjects(ctx, r.Client, objs, cmao); err != nil {
		return nil, err
	}
	return configs, nil
}

// reconcileLoggingStorage applies the hub storage component (LokiStack template +
// storage cert) and attaches the LokiStack config to the target ManagedClusterAddOn.
func (r *ResourceCreatorReconciler) reconcileLoggingStorage(ctx context.Context, cmao *addonv1beta1.ClusterManagementAddOn, opts addon.Options) (ctrl.Result, error) {
	objs, err := lhandlers.BuildDefaultStackStorageResources(ctx, r.Client, opts.Platform.Logs, opts.UserWorkloads.Logs, opts.HubHostname)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build default stack storage resources: %w", err)
	}
	if err = applyLoggingObjects(ctx, r.Client, objs, cmao); err != nil {
		return ctrl.Result{}, err
	}

	clusterConfig, err := lhandlers.BuildDefaultStackStorageClusterConfig(ctx, r.Client, opts.Platform.Logs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build default stack storage addon config: %w", err)
	}

	if len(clusterConfig) == 0 {
		// Default stack is off: drop our LokiStack config from the hub MCAO.
		// opts.HubHostname is the observability API DNS name from the AODC, not
		// the ManagedCluster name used as the MCAO namespace.
		hubName, lookupErr := common.LookupHubClusterName(ctx, r.Client)
		if lookupErr != nil {
			return ctrl.Result{}, fmt.Errorf("failed to look up hub cluster: %w", lookupErr)
		}
		return r.applyStorageAddonConfig(ctx, hubName, nil)
	}

	var result ctrl.Result
	for _, cfg := range clusterConfig {
		result, err = r.applyStorageAddonConfig(ctx, cfg.ClusterNamespace, []addonv1beta1.AddOnConfig{cfg.Config})
		if err != nil || !result.IsZero() {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ResourceCreatorReconciler) applyStorageAddonConfig(ctx context.Context, clusterNamespace string, desired []addonv1beta1.AddOnConfig) (ctrl.Result, error) {
	if err := common.ApplyManagedClusterAddOnConfigs(ctx, r.Log, r.Client, clusterNamespace, desired, lokiv1.GroupVersion.Group, addoncfg.LokiStacksResource); err != nil {
		if errors.IsNotFound(err) && len(desired) > 0 {
			r.Log.Info("hub ManagedClusterAddOn not found, requeueing", "namespace", clusterNamespace)
			return ctrl.Result{RequeueAfter: addoncfg.DefaultContextTimeout}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to apply LokiStack config on ManagedClusterAddOn: %w", err)
	}
	return ctrl.Result{}, nil
}
