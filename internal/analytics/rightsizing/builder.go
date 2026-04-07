// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

// FormatYAML converts a Go data structure to a YAML-formatted string.
// Accepts RSPrometheusRuleConfig, Placement, or any YAML-serializable struct.
func FormatYAML(data interface{}) string {
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return ""
	}
	return string(yamlData)
}

// GetDefaultRSPlacement creates a default placement configuration for right-sizing.
// Empty predicates + tolerations for unreachable/unavailable = selects ALL clusters.
// Matches MCO's rs-utility/placement.go GetDefaultRSPlacement().
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
		return "", fmt.Errorf("only one of inclusion or exclusion criteria allowed for namespacefiltercriteria")
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
			return "", fmt.Errorf("only one of inclusion or exclusion allowed for label_env")
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

// ParseConfigMapData parses configmap data into RSConfigMapData
func ParseConfigMapData(data map[string]string) (RSConfigMapData, error) {
	var configData RSConfigMapData

	if prometheusRuleConfigYAML, ok := data["prometheusRuleConfig"]; ok {
		if err := yaml.Unmarshal([]byte(prometheusRuleConfigYAML), &configData.PrometheusRuleConfig); err != nil {
			return configData, fmt.Errorf("failed to unmarshal prometheusRuleConfig: %v", err)
		}
	}

	if placementYAML, ok := data["placementConfiguration"]; ok {
		if err := yaml.Unmarshal([]byte(placementYAML), &configData.PlacementConfiguration); err != nil {
			return configData, fmt.Errorf("failed to unmarshal placementConfiguration: %v", err)
		}
	} else {
		// Default placement selects all clusters
		configData.PlacementConfiguration = GetDefaultRSPlacement()
	}

	return configData, nil
}

// GetDefaultNamespaceConfigData returns default config data for namespace right-sizing
func GetDefaultNamespaceConfigData() map[string]string {
	ruleConfig := GetDefaultRSPrometheusRuleConfig()
	placement := GetDefaultRSPlacement()
	return map[string]string{
		"prometheusRuleConfig":   FormatYAML(ruleConfig),
		"placementConfiguration": FormatYAML(placement),
	}
}

// GetDefaultVirtualizationConfigData returns default config data for virtualization right-sizing
func GetDefaultVirtualizationConfigData() map[string]string {
	ruleConfig := GetDefaultRSPrometheusRuleConfig()
	placement := GetDefaultRSPlacement()
	return map[string]string{
		"prometheusRuleConfig":   FormatYAML(ruleConfig),
		"placementConfiguration": FormatYAML(placement),
	}
}
