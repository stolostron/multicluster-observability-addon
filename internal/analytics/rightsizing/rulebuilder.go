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

// RuleWithLabels creates a PrometheusRule rule with profile and aggregation labels
// These labels are used by dashboards to select the appropriate aggregation level
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

// BuildRecommendationExpr builds a recommendation expression with the given percentage
func BuildRecommendationExpr(usageMetric string, recommendationPercentage int) string {
	if recommendationPercentage == 0 {
		recommendationPercentage = DefaultRecommendationPercentage
	}
	return fmt.Sprintf(`max_over_time(%s[1d]) * (%d/100)`, usageMetric, recommendationPercentage)
}

// Build1dAggregationExpr builds a 1-day max_over_time aggregation expression
func Build1dAggregationExpr(metric5m string) string {
	return fmt.Sprintf(`max_over_time(%s[1d])`, metric5m)
}
