package resource

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"k8s.io/apimachinery/pkg/util/validation"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

const maxAgentNameLength = validation.DNS1123LabelMaxLength

// agentIndex classifies existing recognized PrometheusAgent resources for a single collector
// component (platform or user-workload) by ownership and by targeted placement:
//   - "CMAO-owned" means controlled by the CMAO and not modified by any other field manager
//     (see common.IsFullyOwnedByCMAO), so MCOA can safely replace or delete it.
//   - "User-owned" means either created by the user or a CMAO default agent subsequently modified
//     by the user, which MCOA must preserve.
type agentIndex struct {
	appName                    string
	staleCMAOAgentsByName      map[string]*cooprometheusv1alpha1.PrometheusAgent
	inUseAgentNames            map[string]struct{}
	placementNameCounts        map[string]int
	cmaoOwnedAgentsByPlacement map[addonv1beta1.PlacementRef][]*cooprometheusv1alpha1.PrometheusAgent
	userCoveredPlacements      map[addonv1beta1.PlacementRef]struct{}
}

func newAgentIndex(cmao *addonv1beta1.ClusterManagementAddOn, items []cooprometheusv1alpha1.PrometheusAgent, isUWL bool) (agentIndex, error) {
	appName := componentName(isUWL)

	idx := agentIndex{
		appName:                    appName,
		staleCMAOAgentsByName:      make(map[string]*cooprometheusv1alpha1.PrometheusAgent),
		inUseAgentNames:            make(map[string]struct{}),
		placementNameCounts:        make(map[string]int),
		cmaoOwnedAgentsByPlacement: make(map[addonv1beta1.PlacementRef][]*cooprometheusv1alpha1.PrometheusAgent),
		userCoveredPlacements:      make(map[addonv1beta1.PlacementRef]struct{}),
	}

	activePlacements := make(map[addonv1beta1.PlacementRef]struct{}, len(cmao.Spec.InstallStrategy.Placements))
	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		if _, seen := activePlacements[placement.PlacementRef]; seen {
			continue
		}
		activePlacements[placement.PlacementRef] = struct{}{}
		idx.placementNameCounts[placement.Name]++
	}

	userOwnedAgentNames := make(map[string]struct{})

	for i := range items {
		agent := &items[i]
		if agent.Labels[addoncfg.ComponentK8sLabelKey] != appName || !isRecognizedAgent(agent, cmao) {
			idx.inUseAgentNames[agent.Name] = struct{}{}
			continue
		}

		isCMAO := common.IsFullyOwnedByCMAO(agent, cmao)
		if !isCMAO {
			userOwnedAgentNames[agent.Name] = struct{}{}
		}

		refs, err := generatePlacementRefs(agent.Annotations[addoncfg.PlacementAnnotationKey])
		if err != nil {
			return agentIndex{}, fmt.Errorf("failed to generate placement refs for agent %s: %w", agent.Name, err)
		}
		targetsActivePlacement := false
		for _, ref := range refs {
			if _, active := activePlacements[ref]; active {
				targetsActivePlacement = true
			}
			if isCMAO {
				idx.cmaoOwnedAgentsByPlacement[ref] = append(idx.cmaoOwnedAgentsByPlacement[ref], agent)
			} else {
				idx.userCoveredPlacements[ref] = struct{}{}
			}
		}

		if isCMAO && !targetsActivePlacement {
			idx.staleCMAOAgentsByName[agent.Name] = agent
		} else {
			idx.inUseAgentNames[agent.Name] = struct{}{}
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

// defaultAgentName returns the default PrometheusAgent name for the given placement.
// When the placement name is unique in the CMAO and not already in use by an active or user-owned
// agent, it returns "mcoa-default-<app>-<name>". When multiple placements share the same name across
// namespaces (or the base name is already in use), it disambiguates with the placement namespace
// ("mcoa-default-<app>-<namespace>-<name>") while keeping the name bounded to maxAgentNameLength (63).
func (idx agentIndex) defaultAgentName(placement addonv1beta1.PlacementRef) string {
	if idx.placementNameCounts[placement.Name] <= 1 {
		baseName := boundAgentName(makeAgentName(idx.appName, placement.Name), placement)
		if _, inUse := idx.inUseAgentNames[baseName]; !inUse {
			return baseName
		}
	}

	nsName := boundAgentName(makeAgentName(idx.appName, placement.Namespace+"-"+placement.Name), placement)
	if _, inUse := idx.inUseAgentNames[nsName]; !inUse {
		return nsName
	}

	return hashAgentName(makeAgentName(idx.appName, placement.Namespace+"-"+placement.Name), placement)
}

func boundAgentName(name string, placement addonv1beta1.PlacementRef) string {
	if len(name) <= maxAgentNameLength {
		return name
	}
	return hashAgentName(name, placement)
}

func hashAgentName(name string, placement addonv1beta1.PlacementRef) string {
	sum := sha256.Sum256([]byte(placement.Namespace + "/" + placement.Name))
	suffix := hex.EncodeToString(sum[:])[:8]
	maxPrefixLen := min(len(name), maxAgentNameLength-1-len(suffix))
	prefix := strings.TrimRight(name[:maxPrefixLen], "-.")
	return prefix + "-" + suffix
}

func isPrometheusAgentConfig(cfg addonv1beta1.AddOnConfig) bool {
	return cfg.Group == cooprometheusv1alpha1.SchemeGroupVersion.Group &&
		cfg.Resource == cooprometheusv1alpha1.PrometheusAgentName &&
		cfg.Namespace == config.HubInstallNamespace
}
