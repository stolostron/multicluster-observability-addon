package rightsizing

import (
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

// RS resource labels — must match MCO's rsutility.RSLabels() so that MCO can
// discover and manage these resources during mode switches and cleanup.
// In the future when only MCOA mode is supported, these can use MCOA-specific labels.
const (
	RSManagedByLabel = "observability.open-cluster-management.io/managed-by"
	RSManagedByValue = "analytics-rightsizing"
)

// RSLabels returns the standard labels applied to all right-sizing hub resources.
func RSLabels() map[string]string {
	return map[string]string{RSManagedByLabel: RSManagedByValue}
}

// Common constants
const (
	DefaultRecommendationPercentage = 110
	MonitoringNamespace             = "openshift-monitoring"

	// PlacementNamespace is where RS Placements are created.
	// Uses global-set namespace which already has a ManagedClusterSetBinding,
	// enabling Placements to select clusters from the default ManagedClusterSet.
	// ReconcileRSResources cleans up RS Placements in other namespaces to handle
	// mode switches where MCO used a custom NamespaceBinding.
	PlacementNamespace = "open-cluster-management-global-set"

	// Namespace right-sizing constants
	NamespacePrometheusRuleName = "acm-rs-namespace-prometheus-rules"
	NamespaceConfigMapName      = "rs-namespace-config"
	NamespacePlacementName      = "rs-placement"

	// Virtualization right-sizing constants
	VirtualizationPrometheusRuleName = "acm-rs-virt-prometheus-rules"
	VirtualizationConfigMapName      = "rs-virt-config"
	VirtualizationPlacementName      = "rs-virt-placement"

	// PlacementDecisionLabel is the standard OCM label on PlacementDecisions referencing their Placement
	PlacementDecisionLabel = "cluster.open-cluster-management.io/placement"
)

// RSLabelFilter represents label filtering criteria for right-sizing
type RSLabelFilter struct {
	LabelName         string   `yaml:"labelName" json:"labelName"`
	InclusionCriteria []string `yaml:"inclusionCriteria,omitempty" json:"inclusionCriteria,omitempty"`
	ExclusionCriteria []string `yaml:"exclusionCriteria,omitempty" json:"exclusionCriteria,omitempty"`
}

// RSPrometheusRuleConfig represents the Prometheus rule configuration for right-sizing
type RSPrometheusRuleConfig struct {
	NamespaceFilterCriteria struct {
		InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
		ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
	} `yaml:"namespaceFilterCriteria" json:"namespaceFilterCriteria"`
	LabelFilterCriteria      []RSLabelFilter `yaml:"labelFilterCriteria" json:"labelFilterCriteria"`
	RecommendationPercentage int             `yaml:"recommendationPercentage" json:"recommendationPercentage"`
}

// RSConfigMapData represents the configmap data structure for right-sizing
type RSConfigMapData struct {
	PrometheusRuleConfig   RSPrometheusRuleConfig   `yaml:"prometheusRuleConfig" json:"prometheusRuleConfig"`
	PlacementConfiguration clusterv1beta1.Placement `yaml:"placementConfiguration" json:"placementConfiguration"`
}
