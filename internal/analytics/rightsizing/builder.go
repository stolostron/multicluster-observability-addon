package rightsizing

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	sigYaml "sigs.k8s.io/yaml"
)

var (
	errMutuallyExclusiveNamespaceFilter = errors.New("only one of inclusion or exclusion criteria allowed for namespacefiltercriteria")
	errMutuallyExclusiveLabelFilter     = errors.New("only one of inclusion or exclusion allowed for label_env")
	errUnmarshalPrometheusRuleConfig    = errors.New("failed to unmarshal prometheusRuleConfig")
	errUnmarshalPlacementConfig         = errors.New("failed to unmarshal placementConfiguration")
)

// FormatJSON marshals a Go data structure to a JSON string for ConfigMap storage.
func FormatJSON(data any) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(jsonData)
}

// GetDefaultRSPlacement creates a default placement configuration for right-sizing.
// Empty predicates = selects ALL clusters (evaluated in-memory during Build).
// Tolerations are retained for backward compatibility with existing ConfigMaps.
func GetDefaultRSPlacement() clusterv1beta1.Placement {
	return clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{},
			Tolerations: []clusterv1beta1.Toleration{
				{
					Key:      "cluster.open-cluster-management.io/unreachable",
					Operator: clusterv1beta1.TolerationOpExists,
				},
				{
					Key:      "cluster.open-cluster-management.io/unavailable",
					Operator: clusterv1beta1.TolerationOpExists,
				},
			},
		},
	}
}

// GetDefaultRSPrometheusRuleConfig creates a default prometheus rule configuration for right-sizing
func GetDefaultRSPrometheusRuleConfig() RSPrometheusRuleConfig {
	var ruleConfig RSPrometheusRuleConfig
	ruleConfig.NamespaceFilterCriteria.ExclusionCriteria = []string{"openshift.*"}
	ruleConfig.RecommendationPercentage = DefaultRecommendationPercentage
	return ruleConfig
}

// BuildNamespaceFilter creates a namespace filter string for Prometheus queries
func BuildNamespaceFilter(nsConfig RSPrometheusRuleConfig) (string, error) {
	ns := nsConfig.NamespaceFilterCriteria
	if len(ns.InclusionCriteria) > 0 && len(ns.ExclusionCriteria) > 0 {
		return "", errMutuallyExclusiveNamespaceFilter
	}
	if len(ns.InclusionCriteria) > 0 {
		return fmt.Sprintf(`namespace=~"%s"`, strings.Join(ns.InclusionCriteria, "|")), nil
	}
	if len(ns.ExclusionCriteria) > 0 {
		return fmt.Sprintf(`namespace!~"%s"`, strings.Join(ns.ExclusionCriteria, "|")), nil
	}
	return `namespace!=""`, nil
}

// BuildLabelJoin creates a label join string for Prometheus queries
func BuildLabelJoin(labelFilters []RSLabelFilter) (string, error) {
	for _, l := range labelFilters {
		if l.LabelName != "label_env" {
			continue
		}
		if len(l.InclusionCriteria) > 0 && len(l.ExclusionCriteria) > 0 {
			return "", errMutuallyExclusiveLabelFilter
		}
		var selector string
		switch {
		case len(l.InclusionCriteria) > 0:
			selector = fmt.Sprintf(`kube_namespace_labels{label_env=~"%s"}`, strings.Join(l.InclusionCriteria, "|"))
		case len(l.ExclusionCriteria) > 0:
			selector = fmt.Sprintf(`kube_namespace_labels{label_env!~"%s"}`, strings.Join(l.ExclusionCriteria, "|"))
		default:
			continue
		}
		return fmt.Sprintf(`* on (namespace) group_left() (%s or kube_namespace_labels{label_env=""})`, selector), nil
	}
	return "", nil
}

// ParseConfigMapData parses ConfigMap data into RSConfigMapData.
// Uses sigs.k8s.io/yaml for unmarshaling because MCO writes ConfigMap values
// in YAML format while MCOA writes JSON. sigs.k8s.io/yaml handles both
// transparently and respects json struct tags.
//
// Placement parsing is best-effort: MCO serializes Placement using
// gopkg.in/yaml.v2 which writes intstr.IntOrString fields as objects
// ({type,intval,strval}) instead of scalars, causing unmarshal failures.
// Since MCO always writes empty predicates and real placement comes from
// dedicated placement ConfigMaps, we fall back to the default on error.
func ParseConfigMapData(data map[string]string) (RSConfigMapData, error) {
	var configData RSConfigMapData

	if ruleConfigRaw, ok := data["prometheusRuleConfig"]; ok {
		if err := sigYaml.Unmarshal([]byte(ruleConfigRaw), &configData.PrometheusRuleConfig); err != nil {
			return configData, fmt.Errorf("%w: %w", errUnmarshalPrometheusRuleConfig, err)
		}
	}

	configData.PlacementConfiguration = GetDefaultRSPlacement()
	if placementRaw, ok := data["placementConfiguration"]; ok {
		var placement clusterv1beta1.Placement
		if err := sigYaml.Unmarshal([]byte(placementRaw), &placement); err == nil {
			configData.PlacementConfiguration = placement
		}
	}

	return configData, nil
}

// ParsePlacementConfigMap parses a dedicated placement ConfigMap.
// The ConfigMap stores the Placement in a "placementConfiguration" key
// (JSON or YAML format). Returns the Placement and true if found, or
// an empty Placement and false if the key is missing.
func ParsePlacementConfigMap(data map[string]string) (clusterv1beta1.Placement, bool, error) {
	raw, ok := data["placementConfiguration"]
	if !ok {
		return clusterv1beta1.Placement{}, false, nil
	}
	var placement clusterv1beta1.Placement
	if err := sigYaml.Unmarshal([]byte(raw), &placement); err != nil {
		return placement, false, fmt.Errorf("%w: %w", errUnmarshalPlacementConfig, err)
	}
	return placement, true, nil
}

// GetDefaultNamespaceConfigData returns default config data for namespace right-sizing
func GetDefaultNamespaceConfigData() map[string]string {
	ruleConfig := GetDefaultRSPrometheusRuleConfig()
	placement := GetDefaultRSPlacement()
	return map[string]string{
		"prometheusRuleConfig":   FormatJSON(ruleConfig),
		"placementConfiguration": FormatJSON(placement),
	}
}

// GetDefaultVirtualizationConfigData returns default config data for virtualization right-sizing
func GetDefaultVirtualizationConfigData() map[string]string {
	ruleConfig := GetDefaultRSPrometheusRuleConfig()
	placement := GetDefaultRSPlacement()
	return map[string]string{
		"prometheusRuleConfig":   FormatJSON(ruleConfig),
		"placementConfiguration": FormatJSON(placement),
	}
}

// GetDefaultWorkloadConfigData returns default config data for workload-pod right-sizing
func GetDefaultWorkloadConfigData() map[string]string {
	ruleConfig := GetDefaultRSPrometheusRuleConfig()
	placement := GetDefaultRSPlacement()
	return map[string]string{
		"prometheusRuleConfig":   FormatJSON(ruleConfig),
		"placementConfiguration": FormatJSON(placement),
	}
}
