package rightsizing

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// prometheusDurationPattern matches a single-unit Prometheus duration and captures the
// numeric part and the unit separately. Compound durations (e.g. "1h30m") are
// intentionally unsupported to keep validation simple and match ConfigMap usage patterns.
var prometheusDurationPattern = regexp.MustCompile(`^(\d+)(ms|s|m|h|d|w|y)$`)

// durationUnitMultiplier maps Prometheus duration units to their time.Duration value.
// Prometheus defines d/w/y as fixed spans (24h / 7d / 365d), so no calendar math applies.
var durationUnitMultiplier = map[string]time.Duration{
	"ms": time.Millisecond,
	"s":  time.Second,
	"m":  time.Minute,
	"h":  time.Hour,
	"d":  24 * time.Hour,
	"w":  7 * 24 * time.Hour,
	"y":  365 * 24 * time.Hour,
}

// Bounds for the ConfigMap-driven evaluation intervals. The two fields feed different
// parts of the pipeline, so they do not share an upper bound.
const (
	// AggregationWindow is the fixed lookback of the daily aggregation rules. It is
	// hardcoded as [1d] in Build1dMaxAggregationExpr and the percentile builders, which
	// is what constrains how coarse EvaluationInterval may become.
	AggregationWindow = 24 * time.Hour

	// MinSamplesPerAggregationWindow is the smallest number of rolling-window samples the
	// daily rules need for the percentile profiles to stay distinguishable. At this count
	// quantile_over_time already reads P99/P95 off the top two or three samples; below it
	// they say little more than the maximum and the configurable-profile feature silently
	// stops meaning anything.
	MinSamplesPerAggregationWindow = 24

	// MinEvaluationInterval applies to both fields. The rolling interval doubles as the
	// lookback window of a PromQL subquery whose resolution defaults to the global
	// evaluation interval (typically 30s on OpenShift), so below a minute the window holds
	// barely more than a single sample while Prometheus is still forced to re-evaluate the
	// whole rule group at that rate.
	MinEvaluationInterval = time.Minute

	// MaxEvaluationInterval caps the rolling-window interval so the daily rules always see
	// at least MinSamplesPerAggregationWindow points. It also bounds the subquery range:
	// the value is substituted into [X:], and at a 30s resolution an unbounded window such
	// as "1y" expands to over a million points per series, stalling or OOM-ing Prometheus
	// on every managed cluster.
	MaxEvaluationInterval = AggregationWindow / MinSamplesPerAggregationWindow

	// MaxEvaluationInterval1d caps how rarely the daily rules may re-run. Each evaluation
	// aggregates the trailing AggregationWindow, so re-evaluating less often than that
	// leaves gaps between consecutive windows: data that no evaluation ever observes.
	MaxEvaluationInterval1d = AggregationWindow
)

// parsePrometheusDuration converts a single-unit Prometheus duration into a time.Duration.
// It reports false for malformed input, a zero numeric part (Prometheus rejects zero-value
// intervals and subquery ranges), or a value so large it would overflow time.Duration.
func parsePrometheusDuration(s string) (time.Duration, bool) {
	m := prometheusDurationPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, false
	}
	unit := durationUnitMultiplier[m[2]]
	// Guard against int64 overflow on absurd inputs such as "999999999999y".
	if int64(n) > int64(math.MaxInt64)/int64(unit) {
		return 0, false
	}
	return time.Duration(n) * unit, true
}

// validateInterval reports whether s is a single-unit Prometheus duration (e.g. "5m",
// "1h", "300s") with a non-zero value inside [MinEvaluationInterval, maxAllowed].
// Empty strings are accepted and treated as "use default" by the callers.
func validateInterval(s string, maxAllowed time.Duration) bool {
	if s == "" {
		return true
	}
	d, ok := parsePrometheusDuration(s)
	if !ok {
		return false
	}
	return d >= MinEvaluationInterval && d <= maxAllowed
}

// ValidateEvaluationInterval reports whether s is usable as the rolling-window evaluation
// interval. Beyond MaxEvaluationInterval the daily aggregation rules no longer see enough
// samples for the percentile profiles to say more than the maximum does.
func ValidateEvaluationInterval(s string) bool {
	return validateInterval(s, MaxEvaluationInterval)
}

// ValidateEvaluationInterval1d reports whether s is usable as the daily-aggregation
// evaluation interval. Its ceiling is the aggregation window itself, since re-running less
// often than that would leave gaps no evaluation ever covers.
func ValidateEvaluationInterval1d(s string) bool {
	return validateInterval(s, MaxEvaluationInterval1d)
}

// RedundantEvaluationFactor reports how many times the daily aggregation rules re-run per
// new rolling-window sample when the rolling interval is the coarser of the two. A factor
// above 1 means those rules recompute over input that has not changed since the last run:
// the recorded value is still correct, but the extra evaluations and the duplicate samples
// they append to the recommendation series are pure overhead. This is a quality warning,
// not a validation failure — the configuration works, it is just wasteful.
//
// Empty fields are resolved to their defaults first, so the comparison reflects the
// intervals that will actually run. ok is false when the intervals are already well
// ordered or when either value cannot be parsed.
//
// The factor is rounded up: when the intervals do not divide evenly (e.g. 20m rolling
// against 15m daily) truncating would report 1, which reads as "no redundancy" and
// contradicts the warning being emitted alongside it.
func RedundantEvaluationFactor(rolling, daily string) (int, bool) {
	if rolling == "" {
		rolling = DefaultEvaluationInterval
	}
	if daily == "" {
		daily = DefaultEvaluationInterval1d
	}
	r, rok := parsePrometheusDuration(rolling)
	d, dok := parsePrometheusDuration(daily)
	if !rok || !dok || r <= d {
		return 0, false
	}
	return int((r + d - 1) / d), true
}

// ResolveEvaluationInterval returns the configured rolling-window eval interval as a
// monitoringv1.Duration, falling back to Duration5m when the config value is absent.
func ResolveEvaluationInterval(config RSPrometheusRuleConfig) monitoringv1.Duration {
	if config.EvaluationInterval != "" {
		return monitoringv1.Duration(config.EvaluationInterval)
	}
	return Duration5m
}

// ResolveEvaluationInterval1d returns the configured 1d-group eval interval as a
// monitoringv1.Duration, falling back to Duration1d when the config value is absent.
func ResolveEvaluationInterval1d(config RSPrometheusRuleConfig) monitoringv1.Duration {
	if config.EvaluationInterval1d != "" {
		return monitoringv1.Duration(config.EvaluationInterval1d)
	}
	return Duration1d
}

var percentileNamePattern = regexp.MustCompile(`^[Pp]([0-9]{1,2})$`)

// Standard evaluation intervals for PrometheusRules. These are the fallbacks used when
// the ConfigMap does not set the corresponding field, so they are derived from the same
// constants the handler writes into the ConfigMap to keep the two paths from drifting.
var (
	// Duration5m is the default evaluation interval for the rolling-window rule groups.
	Duration5m = monitoringv1.Duration(DefaultEvaluationInterval)
	// Duration1d is the default evaluation interval for daily aggregated metrics.
	// Rules evaluate every 15 minutes but aggregate data over 1 day (via max_over_time(...[1d])).
	// This provides fresh dashboard data without waiting a full day between evaluations.
	Duration1d = monitoringv1.Duration(DefaultEvaluationInterval1d)
)

// RuleBuilder provides common utilities for building PrometheusRule rules
type RuleBuilder struct {
	// LabelJoin is an optional label join expression to append to metrics
	LabelJoin string
	// Profile is the profile label value for aggregated rules (default: "Max OverAll")
	Profile string
	// Aggregation is the aggregation label value (default: "1d")
	Aggregation string
}

// NewRuleBuilder creates a new RuleBuilder with default values
func NewRuleBuilder(labelJoin string) *RuleBuilder {
	return &RuleBuilder{
		LabelJoin:   labelJoin,
		Profile:     "Max OverAll",
		Aggregation: "1d",
	}
}

// Rule creates a basic PrometheusRule rule with optional label join
func (rb *RuleBuilder) Rule(record, metricExpr string) monitoringv1.Rule {
	expr := metricExpr
	if rb.LabelJoin != "" {
		expr = fmt.Sprintf("%s %s", metricExpr, rb.LabelJoin)
	}
	return monitoringv1.Rule{
		Record: record,
		Expr:   intstr.FromString(expr),
	}
}

// RuleNoJoin creates a basic PrometheusRule rule without appending the label join.
// Use for cluster-level rules where namespace has been aggregated away.
func (rb *RuleBuilder) RuleNoJoin(record, metricExpr string) monitoringv1.Rule {
	return monitoringv1.Rule{
		Record: record,
		Expr:   intstr.FromString(metricExpr),
	}
}

// RuleWithLabels creates a PrometheusRule rule with profile and aggregation labels.
// Dashboards use these labels to select the appropriate aggregation level.
func (rb *RuleBuilder) RuleWithLabels(record, expr string) monitoringv1.Rule {
	return monitoringv1.Rule{
		Record: record,
		Expr:   intstr.FromString(expr),
		Labels: map[string]string{
			"profile":     rb.Profile,
			"aggregation": rb.Aggregation,
		},
	}
}

// WithProfile returns a copy of the RuleBuilder with a different profile name.
func (rb *RuleBuilder) WithProfile(name string) *RuleBuilder {
	return &RuleBuilder{
		LabelJoin:   rb.LabelJoin,
		Profile:     name,
		Aggregation: rb.Aggregation,
	}
}

// ProfileConfig defines how a recommendation profile aggregates 5m metrics over 1d.
type ProfileConfig struct {
	Name    string
	AggExpr func(metric5m string) string
}

// RecommendationProfiles defines the available recommendation profiles.
// Dashboards auto-discover profiles via label_values(profile, ...).
var RecommendationProfiles = []ProfileConfig{
	{Name: "Max OverAll", AggExpr: Build1dMaxAggregationExpr},
	{Name: "P99", AggExpr: BuildP99AggregationExpr},
	{Name: "P95", AggExpr: BuildP95AggregationExpr},
}

// BuildProfiledRecommendationExpr builds a recommendation expression for a given profile.
func BuildProfiledRecommendationExpr(usageMetric string, rp int, profile ProfileConfig) string {
	if rp == 0 {
		rp = DefaultRecommendationPercentage
	}
	return fmt.Sprintf(`%s * (%d/100)`, profile.AggExpr(usageMetric), rp)
}

// Build1dMaxAggregationExpr builds a 1-day max_over_time aggregation expression.
func Build1dMaxAggregationExpr(metric5m string) string {
	return fmt.Sprintf(`max_over_time(%s[1d])`, metric5m)
}

// BuildP99AggregationExpr builds a 1-day 99th-percentile aggregation expression.
func BuildP99AggregationExpr(metric5m string) string {
	return fmt.Sprintf(`quantile_over_time(0.99, %s[1d])`, metric5m)
}

// BuildP95AggregationExpr builds a 1-day 95th-percentile aggregation expression.
func BuildP95AggregationExpr(metric5m string) string {
	return fmt.Sprintf(`quantile_over_time(0.95, %s[1d])`, metric5m)
}

// BuildPercentileAggregationExpr builds a quantile_over_time expression for an arbitrary percentile.
func BuildPercentileAggregationExpr(percentile float64) func(string) string {
	return func(metric5m string) string {
		return fmt.Sprintf(`quantile_over_time(%s, %s[1d])`,
			strconv.FormatFloat(percentile, 'f', -1, 64), metric5m)
	}
}

// knownProfiles maps well-known profile names to their aggregation functions.
var knownProfiles = map[string]func(string) string{
	"Max OverAll": Build1dMaxAggregationExpr,
	"P99":         BuildP99AggregationExpr,
	"P95":         BuildP95AggregationExpr,
}

// ParseAggregatorNames converts a list of profile name strings (e.g. "Max OverAll", "P99", "P90")
// into ProfileConfig slices. Well-known names use pre-built functions; "Pxx" names are parsed
// dynamically and canonicalized (e.g. "p90" -> "P90"). Duplicates are collapsed and
// unrecognized names are skipped.
func ParseAggregatorNames(names []string) []ProfileConfig {
	seen := make(map[string]bool)
	var profiles []ProfileConfig
	for _, name := range names {
		name = strings.TrimSpace(name)
		if fn, ok := knownProfiles[name]; ok {
			if !seen[name] {
				seen[name] = true
				profiles = append(profiles, ProfileConfig{Name: name, AggExpr: fn})
			}
			continue
		}
		if canonical, p, ok := parsePercentileName(name); ok {
			if !seen[canonical] {
				seen[canonical] = true
				profiles = append(profiles, ProfileConfig{
					Name:    canonical,
					AggExpr: BuildPercentileAggregationExpr(p),
				})
			}
		}
	}
	return profiles
}

// parsePercentileName extracts the percentile value from names like "P90", "P75", "P50".
// Returns the canonical name (e.g. "P90"), the quantile fraction (e.g. 0.9), and whether
// the name was valid. Only integer percentiles 1-99 are accepted (no floats, no P0/P100).
func parsePercentileName(name string) (string, float64, bool) {
	match := percentileNamePattern.FindStringSubmatch(strings.TrimSpace(name))
	if match == nil {
		return "", 0, false
	}
	val, err := strconv.Atoi(match[1])
	if err != nil || val <= 0 || val >= 100 {
		return "", 0, false
	}
	canonical := fmt.Sprintf("P%d", val)
	return canonical, float64(val) / 100, true
}

// ValidateAggregatorNames checks every name in the list and returns any that are
// not recognized as valid profile names. Valid names are well-known profiles
// (e.g. "Max OverAll", "P99", "P95") or any "Pxx" where xx is between 1 and 99.
func ValidateAggregatorNames(names []string) []string {
	var invalid []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if _, ok := knownProfiles[name]; ok {
			continue
		}
		if _, _, ok := parsePercentileName(name); ok {
			continue
		}
		invalid = append(invalid, name)
	}
	return invalid
}

// ResolveCpuProfiles returns the CPU ProfileConfig list from config, falling back to defaults.
func ResolveCpuProfiles(config RSPrometheusRuleConfig) []ProfileConfig {
	return resolveProfiles(config.CpuAggregator)
}

// ResolveMemoryProfiles returns the memory ProfileConfig list from config, falling back to defaults.
func ResolveMemoryProfiles(config RSPrometheusRuleConfig) []ProfileConfig {
	return resolveProfiles(config.MemoryAggregator)
}

// resolveProfiles falls back to RecommendationProfiles when the list is absent or
// contains no valid entry, so a typo cannot leave the 1d rule groups empty.
func resolveProfiles(names []string) []ProfileConfig {
	if profiles := ParseAggregatorNames(names); len(profiles) > 0 {
		return profiles
	}
	return RecommendationProfiles
}
