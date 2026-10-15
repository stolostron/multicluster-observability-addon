package resource

import (
	"fmt"

	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

// agentIndex classifies existing recognized PrometheusAgent resources for a single collector
// component (platform or user-workload) by ownership and by targeted placement:
//   - "CMAO-owned" means controlled by the CMAO and not modified by any other field manager
//     (see common.IsFullyOwnedByCMAO), so MCOA can safely replace or delete it.
//   - "User-owned" means either created by the user or a CMAO default agent subsequently modified
//     by the user, which MCOA must preserve.
type agentIndex struct {
	appName                    string
	cmaoOwnedAgentsByName      map[string]*cooprometheusv1alpha1.PrometheusAgent
	cmaoOwnedAgentsByPlacement map[addonv1beta1.PlacementRef][]*cooprometheusv1alpha1.PrometheusAgent
	userCoveredPlacements      map[addonv1beta1.PlacementRef]struct{}
}

func newAgentIndex(cmao *addonv1beta1.ClusterManagementAddOn, items []cooprometheusv1alpha1.PrometheusAgent, isUWL bool) (agentIndex, error) {
	appName := componentName(isUWL)

	idx := agentIndex{
		appName:                    appName,
		cmaoOwnedAgentsByName:      make(map[string]*cooprometheusv1alpha1.PrometheusAgent),
		cmaoOwnedAgentsByPlacement: make(map[addonv1beta1.PlacementRef][]*cooprometheusv1alpha1.PrometheusAgent),
		userCoveredPlacements:      make(map[addonv1beta1.PlacementRef]struct{}),
	}

	userOwnedAgentNames := make(map[string]struct{})

	for i := range items {
		agent := &items[i]
		if agent.Labels[addoncfg.ComponentK8sLabelKey] != appName {
			continue
		}
		if !isRecognizedAgent(agent, cmao) {
			continue
		}

		isCMAO := common.IsFullyOwnedByCMAO(agent, cmao)
		if isCMAO {
			idx.cmaoOwnedAgentsByName[agent.Name] = agent
		} else {
			userOwnedAgentNames[agent.Name] = struct{}{}
		}

		refs, err := generatePlacementRefs(agent.Annotations[addoncfg.PlacementAnnotationKey])
		if err != nil {
			return agentIndex{}, fmt.Errorf("failed to generate placement refs for agent %s: %w", agent.Name, err)
		}
		for _, ref := range refs {
			if isCMAO {
				idx.cmaoOwnedAgentsByPlacement[ref] = append(idx.cmaoOwnedAgentsByPlacement[ref], agent)
			} else {
				idx.userCoveredPlacements[ref] = struct{}{}
			}
		}
	}

	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		ref := placement.PlacementRef
		for _, cfg := range placement.Configs {
			if !isPrometheusAgentConfig(cfg) {
				continue
			}
			if _, isUser := userOwnedAgentNames[cfg.Name]; isUser {
				idx.userCoveredPlacements[ref] = struct{}{}
			}
		}
	}

	return idx, nil
}

// cmaoOwnedAgentsForPlacement returns all unmodified CMAO-owned PrometheusAgents (see
// common.IsFullyOwnedByCMAO) referencing the given placement via the placement annotation.
func (idx agentIndex) cmaoOwnedAgentsForPlacement(placement addonv1beta1.PlacementRef) []*cooprometheusv1alpha1.PrometheusAgent {
	return idx.cmaoOwnedAgentsByPlacement[placement]
}

// isPlacementCoveredByUser reports whether a user-owned or user-configured PrometheusAgent for the
// indexed component covers the given placement (either via placement annotation or placement.Configs).
func (idx agentIndex) isPlacementCoveredByUser(placement addonv1beta1.PlacementRef) bool {
	_, ok := idx.userCoveredPlacements[placement]
	return ok
}

func isPrometheusAgentConfig(cfg addonv1beta1.AddOnConfig) bool {
	return cfg.Group == cooprometheusv1alpha1.SchemeGroupVersion.Group &&
		cfg.Resource == cooprometheusv1alpha1.PrometheusAgentName &&
		cfg.Namespace == config.HubInstallNamespace
}
