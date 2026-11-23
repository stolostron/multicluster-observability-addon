package common

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errNotClientObjectType = errors.New("object is not a client.Object")

// DeleteOrphanResources lists resources of type T in the install namespace and removes the ones
// that are fully owned by the CMAO and for which none of the placements referenced in their
// placement-ref annotation (comma-separated "namespace/name" entries, see
// addoncfg.PlacementAnnotationKey) exist on the CMAO anymore. Resources that are user-defined or
// partially owned by a user (as determined by IsFullyOwnedByCMAO) are left in place.
//
// Note: Because IsFullyOwnedByCMAO inspects metadata.managedFields, any resource type T read from
// the controller-runtime cache must be exempted from TransformStripManagedFields in
// addoncfg.GetCacheOptions().
func DeleteOrphanResources[T client.ObjectList](ctx context.Context, logger logr.Logger, k8s client.Client, cmao *addonapiv1beta1.ClusterManagementAddOn, items T) error {
	if err := k8s.List(ctx, items, client.InNamespace(addoncfg.InstallNamespace)); err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
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

		if !IsFullyOwnedByCMAO(obj, cmao) {
			continue
		}

		if hasExistingPlacementRef(obj, placementsDict) {
			continue
		}

		if err := k8s.Delete(ctx, obj); err != nil {
			return fmt.Errorf("failed to delete orphan resource: %w", err)
		}
		logger.Info("default configuration deleted", "name", obj.GetName(), "namespace", obj.GetNamespace(), "kind", obj.GetObjectKind().GroupVersionKind().Kind)
	}

	return nil
}

// IsFullyOwnedByCMAO returns true if obj is owned by cmao (via OwnerReferences) and has not been
// created or partially modified by a user.
// A resource is considered user-owned or partially user-owned if:
//   - it does not have cmao in its OwnerReferences,
//   - it carries the user opt-in label (app.kubernetes.io/part-of: multicluster-observability-addon), or
//   - any non-subresource entry in metadata.managedFields belongs to a manager other than MCOA
//     (addoncfg.Name).
//
// Note: Callers inspecting cached objects must ensure the resource type is exempted from
// TransformStripManagedFields in addoncfg.GetCacheOptions() so metadata.managedFields is retained.
func IsFullyOwnedByCMAO(obj client.Object, cmao *addonapiv1beta1.ClusterManagementAddOn) bool {
	if !hasCMAOOwnerReference(obj, cmao) {
		return false
	}

	if obj.GetLabels()[addoncfg.PartOfK8sLabelKey] == addoncfg.Name {
		return false
	}

	for _, entry := range obj.GetManagedFields() {
		if entry.Subresource != "" {
			continue
		}
		if entry.Manager != addoncfg.Name {
			return false
		}
	}

	return true
}

func hasCMAOOwnerReference(obj client.Object, cmao *addonapiv1beta1.ClusterManagementAddOn) bool {
	if cmao.GetUID() != "" && metav1.IsControlledBy(obj, cmao) {
		return true
	}
	return slices.ContainsFunc(obj.GetOwnerReferences(), func(owner metav1.OwnerReference) bool {
		return owner.Kind == "ClusterManagementAddOn" &&
			owner.Name == cmao.GetName() &&
			strings.HasPrefix(owner.APIVersion, addonapiv1beta1.GroupName+"/")
	})
}

// hasExistingPlacementRef returns true if at least one of the "namespace/name" entries in the
// object's placement-ref annotation is still present in placementsDict.
func hasExistingPlacementRef(obj client.Object, placementsDict map[string]struct{}) bool {
	annotation := obj.GetAnnotations()[addoncfg.PlacementAnnotationKey]
	if annotation == "" {
		return false
	}

	for ref := range strings.SplitSeq(annotation, ",") {
		ref = strings.TrimSpace(ref)
		if _, ok := placementsDict[ref]; ok {
			return true
		}
	}

	return false
}
