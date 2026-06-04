package rightsizing

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Standard evaluation intervals for PrometheusRules
var (
	// Duration5m is the evaluation interval for high-resolution metrics (5-minute aggregations)
	Duration5m = monitoringv1.Duration("5m")
	// Duration1d is the evaluation interval for daily aggregated metrics.
	// Must be <= 5m to stay within the Prometheus default staleness window;
	// otherwise the metrics collector federation misses data between evaluations.
	// The 1-day aggregation window is in the PromQL subquery (e.g. [1d:15m]).
	Duration1d = monitoringv1.Duration("5m")
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

// RecommendationProfiles defines all available recommendation profiles.
// Dashboards auto-discover profiles via label_values(profile, ...).
var RecommendationProfiles = []ProfileConfig{
	{Name: "Max OverAll", AggExpr: Build1dMaxAggregationExpr},
	{Name: "P99", AggExpr: BuildP99AggregationExpr},
	{Name: "P95", AggExpr: BuildP95AggregationExpr},
	{Name: "Avg", AggExpr: BuildAvgAggregationExpr},
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

// BuildAvgAggregationExpr builds a 1-day average aggregation expression.
func BuildAvgAggregationExpr(metric5m string) string {
	return fmt.Sprintf(`avg_over_time(%s[1d])`, metric5m)
}

// Build1dAggregationExpr is an alias for Build1dMaxAggregationExpr (backward compat).
var Build1dAggregationExpr = Build1dMaxAggregationExpr

// BuildRecommendationExpr builds a max-based recommendation expression (backward compat).
func BuildRecommendationExpr(usageMetric string, recommendationPercentage int) string {
	return BuildProfiledRecommendationExpr(usageMetric, recommendationPercentage, RecommendationProfiles[0])
}
