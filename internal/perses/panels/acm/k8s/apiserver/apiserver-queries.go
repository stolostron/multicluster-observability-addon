package apiserver

import (
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/promql/parser"
)

func upVector() parser.Expr {
	return vector.New(
		vector.WithMetricName("up"),
		vector.WithLabelMatchers(
			label.New("service").Equal("kubernetes"),
			label.New("cluster").Equal("$cluster"),
		),
	)
}

var Queries = map[string]parser.Expr{
	"APIServersUp": promqlbuilder.Div(
		promqlbuilder.Sum(upVector()),
		promqlbuilder.Count(upVector()),
	),

	"RequestLatencyP99": vector.New(
		vector.WithMetricName("apiserver_request_duration_seconds:histogram_quantile_99:instance"),
		vector.WithLabelMatchers(
			label.New("instance").EqualRegexp("$instance"),
			label.New("cluster").Equal("$cluster"),
		),
	),
	"LatencyThreshold": promqlbuilder.NewNumber(1),

	"RequestRate2xx": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("sum:apiserver_request_total:5m"),
			vector.WithLabelMatchers(
				label.New("instance").EqualRegexp("$instance"),
				label.New("code").EqualRegexp("2.."),
				label.New("cluster").Equal("$cluster"),
			),
		),
	),
	"RequestRate3xx": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("sum:apiserver_request_total:5m"),
			vector.WithLabelMatchers(
				label.New("instance").EqualRegexp("$instance"),
				label.New("code").EqualRegexp("3.."),
				label.New("cluster").Equal("$cluster"),
			),
		),
	),
	"RequestRate4xx": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("sum:apiserver_request_total:5m"),
			vector.WithLabelMatchers(
				label.New("instance").EqualRegexp("$instance"),
				label.New("code").EqualRegexp("4.."),
				label.New("cluster").Equal("$cluster"),
			),
		),
	),
	"RequestRate5xx": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("sum:apiserver_request_total:5m"),
			vector.WithLabelMatchers(
				label.New("instance").EqualRegexp("$instance"),
				label.New("code").EqualRegexp("5.."),
				label.New("cluster").Equal("$cluster"),
			),
		),
	),

	"WorkQueueLatency": vector.New(
		vector.WithMetricName("workqueue_queue_duration_seconds_bucket:apiserver:histogram_quantile_99"),
		vector.WithLabelMatchers(
			label.New("instance").EqualRegexp("$instance"),
			label.New("cluster").Equal("$cluster"),
		),
	),

	"QueueDepth": promqlbuilder.Sum(
		promqlbuilder.Rate(
			matrix.New(
				vector.New(
					vector.WithMetricName("workqueue_depth"),
					vector.WithLabelMatchers(
						label.New("job").Equal("apiserver"),
						label.New("instance").EqualRegexp("$instance"),
						label.New("cluster").Equal("$cluster"),
					),
				),
				matrix.WithRangeAsVariable("$__rate_interval"),
			),
		),
	).By("instance", "name"),

	"QueueAddRate": promqlbuilder.Sum(
		promqlbuilder.Rate(
			matrix.New(
				vector.New(
					vector.WithMetricName("workqueue_adds_total"),
					vector.WithLabelMatchers(
						label.New("job").Equal("apiserver"),
						label.New("instance").EqualRegexp("$instance"),
						label.New("cluster").Equal("$cluster"),
					),
				),
				matrix.WithRangeAsVariable("$__rate_interval"),
			),
		),
	).By("instance", "name"),

	"Memory": vector.New(
		vector.WithMetricName("process_resident_memory_bytes"),
		vector.WithLabelMatchers(
			label.New("job").Equal("apiserver"),
			label.New("instance").EqualRegexp("$instance"),
			label.New("cluster").Equal("$cluster"),
		),
	),

	"CPUUsage": promqlbuilder.Rate(
		matrix.New(
			vector.New(
				vector.WithMetricName("process_cpu_seconds_total"),
				vector.WithLabelMatchers(
					label.New("job").Equal("apiserver"),
					label.New("instance").EqualRegexp("$instance"),
					label.New("cluster").Equal("$cluster"),
				),
			),
			matrix.WithRangeAsVariable("$__rate_interval"),
		),
	),

	"Goroutines": vector.New(
		vector.WithMetricName("go_goroutines"),
		vector.WithLabelMatchers(
			label.New("job").Equal("apiserver"),
			label.New("instance").EqualRegexp("$instance"),
			label.New("cluster").Equal("$cluster"),
		),
	),
}
