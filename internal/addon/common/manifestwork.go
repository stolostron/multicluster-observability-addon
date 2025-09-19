package common

import (
	"context"
	"fmt"

	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetFeedbackValuesForResources finds all feedback values for a list of specific resources
// across all ManifestWorks for the addon. It performs a single pass over the ManifestWorks
// and returns a map where each key is a ResourceIdentifier and the value is a slice
// of all feedback values found for that resource.
func GetFeedbackValuesForResources(
	ctx context.Context,
	kubeClient client.Client,
	clusterName string,
	addonName string,
	resourceIDs ...workv1.ResourceIdentifier, // Variadic for convenience
) (map[workv1.ResourceIdentifier][]workv1.FeedbackValue, error) {
	results := make(map[workv1.ResourceIdentifier][]workv1.FeedbackValue)
	for _, id := range resourceIDs {
		results[id] = []workv1.FeedbackValue{} // Pre-populate to ensure keys exist
	}

	workList, err := ListAddonManifestWorks(ctx, kubeClient, clusterName, addonName)
	if err != nil {
		return nil, err
	}

	for _, work := range workList.Items {
		for _, manifestStatus := range work.Status.ResourceStatus.Manifests {
			currentID := workv1.ResourceIdentifier{
				Group:     manifestStatus.ResourceMeta.Group,
				Resource:  manifestStatus.ResourceMeta.Resource,
				Name:      manifestStatus.ResourceMeta.Name,
				Namespace: manifestStatus.ResourceMeta.Namespace,
			}

			if _, ok := results[currentID]; ok {
				results[currentID] = append(results[currentID], manifestStatus.StatusFeedbacks.Values...)
			}
		}
	}

	return results, nil
}

// ListAddonManifestWorks lists all manifestworks for a given addon in a managed cluster namespace.
func ListAddonManifestWorks(ctx context.Context, kubeClient client.Client, clusterName, addonName string) (*workv1.ManifestWorkList, error) {
	workList := &workv1.ManifestWorkList{}
	listOpts := []client.ListOption{
		client.InNamespace(clusterName),
		client.MatchingLabels{
			addonapiv1alpha1.AddonLabelKey: addonName,
		},
	}
	err := kubeClient.List(ctx, workList, listOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list manifestworks in namespace %s: %w", clusterName, err)
	}

	return workList, nil
}

// FilterFeedbackValuesByName is a helper to filter a slice of FeedbackValue by name.
func FilterFeedbackValuesByName(values []workv1.FeedbackValue, name string) []workv1.FeedbackValue {
	var filtered []workv1.FeedbackValue
	for _, v := range values {
		if v.Name == name {
			filtered = append(filtered, v)
		}
	}
	return filtered
}
