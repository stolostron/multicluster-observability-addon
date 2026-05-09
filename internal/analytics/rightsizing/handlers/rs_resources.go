package handlers

import (
	"context"
	"fmt"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// RSConfigMapPredicate returns a predicate that filters ConfigMap watch events
// to only RS ConfigMaps. Delete and generic events are ignored to prevent
// MCOA from reconciling when MCO deletes ConfigMaps during its finalizer cleanup.
func RSConfigMapPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isRSConfigMap(e.Object.GetNamespace(), e.Object.GetName())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isRSConfigMap(e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func isRSConfigMap(namespace, name string) bool {
	if namespace != addoncfg.InstallNamespace {
		return false
	}
	switch name {
	case rightsizing.NamespaceConfigMapName, rightsizing.VirtualizationConfigMapName, rightsizing.WorkloadConfigMapName:
		return true
	}
	return false
}

// ReconcileRSResources ensures right-sizing ConfigMap resources are cleaned up
// for disabled features.
// Called from ResourceCreator (hub-wide, not per-cluster) to avoid race conditions.
func (o *OptionsBuilder) ReconcileRSResources(ctx context.Context, opts addon.Options) error {
	if !opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled {
		if err := o.deleteRSConfigMap(ctx, rightsizing.NamespaceConfigMapName); err != nil {
			return fmt.Errorf("failed to cleanup namespace configmap: %w", err)
		}
	}

	if !opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled {
		if err := o.deleteRSConfigMap(ctx, rightsizing.VirtualizationConfigMapName); err != nil {
			return fmt.Errorf("failed to cleanup virtualization configmap: %w", err)
		}
	}

	if !opts.Platform.AnalyticsOptions.RightSizing.WorkloadPodEnabled {
		if err := o.deleteRSConfigMap(ctx, rightsizing.WorkloadConfigMapName); err != nil {
			return fmt.Errorf("failed to cleanup workload configmap: %w", err)
		}
		if err := o.deleteRSConfigMap(ctx, rightsizing.WorkloadPlacementCMName); err != nil {
			return fmt.Errorf("failed to cleanup workload placement configmap: %w", err)
		}
	}

	return nil
}

// deleteRSConfigMap deletes a right-sizing ConfigMap resource if it exists.
func (o *OptionsBuilder) deleteRSConfigMap(ctx context.Context, configMapName string) error {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{Name: configMapName, Namespace: addoncfg.InstallNamespace}
	if err := o.Client.Get(ctx, key, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get configmap %s: %w", configMapName, err)
	}
	if err := o.Client.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete configmap %s: %w", configMapName, err)
	}
	o.Logger.V(1).Info("Deleted right-sizing ConfigMap", "name", configMapName, "namespace", addoncfg.InstallNamespace)
	return nil
}
