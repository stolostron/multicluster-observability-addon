package common

import (
	"context"
	"fmt"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"k8s.io/apimachinery/pkg/api/meta"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CleanOrphanResources lists PrometheusAgents owned by CMOA and removes the ones having no existing placement.
func CleanOrphanResources[T client.ObjectList](ctx context.Context, k8s client.Client, cmao *addonapiv1alpha1.ClusterManagementAddOn, items T) error {
	if err := k8s.List(ctx, items, client.InNamespace(addon.InstallNamespace)); err != nil {
		return fmt.Errorf("failed to list PrometheusAgents: %w", err)
	}

	makePlacementKey := func(namespace, name string) string {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	placementsDict := map[string]struct{}{}
	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		placementsDict[makePlacementKey(placement.Namespace, placement.Name)] = struct{}{}
	}

	// Use the Meta interface to get objects from the list
	objs, err := meta.ExtractList(items)
	if err != nil {
		return fmt.Errorf("failed to extract items from list: %w", err)
	}

	for _, rawObj := range objs {
		obj, ok := rawObj.(client.Object)
		if !ok {
			return fmt.Errorf("object is not a client.Object")
		}

		hasOwnerRef, err := controllerutil.HasOwnerReference(obj.GetOwnerReferences(), cmao, k8s.Scheme())
		if err != nil {
			return fmt.Errorf("failed to check owner references: %w", err)
		}

		if !hasOwnerRef {
			continue
		}

		labels := obj.GetLabels()
		placementNs := labels[addon.PlacementRefNamespaceLabelKey]
		placementName := labels[addon.PlacementRefNameLabelKey]
		if _, ok := placementsDict[makePlacementKey(placementNs, placementName)]; ok {
			continue
		}

		if err := k8s.Delete(ctx, obj); err != nil {
			return fmt.Errorf("failed to delete owned agent: %w", err)
		}
	}

	return nil
}
