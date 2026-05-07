package etcd

import (
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/promql/parser"
)

var Queries = map[string]parser.Expr{
	"Up": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("etcd_server_has_leader"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("job").Equal("etcd"),
			),
		),
	),

	"RPCRate": vector.New(
		vector.WithMetricName("grpc_server_started_total:etcd_unary:sum_rate"),
	),
	"RPCFailedRate": vector.New(
		vector.WithMetricName("rpc_rate:grpc_server_handled_total:sum_rate"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),

	"ActiveStreamsWatch": vector.New(
		vector.WithMetricName("active_streams_watch:grpc_server_handled_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),
	"ActiveStreamsLease": vector.New(
		vector.WithMetricName("active_streams_lease:grpc_server_handled_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
		),
	),

	"DBSize": vector.New(
		vector.WithMetricName("etcd_mvcc_db_total_size_in_bytes"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("job").Equal("etcd"),
		),
	),

	"DiskSyncWAL": promqlbuilder.HistogramQuantile(0.99,
		promqlbuilder.Sum(
			promqlbuilder.Rate(
				matrix.New(
					vector.New(
						vector.WithMetricName("etcd_disk_wal_fsync_duration_seconds_bucket"),
						vector.WithLabelMatchers(
							label.New("cluster").Equal("$cluster"),
							label.New("job").Equal("etcd"),
						),
					),
					matrix.WithRangeAsVariable("$__rate_interval"),
				),
			),
		).By("instance", "le"),
	),
	"DiskSyncBackend": promqlbuilder.HistogramQuantile(0.99,
		promqlbuilder.Sum(
			promqlbuilder.Rate(
				matrix.New(
					vector.New(
						vector.WithMetricName("etcd_disk_backend_commit_duration_seconds_bucket"),
						vector.WithLabelMatchers(
							label.New("cluster").Equal("$cluster"),
							label.New("job").Equal("etcd"),
						),
					),
					matrix.WithRangeAsVariable("$__rate_interval"),
				),
			),
		).By("instance", "le"),
	),

	"Memory": vector.New(
		vector.WithMetricName("process_resident_memory_bytes"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("job").Equal("etcd"),
		),
	),

	"ClientTrafficIn": promqlbuilder.Rate(
		matrix.New(
			vector.New(
				vector.WithMetricName("etcd_network_client_grpc_received_bytes_total"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("job").Equal("etcd"),
				),
			),
			matrix.WithRangeAsVariable("$__rate_interval"),
		),
	),
	"ClientTrafficOut": promqlbuilder.Rate(
		matrix.New(
			vector.New(
				vector.WithMetricName("etcd_network_client_grpc_sent_bytes_total"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("job").Equal("etcd"),
				),
			),
			matrix.WithRangeAsVariable("$__rate_interval"),
		),
	),

	"PeerTrafficIn": promqlbuilder.Sum(
		promqlbuilder.Rate(
			matrix.New(
				vector.New(
					vector.WithMetricName("etcd_network_peer_received_bytes_total"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
						label.New("job").Equal("etcd"),
					),
				),
				matrix.WithRangeAsVariable("$__rate_interval"),
			),
		),
	).By("instance"),
	"PeerTrafficOut": promqlbuilder.Sum(
		promqlbuilder.Rate(
			matrix.New(
				vector.New(
					vector.WithMetricName("etcd_network_peer_sent_bytes_total"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
						label.New("job").Equal("etcd"),
					),
				),
				matrix.WithRangeAsVariable("$__rate_interval"),
			),
		),
	).By("instance"),

	"ProposalFailureRate": promqlbuilder.Sum(
		promqlbuilder.Rate(
			matrix.New(
				vector.New(
					vector.WithMetricName("etcd_server_proposals_failed_total"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
						label.New("job").Equal("etcd"),
					),
				),
				matrix.WithRangeAsVariable("$__rate_interval"),
			),
		),
	),
	"ProposalPending": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("etcd_server_proposals_pending"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("job").Equal("etcd"),
			),
		),
	),
	"ProposalCommitRate": promqlbuilder.Sum(
		promqlbuilder.Rate(
			matrix.New(
				vector.New(
					vector.WithMetricName("etcd_server_proposals_committed_total"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
						label.New("job").Equal("etcd"),
					),
				),
				matrix.WithRangeAsVariable("$__rate_interval"),
			),
		),
	),
	"ProposalApplyRate": promqlbuilder.Sum(
		promqlbuilder.Rate(
			matrix.New(
				vector.New(
					vector.WithMetricName("etcd_server_proposals_applied_total"),
					vector.WithLabelMatchers(
						label.New("cluster").Equal("$cluster"),
						label.New("job").Equal("etcd"),
					),
				),
				matrix.WithRangeAsVariable("$__rate_interval"),
			),
		),
	),

	"LeaderElectionsPerDay": promqlbuilder.Changes(
		matrix.New(
			vector.New(
				vector.WithMetricName("etcd_server_leader_changes_seen_total"),
				vector.WithLabelMatchers(
					label.New("cluster").Equal("$cluster"),
					label.New("job").Equal("etcd"),
				),
			),
			matrix.WithRangeAsString("1d"),
		),
	),
}
