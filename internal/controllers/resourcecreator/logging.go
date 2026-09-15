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
	// Drop leftover LokiStack placement configs before anything else so addon-manager
	// stops fanning storage to spokes, even if object-storage setup later fails.
	if err := common.StripPlacementConfigs(ctx, r.Log, r.Client, lokiv1.GroupVersion.Group, addoncfg.LokiStacksResource); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to strip LokiStack configs from ClusterManagementAddOn: %w", err)
	}

	clusterConfig, err := lhandlers.BuildDefaultStackStorageClusterConfig(ctx, r.Client, opts.Platform.Logs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build default stack storage addon config: %w", err)
	}

	// Strip LokiStack from every MCAO that should not run storage (upgrade leftovers
	// copied onto spec.configs). Leave the target untouched until hub objects exist.
	if _, err = r.applyStorageAddonConfigs(ctx, clusterConfig, false); err != nil {
		return ctrl.Result{}, err
	}

	objs, err := lhandlers.BuildDefaultStackStorageResources(ctx, r.Client, opts.Platform.Logs, opts.UserWorkloads.Logs, opts.HubHostname)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build default stack storage resources: %w", err)
	}
	if err = applyLoggingObjects(ctx, r.Client, objs, cmao); err != nil {
		return ctrl.Result{}, err
	}

	return r.applyStorageAddonConfigs(ctx, clusterConfig, true)
}

// applyStorageAddonConfigs writes LokiStack configs onto ManagedClusterAddOns.
// When attachDesired is false, target clusters are left untouched and leftover
// LokiStack configs are removed from every other MCAO. When true, desired configs
// are applied to their target namespaces (empty desired strips all) and missing
// targets requeue.
func (r *ResourceCreatorReconciler) applyStorageAddonConfigs(ctx context.Context, desired []common.ClusterAddonConfig, attachDesired bool) (ctrl.Result, error) {
	desiredByNS := make(map[string][]addonv1beta1.AddOnConfig, len(desired))
	for _, cfg := range desired {
		desiredByNS[cfg.ClusterNamespace] = append(desiredByNS[cfg.ClusterNamespace], cfg.Config)
	}

	list := &addonv1beta1.ManagedClusterAddOnList{}
	if err := r.List(ctx, list); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list ManagedClusterAddOns: %w", err)
	}

	foundDesired := make(map[string]struct{}, len(desiredByNS))
	for i := range list.Items {
		mcAddon := &list.Items[i]
		if mcAddon.Name != addoncfg.Name {
			continue
		}

		configs, isTarget := desiredByNS[mcAddon.Namespace]
		if isTarget && !attachDesired {
			continue
		}
		if !isTarget {
			configs = nil
		}

		result, err := r.applyStorageAddonConfig(ctx, mcAddon.Namespace, configs)
		if err != nil || !result.IsZero() {
			return result, err
		}
		if isTarget {
			foundDesired[mcAddon.Namespace] = struct{}{}
		}
	}

	if !attachDesired {
		return ctrl.Result{}, nil
	}
	for ns := range desiredByNS {
		if _, ok := foundDesired[ns]; ok {
			continue
		}
		r.Log.Info("hub ManagedClusterAddOn not found, requeueing", "namespace", ns)
		return ctrl.Result{RequeueAfter: addoncfg.DefaultContextTimeout}, nil
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
