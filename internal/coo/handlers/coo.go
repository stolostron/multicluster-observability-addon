package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func InstallOfCOOOnTheHubIsNeeded(ctx context.Context, k8s client.Client, logger logr.Logger, isHub bool) (bool, error) {
	// Currently, the InstallCOO option is only relevant for hub clusters
	// since we don't have k8s clients for the spokes
	if !isHub {
		return false, nil
	}

	cooSub := &operatorv1alpha1.Subscription{}
	key := client.ObjectKey{Name: addoncfg.CooSubscriptionName, Namespace: addoncfg.CooSubscriptionNamespace}
	if err := k8s.Get(ctx, key, cooSub, &client.GetOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return true, fmt.Errorf("failed to get cluster observability operator subscription: %w", err)
	}

	// Missing subscription means the operator is not installed
	if cooSub.Name == "" {
		return true, nil
	}

	// Wrong subscription channel means the operator is an error
	if cooSub.Spec.Channel != addoncfg.CooSubscriptionChannel {
		return false, addoncfg.ErrInvalidSubscriptionChannel
	}

	// If the subscription has our release label, install the operator
	if value, exists := cooSub.Labels["release"]; exists && value == "multicluster-observability-addon" {
		return true, nil
	}

	return false, nil
}

// InstallOfCOOOnSpokeIsNeeded decides, from the hub, whether MCOA should install and
// manage its own Cluster Observability Operator subscription on a spoke cluster.
//
// The hub controller has no direct API access to spokes, so the decision can only rely on
// hub-local information:
//
//  1. Whether a previous reconcile already committed to installing COO on this cluster, by
//     checking if our own Subscription manifest is already part of the ManifestWork spec we
//     last rendered for it. Once that's true we keep managing it forever (sticky decision):
//     otherwise, as soon as our own Subscription succeeds, the CRDs it creates would start
//     reporting as "already installed" (see below) and we'd conclude COO isn't ours to
//     manage anymore, removing (and thus deleting) the very resources we created.
//  2. Otherwise, status feedback reported by the work agent for the
//     alertmanagers.monitoring.rhobs CRD (owned by COO): if it's already OLM-managed, COO
//     was installed by someone else before MCOA ever got a chance to act, so MCOA must
//     leave it alone rather than create a competing Subscription/OperatorGroup/Namespace.
//
// On the very first reconcile(s) for a cluster no status feedback is available yet, so this
// conservatively defers the decision (returns false) rather than risk racing a pre-existing
// installation. Once feedback arrives, the decision above is re-evaluated and, if COO isn't
// present yet, MCOA commits to installing and managing it from then on.
func InstallOfCOOOnSpokeIsNeeded(ctx context.Context, k8s client.Client, logger logr.Logger, clusterName string) (bool, error) {
	committed, err := hasCommittedToInstallCOOSubscription(ctx, k8s, clusterName)
	if err != nil {
		return false, fmt.Errorf("failed to check previous COO install commitment for cluster %s: %w", clusterName, err)
	}
	if committed {
		return true, nil
	}

	subscribed, hasFeedback, err := common.IsCOOSubscribedOnSpoke(ctx, k8s, clusterName, addoncfg.Name)
	if err != nil {
		return false, fmt.Errorf("failed to check if coo is subscribed on cluster %s: %w", clusterName, err)
	}
	if !hasFeedback {
		logger.V(2).Info("no COO status feedback yet for cluster, deferring COO install decision", "clusterName", clusterName)
		return false, nil
	}
	if subscribed {
		logger.V(2).Info("COO already present on cluster, MCOA will not install its own subscription", "clusterName", clusterName)
		return false, nil
	}

	return true, nil
}

// hasCommittedToInstallCOOSubscription checks whether the COO Subscription manifest is
// already part of the ManifestWork(s) MCOA previously rendered for this cluster.
func hasCommittedToInstallCOOSubscription(ctx context.Context, k8s client.Client, clusterName string) (bool, error) {
	workList, err := common.ListAddonManifestWorks(ctx, k8s, clusterName, addoncfg.Name)
	if err != nil {
		return false, fmt.Errorf("failed to list manifestworks for cluster %s: %w", clusterName, err)
	}

	for _, work := range workList.Items {
		for _, manifest := range work.Spec.Workload.Manifests {
			u, err := manifestToUnstructured(manifest)
			if err != nil || u == nil {
				continue
			}
			if u.GetKind() == "Subscription" &&
				u.GetName() == addoncfg.CooSubscriptionName &&
				u.GetNamespace() == addoncfg.CooSubscriptionNamespace {
				return true, nil
			}
		}
	}

	return false, nil
}

// manifestToUnstructured decodes a ManifestWork manifest entry, regardless of whether it
// was populated as a typed/unstructured object (e.g. in unit tests building the object
// in-memory) or as raw JSON bytes (as returned by a real API server).
func manifestToUnstructured(m workv1.Manifest) (*unstructured.Unstructured, error) {
	if u, ok := m.Object.(*unstructured.Unstructured); ok {
		return u, nil
	}
	if m.Object != nil {
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(m.Object)
		if err != nil {
			return nil, fmt.Errorf("failed to convert manifest object to unstructured: %w", err)
		}
		return &unstructured.Unstructured{Object: content}, nil
	}
	if len(m.Raw) > 0 {
		u := &unstructured.Unstructured{}
		if _, _, err := unstructured.UnstructuredJSONScheme.Decode(m.Raw, nil, u); err != nil {
			return nil, fmt.Errorf("failed to decode manifest raw bytes: %w", err)
		}
		return u, nil
	}
	return nil, nil
}

const thanosRulerCustomRulesName = "thanos-ruler-custom-rules"

func HasCardinalityRules(ctx context.Context, k8s client.Client, isHub bool) bool {
	if !isHub {
		return false
	}

	cm, err := common.GetConfigMap(ctx, k8s, addoncfg.InstallNamespace, thanosRulerCustomRulesName)
	if err != nil {
		return false
	}

	rulesData, ok := cm.Data["custom_rules.yaml"]
	if !ok {
		return false
	}

	return strings.Contains(rulesData, "cluster:cardinality")
}

func CardinalityRulesConfigMapPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isCardinalityRulesConfigMap(e.Object.GetNamespace(), e.Object.GetName())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isCardinalityRulesConfigMap(e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isCardinalityRulesConfigMap(e.Object.GetNamespace(), e.Object.GetName())
		},
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func isCardinalityRulesConfigMap(namespace, name string) bool {
	return namespace == addoncfg.InstallNamespace && name == thanosRulerCustomRulesName
}
