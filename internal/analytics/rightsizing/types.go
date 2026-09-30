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

	// Namespace right-sizing constants
	NamespacePrometheusRuleName = "acm-rs-namespace-prometheus-rules"
	NamespaceConfigMapName      = "rs-namespace-config"

	// Virtualization right-sizing constants
	VirtualizationPrometheusRuleName = "acm-rs-virt-prometheus-rules"
	VirtualizationConfigMapName      = "rs-virt-config"

	// Default evaluation intervals for PrometheusRule groups.
	// EvaluationInterval controls how often the raw 5m rolling-window groups are evaluated
	// and the subquery step used in PromQL expressions (e.g. [5m:]).
	// EvaluationInterval1d controls how often the daily-aggregation groups are re-evaluated.
	DefaultEvaluationInterval   = "5m"
	DefaultEvaluationInterval1d = "15m"
)

// RSLabelFilter represents label filtering criteria for right-sizing
type RSLabelFilter struct {
	LabelName         string   `json:"labelName"`
	InclusionCriteria []string `json:"inclusionCriteria,omitempty"`
	ExclusionCriteria []string `json:"exclusionCriteria,omitempty"`
}

// DefaultCpuAggregator is the default set of CPU aggregation profiles.
var DefaultCpuAggregator = []string{"Max OverAll", "P99", "P95"}

// DefaultMemoryAggregator is the default set of memory aggregation profiles.
var DefaultMemoryAggregator = []string{"Max OverAll", "P99", "P95"}

// RSPrometheusRuleConfig represents the Prometheus rule configuration for right-sizing
type RSPrometheusRuleConfig struct {
	NamespaceFilterCriteria struct {
		InclusionCriteria []string `json:"inclusionCriteria"`
		ExclusionCriteria []string `json:"exclusionCriteria"`
	} `json:"namespaceFilterCriteria"`
	LabelFilterCriteria      []RSLabelFilter `json:"labelFilterCriteria"`
	RecommendationPercentage int             `json:"recommendationPercentage"`
	CpuAggregator            []string        `json:"cpuAggregator"`
	MemoryAggregator         []string        `json:"memoryAggregator"`
	// EvaluationInterval controls how often the raw rolling-window PrometheusRule groups
	// are evaluated and the subquery step used in PromQL (e.g. [5m:]). Defaults to "5m".
	// Valid Prometheus duration strings: e.g. "1m", "5m", "10m", "1h".
	EvaluationInterval string `json:"evaluationInterval,omitempty"`
	// EvaluationInterval1d controls how often the daily-aggregation PrometheusRule groups
	// are re-evaluated. Defaults to "15m".
	// Valid Prometheus duration strings: e.g. "5m", "15m", "30m", "1h".
	EvaluationInterval1d string `json:"evaluationInterval1d,omitempty"`
}

// RSConfigMapData represents the configmap data structure for right-sizing
type RSConfigMapData struct {
	PrometheusRuleConfig   RSPrometheusRuleConfig   `json:"prometheusRuleConfig"`
	PlacementConfiguration clusterv1beta1.Placement `json:"placementConfiguration"`
}
