package rightsizing

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var percentileNamePattern = regexp.MustCompile(`^[Pp]([0-9]{1,2})$`)

// Standard evaluation intervals for PrometheusRules
var (
	// Duration5m is the evaluation interval for high-resolution metrics (5-minute aggregations)
	Duration5m = monitoringv1.Duration("5m")
	// Duration15m is the evaluation interval for daily aggregated metrics.
	// Rules evaluate every 15 minutes but aggregate data over 1 day (via max_over_time(...[1d])).
	// This provides fresh dashboard data without waiting a full day between evaluations.
	Duration1d = monitoringv1.Duration("15m")
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
