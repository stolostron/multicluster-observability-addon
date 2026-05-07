package handlers

import (
	"context"
	"fmt"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	return namespace == addoncfg.InstallNamespace &&
		(name == rightsizing.NamespaceConfigMapName || name == rightsizing.VirtualizationConfigMapName)
}

// ReconcileRSResources ensures right-sizing Placement and ConfigMap resources are
// created for enabled features and cleaned up for disabled features.
// Called from ResourceCreator (hub-wide, not per-cluster) to avoid race conditions.
func (o *OptionsBuilder) ReconcileRSResources(ctx context.Context, opts addon.Options) error {
	// NOTE: Do NOT gate on opts.Platform.Enabled here.
	// When both RS features are disabled and no other platform key is set,
	// Platform.Enabled is false — but we still need to run cleanup below.

	// Clean up RS Placements orphaned in non-canonical namespaces.
	// This handles mode switches where MCO used a custom NamespaceBinding.
	if err := o.deleteOrphanRSPlacements(ctx); err != nil {
		return fmt.Errorf("failed to cleanup orphan RS placements: %w", err)
	}

	if opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled {
		configData, err := o.getConfigData(ctx, rightsizing.NamespaceConfigMapName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				configData.PlacementConfiguration = rightsizing.GetDefaultRSPlacement()
			} else {
				return fmt.Errorf("failed to get namespace config: %w", err)
			}
		}
		if err := o.ensureRSPlacement(ctx, rightsizing.NamespacePlacementName, configData.PlacementConfiguration); err != nil {
			return fmt.Errorf("failed to ensure namespace placement: %w", err)
		}
	} else {
		if err := o.deleteRSPlacement(ctx, rightsizing.NamespacePlacementName); err != nil {
			return fmt.Errorf("failed to cleanup namespace placement: %w", err)
		}
		if err := o.deleteRSConfigMap(ctx, rightsizing.NamespaceConfigMapName); err != nil {
			return fmt.Errorf("failed to cleanup namespace configmap: %w", err)
		}
	}

	if opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled {
		configData, err := o.getConfigData(ctx, rightsizing.VirtualizationConfigMapName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				configData.PlacementConfiguration = rightsizing.GetDefaultRSPlacement()
			} else {
				return fmt.Errorf("failed to get virtualization config: %w", err)
			}
		}
		if err := o.ensureRSPlacement(ctx, rightsizing.VirtualizationPlacementName, configData.PlacementConfiguration); err != nil {
			return fmt.Errorf("failed to ensure virtualization placement: %w", err)
		}
	} else {
		if err := o.deleteRSPlacement(ctx, rightsizing.VirtualizationPlacementName); err != nil {
			return fmt.Errorf("failed to cleanup virtualization placement: %w", err)
		}
		if err := o.deleteRSConfigMap(ctx, rightsizing.VirtualizationConfigMapName); err != nil {
			return fmt.Errorf("failed to cleanup virtualization configmap: %w", err)
		}
	}

	return nil
}

// ensureRSPlacement creates or updates a right-sizing Placement resource.
// Handles AlreadyExists race condition gracefully.
func (o *OptionsBuilder) ensureRSPlacement(ctx context.Context, placementName string, placementConfig clusterv1beta1.Placement) error {
	key := types.NamespacedName{Name: placementName, Namespace: rightsizing.PlacementNamespace}
	placement := &clusterv1beta1.Placement{}

	err := o.Client.Get(ctx, key, placement)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get placement %s: %w", placementName, err)
		}

		// Create new placement
		placement = &clusterv1beta1.Placement{
			ObjectMeta: metav1.ObjectMeta{
				Name:      placementName,
				Namespace: rightsizing.PlacementNamespace,
				Labels:    rightsizing.RSLabels(),
			},
			Spec: placementConfig.Spec,
		}

		if createErr := o.Client.Create(ctx, placement); createErr != nil {
			if apierrors.IsAlreadyExists(createErr) {
				// Concurrent create — fall through to update
				if err := o.Client.Get(ctx, key, placement); err != nil {
					return fmt.Errorf("failed to re-fetch placement after AlreadyExists: %w", err)
				}
			} else {
				return fmt.Errorf("failed to create placement %s: %w", placementName, createErr)
			}
		} else {
			o.Logger.V(1).Info("Created right-sizing Placement", "name", placementName, "namespace", rightsizing.PlacementNamespace)
			return nil
		}
	}

	// Update existing placement spec
	placement.Spec = placementConfig.Spec
	if err := o.Client.Update(ctx, placement); err != nil {
		return fmt.Errorf("failed to update placement %s: %w", placementName, err)
	}
	o.Logger.V(1).Info("Updated right-sizing Placement", "name", placementName, "namespace", rightsizing.PlacementNamespace)
	return nil
}

// isClusterSelectedByRSPlacement checks if a cluster is selected by a right-sizing
// Placement by reading the PlacementDecisions associated with that Placement.
func (o *OptionsBuilder) isClusterSelectedByRSPlacement(ctx context.Context, placementName, clusterName string) (bool, error) {
	placementDecisionList := &clusterv1beta1.PlacementDecisionList{}
	err := o.Client.List(ctx, placementDecisionList,
		client.InNamespace(rightsizing.PlacementNamespace),
		client.MatchingLabels{rightsizing.PlacementDecisionLabel: placementName},
	)
	if err != nil {
		return false, fmt.Errorf("failed to list PlacementDecisions for %s: %w", placementName, err)
	}

	if len(placementDecisionList.Items) == 0 {
		// No PlacementDecisions yet — Placement may be newly created.
		// Default to true (fail-open) to avoid blocking deployment while scheduler catches up.
		// Window is typically 10-30 seconds. Rules on wrong clusters briefly is benign.
		o.Logger.V(1).Info("No PlacementDecisions found, defaulting to selected",
			"placement", placementName, "cluster", clusterName)
		return true, nil
	}

	for _, pd := range placementDecisionList.Items {
		for _, decision := range pd.Status.Decisions {
			if decision.ClusterName == clusterName {
				return true, nil
			}
		}
	}

	o.Logger.V(1).Info("Cluster not selected by placement",
		"placement", placementName, "cluster", clusterName)
	return false, nil
}

// deleteOrphanRSPlacements finds and deletes RS Placements that exist outside
// PlacementNamespace. This cleans up orphans left by MCO when it was using a
// custom NamespaceBinding namespace and the user switches to MCOA mode.
func (o *OptionsBuilder) deleteOrphanRSPlacements(ctx context.Context) error {
	placementList := &clusterv1beta1.PlacementList{}
	if err := o.Client.List(ctx, placementList,
		client.MatchingLabels(rightsizing.RSLabels()),
	); err != nil {
		return fmt.Errorf("failed to list RS placements: %w", err)
	}

	for i := range placementList.Items {
		p := &placementList.Items[i]
		if p.Namespace == rightsizing.PlacementNamespace {
			continue
		}
		if err := o.Client.Delete(ctx, p); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete orphan RS placement %s/%s: %w", p.Namespace, p.Name, err)
		}
		o.Logger.Info("Deleted orphan right-sizing Placement from non-canonical namespace",
			"name", p.Name, "namespace", p.Namespace)
	}
	return nil
}

// deleteRSPlacement deletes a right-sizing Placement resource if it exists.
func (o *OptionsBuilder) deleteRSPlacement(ctx context.Context, placementName string) error {
	placement := &clusterv1beta1.Placement{}
	key := types.NamespacedName{Name: placementName, Namespace: rightsizing.PlacementNamespace}
	if err := o.Client.Get(ctx, key, placement); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get placement %s: %w", placementName, err)
	}
	if err := o.Client.Delete(ctx, placement); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete placement %s: %w", placementName, err)
	}
	o.Logger.V(1).Info("Deleted right-sizing Placement", "name", placementName, "namespace", rightsizing.PlacementNamespace)
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
