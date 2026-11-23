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

	// Workload-pod right-sizing constants
	WorkloadPrometheusRuleName = "acm-rs-workload-prometheus-rules"
	WorkloadConfigMapName      = "rs-workload-config"
	PodPrometheusRuleName      = "acm-rs-pod-prometheus-rules"
	PodConfigMapName           = "rs-pod-config"

	// Default evaluation intervals for PrometheusRule groups.
	// EvaluationInterval controls how often the raw rolling-window groups are evaluated
	// and the lookback window of the PromQL subqueries they contain (e.g. [5m:]).
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
	// are evaluated, and doubles as the lookback window of the PromQL subqueries inside
	// them (e.g. [5m:]). Raising it reduces the evaluation cost per hour and the number of
	// samples stored per recorded series. Each run gets heavier, though: its subqueries
	// cover a longer window, so at 1h a run holds about 12 times as many points in memory
	// as at 5m. Defaults to "5m".
	//
	// The one window it does not drive is the rate() that turns the VM CPU counter into
	// cores: that stays at 5m, so usage peaks keep the same meaning at every interval.
	//
	// It does shape the percentile profiles. The daily rules take P99/P95 over the samples
	// recorded in a day, and each sample is the peak of one interval, so a coarser
	// interval gives them fewer points that each stand for a longer stretch. The same
	// workload then yields different percentiles, higher or lower depending on where its
	// bursts fall between evaluations. Max OverAll, a maximum of those peaks, is not
	// affected. Percentile recommendations are only comparable between clusters and
	// periods that used the same interval.
	//
	// Changing it can lose some usage once. Prometheus moves a rule group to new run times
	// when its interval changes, and each run looks back only one interval, so the stretch
	// between the last run at the old interval and the first run at the new one can be
	// covered by no run at all. Lowering 1h to 5m can skip up to 55m this way, and a usage
	// peak inside that stretch can be missed by every profile; raising the interval to a
	// multiple of the old one skips nothing. After any change the percentile profiles take
	// about a day to settle, while the 1d window still mixes samples taken at both intervals.
	//
	// Must be a single-unit Prometheus duration between 5m and 1h: e.g. "5m", "10m", "30m", "1h".
	// The 5m floor is the default, so this setting can only make the rolling rules cheaper per hour.
	// Below it every managed cluster would evaluate them and store their samples more often,
	// and the daily rules would read more samples per run, while only the percentile
	// profiles would change.
	// The 1h ceiling comes from the fixed 1d aggregation window the daily rules apply on
	// top of these series: at 1h they see 24 samples per day, where P99/P95 are already
	// read off the top two or three samples, and anything coarser leaves them saying
	// little more than the maximum.
	EvaluationInterval string `json:"evaluationInterval,omitempty"`
	// EvaluationInterval1d controls how often the daily-aggregation PrometheusRule groups
	// are re-evaluated. It does not change the aggregation window itself, which is always
	// 1d, only how often the dashboards get a new value. Defaults to "15m".
	//
	// Raising it saves work on every managed cluster, since each run re-reads a full day of
	// the rolling-window series for every daily rule, but it has a cost on the hub side. Of
	// the recorded series only these daily ones are federated, at most one sample per
	// evaluation, so the dashboard graphs that plot them directly thin out as the interval
	// grows and are close to empty once it reaches hours. Stat and table panels aggregate
	// over days, so they stay populated, but each value rests on fewer samples.
	//
	// Must be a single-unit Prometheus duration between 15m and 6h: e.g. "15m", "30m", "1h", "2h".
	// The 15m floor is the default, so this setting can only make the daily rules run less
	// often. Running them more often would multiply their work on every managed cluster, and
	// the samples sent to the hub, only to refresh a 1d aggregate that barely moves between runs.
	// The 6h ceiling keeps at least four runs a day. The hub receives at most one sample per
	// run and the shortest dashboard window is 1d, so at four a day that window still has data
	// after three lost samples in a row, where at 24h a single lost one would empty it for a day.
	EvaluationInterval1d string `json:"evaluationInterval1d,omitempty"`
}

// RSConfigMapData represents the configmap data structure for right-sizing
type RSConfigMapData struct {
	PrometheusRuleConfig   RSPrometheusRuleConfig   `json:"prometheusRuleConfig"`
	PlacementConfiguration clusterv1beta1.Placement `json:"placementConfiguration"`
}
