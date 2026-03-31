package slo

import (
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

// variableExpr renders a Perses variable reference (e.g. $top) as a PromQL expression.
type variableExpr struct {
	name string
}

func (v *variableExpr) String() string                            { return v.name }
func (v *variableExpr) Pretty(int) string                         { return v.name }
func (v *variableExpr) Type() parser.ValueType                    { return parser.ValueTypeScalar }
func (v *variableExpr) PromQLExpr()                               {}
func (v *variableExpr) PositionRange() posrange.PositionRange     { return posrange.PositionRange{} }
func (v *variableExpr) SetPositionRange(_ posrange.PositionRange) {}

// fleetSLIBinTrendMatrix returns a matrix selector for the SLI bin trend metric
// with cluster regex match and $window range variable.
func fleetSLIBinTrendMatrix() *matrix.Builder {
	return matrix.New(
		vector.New(
			vector.WithMetricName("sli:apiserver_request_duration_seconds:bin:trend:1m"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
			),
		),
		matrix.WithRangeAsVariable("$window"),
	)
}

// fleetSLO returns floor(sum_over_time(sli_bin_trend[$window])) / count_over_time(sli_bin_trend[$window])
func fleetSLO() parser.Expr {
	return promqlbuilder.Div(
		promqlbuilder.Floor(promqlbuilder.SumOverTime(fleetSLIBinTrendMatrix())),
		promqlbuilder.CountOverTime(fleetSLIBinTrendMatrix()),
	)
}

// bottomKVariable creates a bottomk aggregation using a variable parameter (e.g. $top).
func bottomKVariable(variable string, expr parser.Expr) *parser.AggregateExpr {
	return &parser.AggregateExpr{
		Op:    parser.BOTTOMK,
		Expr:  expr,
		Param: &variableExpr{name: variable},
	}
}

// clusterSLIBinTrendMatrix returns a matrix selector for the SLI bin trend metric
// with exact cluster match and the given range string.
func clusterSLIBinTrendMatrix(rangeStr string) *matrix.Builder {
	return matrix.New(
		vector.New(
			vector.WithMetricName("sli:apiserver_request_duration_seconds:bin:trend:1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
		matrix.WithRangeAsString(rangeStr),
	)
}

// clusterSLO returns max(floor(sum_over_time(sli_bin_trend[range])) / count_over_time(sli_bin_trend[range]))
// Wrapped in max() to collapse extra labels (receive, tenant_id, etc.) into a single series for stat/gauge panels.
func clusterSLO(rangeStr string) parser.Expr {
	return promqlbuilder.Max(
		promqlbuilder.Div(
			promqlbuilder.Floor(promqlbuilder.SumOverTime(clusterSLIBinTrendMatrix(rangeStr))),
			promqlbuilder.CountOverTime(clusterSLIBinTrendMatrix(rangeStr)),
		),
	)
}

// errorBudget returns 0.99 - SLO(range)
func errorBudget(rangeStr string) parser.Expr {
	return promqlbuilder.Sub(
		promqlbuilder.NewNumber(0.99),
		clusterSLO(rangeStr),
	)
}

// Fleet-level SLO queries (use $window variable for range)
var FleetQueries = map[string]parser.Expr{
	// Number of clusters exceeding SLO target
	"ClustersExceededSLO": promqlbuilder.Sum(
		promqlbuilder.Lss(
			fleetSLO(),
			promqlbuilder.NewNumber(0.99),
		).Bool(),
	),
	// Number of clusters meeting SLO target
	"ClustersMeetingSLO": promqlbuilder.Sum(
		promqlbuilder.Gte(
			fleetSLO(),
			promqlbuilder.NewNumber(0.99),
		).Bool(),
	),
	// Top clusters sorted by worst SLO
	"TopClustersSLO": promqlbuilder.SortDesc(
		bottomKVariable("$top", fleetSLO()),
	),
	// Top clusters error budget
	"TopClustersErrorBudget": promqlbuilder.Sub(
		promqlbuilder.NewNumber(0.99),
		promqlbuilder.SortDesc(
			bottomKVariable("$top", fleetSLO()),
		),
	),
	// Top clusters SLI trend
	"TopClustersSLITrend": bottomKVariable("$top",
		vector.New(
			vector.WithMetricName("sli:apiserver_request_duration_seconds:trend:1m"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
			),
		),
	),
	// Target threshold line
	"TargetThreshold": promqlbuilder.NewNumber(0.99),
}

// Cluster-level SLO queries
var ClusterQueries = map[string]parser.Expr{
	// SLI bin trend (for Target stat)
	"SLIBinTrend": vector.New(
		vector.WithMetricName("sli:apiserver_request_duration_seconds:bin:trend:1m"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	// SLO over 7 days
	"SLO7d": clusterSLO("7d"),
	// SLO over 30 days
	"SLO30d": clusterSLO("30d"),
	// Day of the week
	"DayOfWeek": &parser.Call{
		Func: parser.Functions["day_of_week"],
		Args: parser.Expressions{},
	},
	// Day of the month
	"DayOfMonth": &parser.Call{
		Func: parser.Functions["day_of_month"],
		Args: parser.Expressions{},
	},
	// Error budget consumed (7 days)
	"ErrorBudget7d": errorBudget("7d"),
	// Error budget consumed (30 days)
	"ErrorBudget30d": errorBudget("30d"),
	// Downtime remaining (7 days) in minutes: error_budget * total_minutes * -1
	"DowntimeRemaining7d": promqlbuilder.Mul(
		promqlbuilder.Mul(
			errorBudget("7d"),
			promqlbuilder.NewNumber(10080), // 7 * 24 * 60
		),
		promqlbuilder.NewNumber(-1),
	),
	// Downtime remaining (30 days) in minutes: error_budget * total_minutes * -1
	"DowntimeRemaining30d": promqlbuilder.Mul(
		promqlbuilder.Mul(
			errorBudget("30d"),
			promqlbuilder.NewNumber(43200), // 30 * 24 * 60
		),
		promqlbuilder.NewNumber(-1),
	),
	// SLI trend
	"SLITrend": vector.New(
		vector.WithMetricName("sli:apiserver_request_duration_seconds:trend:1m"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	// Target threshold line
	"TargetThreshold": promqlbuilder.NewNumber(0.99),
}
