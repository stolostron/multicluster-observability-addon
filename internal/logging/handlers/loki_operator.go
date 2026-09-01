package handlers

import (
	"context"
	"fmt"

	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileLokiOperator installs Loki Operator on the hub cluster via OLM when enabled, applying
// the Namespace, OperatorGroup and Subscription objects directly with Server-Side Apply against
// the hub's own API server.
//
// This is deliberately separate from BuildDefaultStackResources: installing the operator is not
// funneled through that function's Objects/DefaultConfig pipeline (which exists to build the
// logging-stack CRs and their addon config references). Instead it's applied here as its own
// step, independent of whether the default logging stack (platform.DefaultStack) is enabled --
// turning on the mcoa-loki-operator annotation gets the operator (and its CRDs) running on the
// hub so they're already Established by the time DefaultStack is turned on. This mirrors the
// existing mcoa-thanos-operator pattern, which also installs its operator without requiring any
// specific instance CR to be requested.
func ReconcileLokiOperator(ctx context.Context, k8s client.Client, cmao *addonv1beta1.ClusterManagementAddOn, enabled bool) error {
	if !enabled {
		return nil
	}

	for _, obj := range manifests.BuildLokiOperatorResources() {
		if err := common.ServerSideApply(ctx, k8s, obj, cmao); err != nil {
			return fmt.Errorf("failed to apply loki operator resource %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}
