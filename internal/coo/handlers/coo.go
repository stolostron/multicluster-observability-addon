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
