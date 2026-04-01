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

	"ClustersExceededSLO": promqlbuilder.Sum(
		promqlbuilder.Lss(
			fleetSLO(),
			promqlbuilder.NewNumber(0.99),
		).Bool(),
	),

	"ClustersMeetingSLO": promqlbuilder.Sum(
		promqlbuilder.Gte(
			fleetSLO(),
			promqlbuilder.NewNumber(0.99),
		).Bool(),
	),

	"TopClustersSLO": promqlbuilder.SortDesc(
		bottomKVariable("$top", fleetSLO()),
	),

	"TopClustersErrorBudget": promqlbuilder.Sub(
		promqlbuilder.NewNumber(0.99),
		promqlbuilder.SortDesc(
			bottomKVariable("$top", fleetSLO()),
		),
	),

	"TopClustersSLITrend": bottomKVariable("$top",
		vector.New(
			vector.WithMetricName("sli:apiserver_request_duration_seconds:trend:1m"),
			vector.WithLabelMatchers(
				label.New("cluster").EqualRegexp("$cluster"),
			),
		),
	),

	"TargetThreshold": promqlbuilder.NewNumber(0.99),
}

var ClusterQueries = map[string]parser.Expr{
	"SLIBinTrend": vector.New(
		vector.WithMetricName("sli:apiserver_request_duration_seconds:bin:trend:1m"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),

	"SLO7d": clusterSLO("7d"),

	"SLO30d": clusterSLO("30d"),

	"DayOfWeek": &parser.Call{
		Func: parser.Functions["day_of_week"],
		Args: parser.Expressions{},
	},

	"DayOfMonth": &parser.Call{
		Func: parser.Functions["day_of_month"],
		Args: parser.Expressions{},
	},

	"ErrorBudget7d": errorBudget("7d"),

	"ErrorBudget30d": errorBudget("30d"),

	"DowntimeRemaining7d": promqlbuilder.Mul(
		promqlbuilder.Mul(
			errorBudget("7d"),
			promqlbuilder.NewNumber(10080),
		),
		promqlbuilder.NewNumber(-1),
	),

	"DowntimeRemaining30d": promqlbuilder.Mul(
		promqlbuilder.Mul(
			errorBudget("30d"),
			promqlbuilder.NewNumber(43200), // 30 * 24 * 60
		),
		promqlbuilder.NewNumber(-1),
	),

	"SLITrend": vector.New(
		vector.WithMetricName("sli:apiserver_request_duration_seconds:trend:1m"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),

	"TargetThreshold": promqlbuilder.NewNumber(0.99),
}
