package common

import (
	"context"
	"fmt"
	"strings"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const crdResourceName = "customresourcedefinitions"

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

	for _, work := range workList.Items {
		for i, manifestStatus := range work.Status.ResourceStatus.Manifests {
			currentID := workv1.ResourceIdentifier{
				Group:     manifestStatus.ResourceMeta.Group,
				Resource:  manifestStatus.ResourceMeta.Resource,
				Name:      manifestStatus.ResourceMeta.Name,
				Namespace: manifestStatus.ResourceMeta.Namespace,
			}
			if currentID == resourceID {
				return &work.Status.ResourceStatus.Manifests[i], nil
			}
		}
	}

	return nil, nil
}

// IsCOOSubscribedOnSpoke reports whether the Cluster Observability Operator is already
// present on the given managed cluster. Since the hub has no direct API access to the
// spoke, this relies on status feedback the work agent reports back for the
// alertmanagers.monitoring.rhobs CRD (owned by COO), specifically the "olm.managed" label
// that OLM stamps on CRDs it installed via a Subscription/CSV.
//
// hasFeedback is false when the work agent hasn't reported anything for that CRD yet, e.g.
// on the very first reconcile for a cluster before any ManifestWork exists or before it has
// been picked up on the spoke. Callers should treat that as "unknown" rather than "not
// installed", since the CRD may in fact already exist and simply hasn't been observed yet.
//
// Note this is deliberately based on whether the resource itself was observed (its Available
// condition), not on whether the "olm.managed" JSONPath produced a value. A CRD that exists but
// carries no "olm.managed" label (e.g. MCOA's own placeholder) yields zero feedback values, which
// is a confirmed "not OLM-managed" signal, not an "unknown" one; conflating the two would leave a
// cluster stuck deferring forever after COO is fully removed instead of self-healing.
func IsCOOSubscribedOnSpoke(ctx context.Context, kubeClient client.Client, clusterName, addonName string) (subscribed bool, hasFeedback bool, err error) {
	crdID := workv1.ResourceIdentifier{
		Group:    apiextensionsv1.GroupName,
		Resource: crdResourceName,
		Name:     mconfig.AlertmanagerCRDName,
	}

	condition, err := GetManifestCondition(ctx, kubeClient, clusterName, addonName, crdID)
	if err != nil {
		return false, false, fmt.Errorf("failed to get manifest condition for %s: %w", crdID.Name, err)
	}

	if condition == nil || !meta.IsStatusConditionTrue(condition.Conditions, workv1.WorkAvailable) {
		return false, false, nil
	}

	for _, v := range FilterFeedbackValuesByName(condition.StatusFeedbacks.Values, addoncfg.IsOLMManagedFeedbackName) {
		if v.Value.String != nil && strings.ToLower(*v.Value.String) == "true" {
			return true, true, nil
		}
	}

	return false, true, nil
}
