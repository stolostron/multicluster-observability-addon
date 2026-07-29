package common

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"k8s.io/apimachinery/pkg/api/meta"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var errNotClientObjectType = errors.New("object is not a client.Object")

// DeleteOrphanResources lists resources of type T that are either owned by the CMAO (default
// resources) or opted into MCOA management via the part-of label (user-defined resources), and
// removes the ones for which none of the placements referenced in their placement-ref annotation
// (comma-separated "namespace/name" entries, see addoncfg.PlacementAnnotationKey) exist on the
// CMAO anymore. A resource referencing multiple placements is only deleted once all of them are
// gone.
func DeleteOrphanResources[T client.ObjectList](ctx context.Context, logger logr.Logger, k8s client.Client, cmao *addonapiv1beta1.ClusterManagementAddOn, items T) error {
	if err := k8s.List(ctx, items, client.InNamespace(addoncfg.InstallNamespace)); err != nil {
		return fmt.Errorf("failed to list PrometheusAgents: %w", err)
	}

	placementsDict := map[string]struct{}{}
	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		placementsDict[fmt.Sprintf("%s/%s", placement.Namespace, placement.Name)] = struct{}{}
	}

	// Use the Meta interface to get objects from the list
	objs, err := meta.ExtractList(items)
	if err != nil {
		return fmt.Errorf("failed to extract items from list: %w", err)
	}

	for _, rawObj := range objs {
		obj, ok := rawObj.(client.Object)
		if !ok {
			return errNotClientObjectType
		}

		hasOwnerRef, err := controllerutil.HasOwnerReference(obj.GetOwnerReferences(), cmao, k8s.Scheme())
		if err != nil {
			return fmt.Errorf("failed to check owner references: %w", err)
		}

		isUserDefined := obj.GetLabels()[addoncfg.PartOfK8sLabelKey] == addoncfg.Name

		if !hasOwnerRef && !isUserDefined {
			continue
		}

		if hasExistingPlacementRef(obj, placementsDict) {
			continue
		}

		if err := k8s.Delete(ctx, obj); err != nil {
			return fmt.Errorf("failed to delete owned agent: %w", err)
		}
		logger.Info("default configuration deleted", "name", obj.GetName(), "namespace", obj.GetNamespace(), "kind", obj.GetObjectKind().GroupVersionKind().Kind)
	}

	return nil
}

// hasExistingPlacementRef returns true if at least one of the "namespace/name" entries in the
// object's placement-ref annotation is still present in placementsDict, or refers to the internal
// "dummy" placement sentinel.
func hasExistingPlacementRef(obj client.Object, placementsDict map[string]struct{}) bool {
	annotation := obj.GetAnnotations()[addoncfg.PlacementAnnotationKey]
	if annotation == "" {
		return false
	}

	for ref := range strings.SplitSeq(annotation, ",") {
		ref = strings.TrimSpace(ref)
		if isDummyPlacementRef(ref) {
			return true
		}
		if _, ok := placementsDict[ref]; ok {
			return true
		}
	}

	return false
}

// isDummyPlacementRef returns true if ref refers to the internal "dummy" placement sentinel used by
// CreateDefaultAgent as a placeholder when no default agent is otherwise needed. It never
// corresponds to a real Placement, so it must never be treated as orphaned.
func isDummyPlacementRef(ref string) bool {
	_, name, ok := strings.Cut(ref, "/")
	return ok && name == "dummy"
}
