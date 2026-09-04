package common

import (
	"context"
	"fmt"
	"strings"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"k8s.io/apimachinery/pkg/api/meta"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
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
			addonapiv1beta1.AddonLabelKey: addonName,
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

// GetManifestCondition returns the ManifestCondition reported by the work agent for a given
// resource, across all ManifestWorks for the addon on the given cluster. It returns nil if no
// ManifestWork has reported status for that resource yet.
func GetManifestCondition(ctx context.Context, kubeClient client.Client, clusterName, addonName string, resourceID workv1.ResourceIdentifier) (*workv1.ManifestCondition, error) {
	workList, err := ListAddonManifestWorks(ctx, kubeClient, clusterName, addonName)
	if err != nil {
		return nil, err
	}

	for w := range workList.Items {
		for i, manifestStatus := range workList.Items[w].Status.ResourceStatus.Manifests {
			currentID := workv1.ResourceIdentifier{
				Group:     manifestStatus.ResourceMeta.Group,
				Resource:  manifestStatus.ResourceMeta.Resource,
				Name:      manifestStatus.ResourceMeta.Name,
				Namespace: manifestStatus.ResourceMeta.Namespace,
			}
			if matchesResourceIdentifier(currentID, resourceID) {
				return &workList.Items[w].Status.ResourceStatus.Manifests[i], nil
			}
		}
	}

	return nil, nil
}

// IsCOOSubscribedOnSpoke reports whether the Cluster Observability Operator is already
// present on the given managed cluster AND was installed by someone other than MCOA.
// This relies on a coo-status ConfigMap written by the endpoint-monitoring-operator on
// the spoke, which reports both installed status and who manages the Subscription
// (mcoa vs external).
//
// Returns subscribed=true only when COO is installed by an external party (admin).
// When COO is installed by MCOA itself, returns subscribed=false so MCOA keeps managing it.
// hasFeedback is false when the endpoint-monitoring-operator hasn't written the ConfigMap yet.
func IsCOOSubscribedOnSpoke(ctx context.Context, kubeClient client.Client, clusterName, addonName string) (subscribed bool, hasFeedback bool, err error) {
	cmID := workv1.ResourceIdentifier{
		Group:    "",
		Resource: "configmaps",
		Name:     addoncfg.CooStatusConfigMapName,
	}

	condition, err := GetManifestCondition(ctx, kubeClient, clusterName, addonName, cmID)
	if err != nil {
		return false, false, fmt.Errorf("failed to get manifest condition for %s: %w", cmID.Name, err)
	}

	if condition == nil || !meta.IsStatusConditionTrue(condition.Conditions, workv1.WorkAvailable) {
		return false, false, nil
	}

	var installed bool
	var managedBy string
	for _, v := range FilterFeedbackValuesByName(condition.StatusFeedbacks.Values, addoncfg.CooStatusInstalledFeedbackName) {
		if v.Value.String != nil && strings.ToLower(*v.Value.String) == "true" {
			installed = true
		}
	}
	for _, v := range FilterFeedbackValuesByName(condition.StatusFeedbacks.Values, addoncfg.CooStatusManagedByFeedbackName) {
		if v.Value.String != nil {
			managedBy = *v.Value.String
		}
	}

	if installed && managedBy != "mcoa" {
		return true, true, nil
	}

	return false, true, nil
}

// matchesResourceIdentifier compares two ResourceIdentifiers. If the target
// namespace is empty, namespace is ignored (wildcard match).
func matchesResourceIdentifier(current, target workv1.ResourceIdentifier) bool {
	if current.Group != target.Group || current.Resource != target.Resource || current.Name != target.Name {
		return false
	}
	if target.Namespace != "" && current.Namespace != target.Namespace {
		return false
	}
	return true
}
