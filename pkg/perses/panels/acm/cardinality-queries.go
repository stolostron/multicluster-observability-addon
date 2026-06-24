package acm

import (
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
)

// Reusable sub-expressions

func clusterCardinalityVector(opts ...vector.Option) *parser.VectorSelector {
	return vector.New(append([]vector.Option{vector.WithMetricName("cluster:cardinality")}, opts...)...)
}

func nameCardinalityVector(opts ...vector.Option) *parser.VectorSelector {
	return vector.New(append([]vector.Option{vector.WithMetricName("name:cardinality")}, opts...)...)
}

func globalRulesVector(opts ...vector.Option) *parser.VectorSelector {
	return vector.New(append([]vector.Option{vector.WithMetricName("name:no_cluster:cardinality")}, opts...)...)
}

func lastOverTime35m(v *parser.VectorSelector) *parser.Call {
	return promqlbuilder.LastOverTime(matrix.New(v, matrix.WithRangeAsString("35m")))
}

func allSeriesVector(matchers ...*labels.Matcher) *parser.VectorSelector {
	return vector.New(vector.WithLabelMatchers(
		append([]*labels.Matcher{label.New("__name__").EqualRegexp(".+")}, matchers...)...,
	))
}

func namedSeriesVector(metricName string, matchers ...*labels.Matcher) *parser.VectorSelector {
	return vector.New(vector.WithLabelMatchers(
		append([]*labels.Matcher{label.New("__name__").Equal(metricName)}, matchers...)...,
	))
}

// outlierExpr builds: last_over_time(metric[35m]) > ignoring(ignoreLabel) group_left
//
//	avg(last_over_time(metric[35m])) + 3 * stddev(last_over_time(metric[35m]))
func outlierExpr(metricVectorFn func(...vector.Option) *parser.VectorSelector, ignoreLabel string) parser.Expr {
	lot := lastOverTime35m(metricVectorFn())
	return promqlbuilder.Gtr(
		lot,
		promqlbuilder.Add(
			promqlbuilder.Avg(lastOverTime35m(metricVectorFn())),
			promqlbuilder.Mul(
				promqlbuilder.NewNumber(3),
				promqlbuilder.Stddev(lastOverTime35m(metricVectorFn())),
			),
		),
	).Ignoring(ignoreLabel).GroupLeft()
}

var CardinalityQueries = map[string]parser.Expr{
	// Overview - Outliers
	"ClusterOutliersCount": promqlbuilder.Count(outlierExpr(clusterCardinalityVector, "cluster")),
	"ClusterOutliersTable": outlierExpr(clusterCardinalityVector, "cluster"),
	"MetricOutliersCount":  promqlbuilder.Count(outlierExpr(nameCardinalityVector, "metric_name")),
	"MetricOutliersTable":  outlierExpr(nameCardinalityVector, "metric_name"),

	// Overview - Excluded Clusters
	"ExcludedClustersCount": promqlbuilder.Count(
		promqlbuilder.Unless(
			promqlbuilder.Group(vector.New(vector.WithMetricName("up"))).By("cluster"),
			promqlbuilder.Group(lastOverTime35m(clusterCardinalityVector())).By("cluster"),
		),
	),
	"ExcludedClustersList": promqlbuilder.Unless(
		promqlbuilder.Group(vector.New(vector.WithMetricName("up"))).By("cluster"),
		promqlbuilder.Group(lastOverTime35m(clusterCardinalityVector())).By("cluster"),
	),

	// Overview - Cluster Cardinality
	"ClusterCardinalityOverTime": promqlbuilder.TopK(clusterCardinalityVector(), 8),
	"ClusterCardinalityNow":     lastOverTime35m(clusterCardinalityVector()),
	"ClusterCardinality7dAgo":   lastOverTime35m(clusterCardinalityVector(vector.WithOffsetAsString("7d"))),
	"ClusterCardinality30dAgo":  lastOverTime35m(clusterCardinalityVector(vector.WithOffsetAsString("30d"))),

	// Overview - Metric Cardinality
	"MetricCardinalityOverTime": promqlbuilder.TopK(nameCardinalityVector(), 8),
	"MetricCardinalityNow":     lastOverTime35m(nameCardinalityVector()),
	"MetricCardinality7dAgo":   lastOverTime35m(nameCardinalityVector(vector.WithOffsetAsString("7d"))),
	"MetricCardinality30dAgo":  lastOverTime35m(nameCardinalityVector(vector.WithOffsetAsString("30d"))),

	// Overview - Global Recording Rules
	"GlobalRulesOverTime": promqlbuilder.TopK(globalRulesVector(), 8),
	"GlobalRulesNow":      lastOverTime35m(globalRulesVector()),
	"GlobalRules7dAgo":    lastOverTime35m(globalRulesVector(vector.WithOffsetAsString("7d"))),
	"GlobalRules30dAgo":   lastOverTime35m(globalRulesVector(vector.WithOffsetAsString("30d"))),

	// Overview - Total Cardinality
	"TotalCardinalityOverTime": promqlbuilder.Add(
		promqlbuilder.Sum(clusterCardinalityVector()),
		promqlbuilder.Sum(globalRulesVector()),
	),

	// Cluster Dashboard - By Namespace
	"ClusterByNamespaceOverTime": promqlbuilder.TopK(
		promqlbuilder.Sum(
			lastOverTime35m(vector.New(
				vector.WithMetricName("cluster_namespace:cardinality"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("namespace").NotEqual(""),
				),
			)),
		).By("namespace"),
		8,
	),
	"ClusterNonNamespacedOverTime": promqlbuilder.Sum(
		lastOverTime35m(vector.New(
			vector.WithMetricName("cluster_namespace:cardinality"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal(""),
			),
		)),
	),
	"ClusterByNamespaceTable": promqlbuilder.Sum(
		lastOverTime35m(vector.New(
			vector.WithMetricName("cluster_namespace:cardinality"),
			vector.WithLabelMatchers(label.New("cluster").Equal("$cluster")),
		)),
	).By("namespace"),

	// Cluster Dashboard - By Pod
	"ClusterByPodOverTime": promqlbuilder.TopK(
		promqlbuilder.Count(
			allSeriesVector(
				label.New("namespace").Equal("$namespace"),
				label.New("cluster").Equal("$cluster"),
				label.New("pod").NotEqual(""),
			),
		).By("pod"),
		8,
	),
	"ClusterNoPodOverTime": promqlbuilder.Count(
		allSeriesVector(
			label.New("namespace").Equal("$namespace"),
			label.New("cluster").Equal("$cluster"),
			label.New("pod").Equal(""),
		),
	),
	"ClusterByPodTable": promqlbuilder.Count(
		allSeriesVector(
			label.New("namespace").Equal("$namespace"),
			label.New("cluster").Equal("$cluster"),
		),
	).By("pod"),

	// Cluster Dashboard - In Pod (by metric name)
	"ClusterInPodOverTime": promqlbuilder.TopK(
		promqlbuilder.Count(
			allSeriesVector(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").Equal("$pod"),
			),
		).By("__name__"),
		8,
	),
	"ClusterInPodTable": promqlbuilder.Count(
		allSeriesVector(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").Equal("$namespace"),
			label.New("pod").Equal("$pod"),
		),
	).By("__name__"),

	// Cluster Dashboard - Raw Timeseries
	"ClusterRawTimeseries": promqlbuilder.TopK(
		namedSeriesVector("$metric_name",
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").Equal("$namespace"),
			label.New("pod").Equal("$pod"),
		),
		100,
	),

	// Name Dashboard - By Cluster for Metric
	"NameByClusterOverTime": promqlbuilder.Sum(
		lastOverTime35m(vector.New(
			vector.WithMetricName("cluster_name:cardinality"),
			vector.WithLabelMatchers(label.New("metric_name").Equal("$metric_name")),
		)),
	).By("cluster"),
	"NameByClusterTable": promqlbuilder.Sum(
		lastOverTime35m(vector.New(
			vector.WithMetricName("cluster_name:cardinality"),
			vector.WithLabelMatchers(label.New("metric_name").Equal("$metric_name")),
		)),
	).By("cluster"),
	"NameByClusterTotalTable": promqlbuilder.Sum(
		lastOverTime35m(vector.New(
			vector.WithMetricName("cluster_name:cardinality"),
			vector.WithLabelMatchers(label.New("metric_name").Equal("$metric_name")),
		)),
	),

	// Name Dashboard - By Namespace
	"NameByNamespaceOverTime": promqlbuilder.TopK(
		promqlbuilder.Count(
			namedSeriesVector("$metric_name",
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").NotEqual(""),
			),
		).By("namespace"),
		8,
	),
	"NameNonNamespacedOverTime": promqlbuilder.Count(
		namedSeriesVector("$metric_name",
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").Equal(""),
		),
	),
	"NameByNamespaceTable": promqlbuilder.Count(
		namedSeriesVector("$metric_name",
			label.New("cluster").Equal("$cluster"),
		),
	).By("namespace"),

	// Name Dashboard - By Pod
	"NameByPodOverTime": promqlbuilder.TopK(
		promqlbuilder.Count(
			namedSeriesVector("$metric_name",
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").Equal("$namespace"),
				label.New("pod").NotEqual(""),
			),
		).By("pod"),
		8,
	),
	"NameNoPodOverTime": promqlbuilder.Count(
		namedSeriesVector("$metric_name",
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").Equal("$namespace"),
			label.New("pod").Equal(""),
		),
	),
	"NameByPodTable": promqlbuilder.Count(
		namedSeriesVector("$metric_name",
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").Equal("$namespace"),
		),
	).By("pod"),

	// Name Dashboard - Raw Timeseries
	"NameRawTimeseries": promqlbuilder.TopK(
		namedSeriesVector("$metric_name",
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").Equal("$namespace"),
			label.New("pod").Equal("$pod"),
		),
		100,
	),
}
