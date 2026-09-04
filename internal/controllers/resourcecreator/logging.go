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
	if err := applyLoggingObjects(ctx, r.Client, objs, cmao); err != nil {
		return nil, err
	}
	return configs, nil
}

// reconcileLoggingStorage applies the hub storage component (LokiStack template +
// storage cert) and attaches the LokiStack config to the hub ManagedClusterAddOn.
func (r *ResourceCreatorReconciler) reconcileLoggingStorage(ctx context.Context, cmao *addonv1beta1.ClusterManagementAddOn, opts addon.Options) (ctrl.Result, error) {
	objs, clusterConfig, err := lhandlers.BuildDefaultStackStorageResources(ctx, r.Client, opts.Platform.Logs, opts.UserWorkloads.Logs, opts.HubHostname)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build default stack storage resources: %w", err)
	}
	if err := applyLoggingObjects(ctx, r.Client, objs, cmao); err != nil {
		return ctrl.Result{}, err
	}

	hubName, err := common.LookupHubClusterName(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to look up hub cluster: %w", err)
	}
	desired := make([]addonv1beta1.AddOnConfig, 0, len(clusterConfig))
	for _, cfg := range clusterConfig {
		if cfg.ClusterNamespace == hubName {
			desired = append(desired, cfg.Config)
		}
	}
	if err := common.ApplyManagedClusterAddOnConfigs(ctx, r.Log, r.Client, hubName, desired, lokiv1.GroupVersion.Group, addoncfg.LokiStacksResource); err != nil {
		if errors.IsNotFound(err) && len(desired) > 0 {
			r.Log.Info("hub ManagedClusterAddOn not found, requeueing", "namespace", hubName)
			return ctrl.Result{RequeueAfter: addoncfg.DefaultContextTimeout}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to apply LokiStack config on ManagedClusterAddOn: %w", err)
	}
	return ctrl.Result{}, nil
}
