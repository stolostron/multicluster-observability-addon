package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// CanManageCOOOnTheHub determines whether MCOA is responsible for managing the COO
// installation on the hub. If COO was installed by an external party (e.g. an admin
// installed it in openshift-operators or without MCOA's release label), MCOA will not
// install or modify its own Subscription to avoid OLM conflicts.
// Note: This does not evaluate MCOA feature flags; feature enablement is evaluated separately
// by EvaluateHubStackFeatures.
func CanManageCOOOnTheHub(ctx context.Context, k8s client.Reader, logger logr.Logger) (bool, error) {
	cooSub, err := findCOOSubscription(ctx, k8s)
	if err != nil {
		return false, fmt.Errorf("failed to get cluster observability operator subscription: %w", err)
	}

	// Missing subscription means the operator is not installed anywhere on the hub
	if cooSub == nil {
		return true, nil
	}

	// Wrong subscription channel means the operator is an error
	if cooSub.Spec == nil || cooSub.Spec.Channel != addoncfg.CooSubscriptionChannel {
		return false, addoncfg.ErrInvalidSubscriptionChannel
	}

	// If the subscription is in MCOA's namespace and has our release label, install the operator (keep managing it)
	if cooSub.Namespace == addoncfg.CooSubscriptionNamespace {
		if value, exists := cooSub.Labels[addoncfg.ReleaseLabelKey]; exists && value == addoncfg.Name {
			return true, nil
		}
	}

	// COO is installed externally (e.g. in openshift-operators); do not install a competing one
	logger.V(2).Info("COO installed externally on the hub, MCOA will not install its own subscription",
		"namespace", cooSub.Namespace, "name", cooSub.Name)
	return false, nil
}

func findCOOSubscription(ctx context.Context, k8s client.Reader) (*operatorv1alpha1.Subscription, error) {
	subList := &operatorv1alpha1.SubscriptionList{}
	if err := k8s.List(ctx, subList); err != nil {
		return nil, err
	}

	var candidate *operatorv1alpha1.Subscription
	for i := range subList.Items {
		sub := &subList.Items[i]
		if isCooSubscription(sub) {
			// If MCOA's own managed subscription in CooSubscriptionNamespace exists, prefer it
			if sub.Namespace == addoncfg.CooSubscriptionNamespace {
				return sub, nil
			}
			if candidate == nil {
				candidate = sub
			}
		}
	}
	return candidate, nil
}

const thanosRulerCustomRulesName = "thanos-ruler-custom-rules"

// HasCardinalityRules reports whether the cardinality recording rules ConfigMap is present.
// Callers must only invoke this for hub clusters, since the ConfigMap only exists there.
func HasCardinalityRules(ctx context.Context, k8s client.Reader) bool {
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

func CardinalityRulesConfigMapPredicate() predicate.Predicate {
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

func isCooSubscription(obj client.Object) bool {
	if obj.GetName() == addoncfg.CooSubscriptionName {
		return true
	}
	if sub, ok := obj.(*operatorv1alpha1.Subscription); ok && sub.Spec != nil {
		return sub.Spec.Package == addoncfg.CooSubscriptionName
	}
	return false
}

// CooSubscriptionPredicate filters events for the cluster-observability-operator Subscription
// across any namespace (handling both MCOA-managed and pre-existing admin installations).
// It ignores status updates to avoid unnecessary reconciliations when OLM modifies subscription status fields.
func CooSubscriptionPredicate() predicate.Predicate {
	return predicate.And(
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return isCooSubscription(obj)
		}),
		predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.LabelChangedPredicate{},
		),
	)
}
