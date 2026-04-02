package networking

import (
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	"github.com/prometheus/prometheus/promql/parser"
)

// Cluster-level queries (by namespace)

var ClusterQueries = map[string]parser.Expr{
	"ReceiveBandwidthByNamespace": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("namespace_pod:container_network_receive_bytes_total:sum"),
				vector.WithLabelMatchers(
					label.New("namespace").EqualRegexp(".+"),
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
	),
	"TransmitBandwidthByNamespace": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("namespace_pod:container_network_transmit_bytes_total:sum"),
				vector.WithLabelMatchers(
					label.New("namespace").EqualRegexp(".+"),
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
	),
	"ReceivedPacketsByNamespace": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("namespace_pod:container_network_receive_packets_total:sum"),
				vector.WithLabelMatchers(
					label.New("namespace").EqualRegexp(".+"),
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
	),
	"TransmittedPacketsByNamespace": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("namespace_pod:container_network_transmit_packets_total:sum"),
				vector.WithLabelMatchers(
					label.New("namespace").EqualRegexp(".+"),
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
	),
	"ReceivedPacketsDroppedByNamespace": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("namespace_pod:container_network_receive_packets_dropped_total:sum"),
				vector.WithLabelMatchers(
					label.New("namespace").EqualRegexp(".+"),
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
	),
	"TransmittedPacketsDroppedByNamespace": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			vector.New(
				vector.WithMetricName("namespace_pod:container_network_transmit_packets_dropped_total:sum"),
				vector.WithLabelMatchers(
					label.New("namespace").EqualRegexp(".+"),
					label.New("cluster").Equal("$cluster"),
				),
			),
		).By("namespace"),
	),
	// Table queries (same expressions without sort_desc, used for table panel)
	"TableReceiveBandwidthByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_receive_bytes_total:sum"),
			vector.WithLabelMatchers(
				label.New("namespace").EqualRegexp(".+"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"TableTransmitBandwidthByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_transmit_bytes_total:sum"),
			vector.WithLabelMatchers(
				label.New("namespace").EqualRegexp(".+"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"TableReceivedPacketsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_receive_packets_total:sum"),
			vector.WithLabelMatchers(
				label.New("namespace").EqualRegexp(".+"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"TableTransmittedPacketsByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_transmit_packets_total:sum"),
			vector.WithLabelMatchers(
				label.New("namespace").EqualRegexp(".+"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"TableReceivedPacketsDroppedByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_receive_packets_dropped_total:sum"),
			vector.WithLabelMatchers(
				label.New("namespace").EqualRegexp(".+"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	"TableTransmittedPacketsDroppedByNamespace": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_transmit_packets_dropped_total:sum"),
			vector.WithLabelMatchers(
				label.New("namespace").EqualRegexp(".+"),
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("namespace"),
	// TCP retransmit queries use $interval and $resolution variables
	"TCPRetransmits": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			promqlbuilder.Div(
				promqlbuilder.Rate(
					matrix.New(
						vector.New(
							vector.WithMetricName("node_netstat_Tcp_RetransSegs"),
							vector.WithLabelMatchers(
								label.New("cluster").Equal("$cluster"),
							),
						),
						matrix.WithRangeAsVariable("$interval:$resolution"),
					),
				),
				promqlbuilder.Rate(
					matrix.New(
						vector.New(
							vector.WithMetricName("node_netstat_Tcp_OutSegs"),
							vector.WithLabelMatchers(
								label.New("cluster").Equal("$cluster"),
							),
						),
						matrix.WithRangeAsVariable("$interval:$resolution"),
					),
				),
			),
		).By("instance"),
	),
	"TCPSynRetransmits": promqlbuilder.SortDesc(
		promqlbuilder.Sum(
			promqlbuilder.Div(
				promqlbuilder.Rate(
					matrix.New(
						vector.New(
							vector.WithMetricName("node_netstat_TcpExt_TCPSynRetrans"),
							vector.WithLabelMatchers(
								label.New("cluster").Equal("$cluster"),
							),
						),
						matrix.WithRangeAsVariable("$interval:$resolution"),
					),
				),
				promqlbuilder.Rate(
					matrix.New(
						vector.New(
							vector.WithMetricName("node_netstat_Tcp_RetransSegs"),
							vector.WithLabelMatchers(
								label.New("cluster").Equal("$cluster"),
							),
						),
						matrix.WithRangeAsVariable("$interval:$resolution"),
					),
				),
			),
		).By("instance"),
	),
}

// Namespace (Pods) queries (by pod, filtered by namespace)

var NamespacePodsQueries = map[string]parser.Expr{
	"ReceiveBandwidth": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_receive_bytes_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	).By("pod"),
	"TransmitBandwidth": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_transmit_bytes_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	).By("pod"),
	"ReceivedPackets": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_receive_packets_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	).By("pod"),
	"TransmittedPackets": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_transmit_packets_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	).By("pod"),
	"ReceivedPacketsDropped": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_receive_packets_dropped_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	).By("pod"),
	"TransmittedPacketsDropped": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_transmit_packets_dropped_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	).By("pod"),
	// Time series queries (no aggregation by pod - raw metric for stacking)
	"ReceiveBandwidthTS": vector.New(
		vector.WithMetricName("namespace_pod:container_network_receive_bytes_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
		),
	),
	"TransmitBandwidthTS": vector.New(
		vector.WithMetricName("namespace_pod:container_network_transmit_bytes_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
		),
	),
	"ReceivedPacketsTS": vector.New(
		vector.WithMetricName("namespace_pod:container_network_receive_packets_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
		),
	),
	"TransmittedPacketsTS": vector.New(
		vector.WithMetricName("namespace_pod:container_network_transmit_packets_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
		),
	),
	"ReceivedPacketsDroppedTS": vector.New(
		vector.WithMetricName("namespace_pod:container_network_receive_packets_dropped_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
		),
	),
	"TransmittedPacketsDroppedTS": vector.New(
		vector.WithMetricName("namespace_pod:container_network_transmit_packets_dropped_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
		),
	),
	// Gauge queries (total, no grouping)
	"TotalReceiveBandwidth": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_receive_bytes_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	),
	"TotalTransmitBandwidth": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("namespace_pod:container_network_transmit_bytes_total:sum"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
				label.New("namespace").EqualRegexp("$namespace"),
			),
		),
	),
}

// Node queries (by instance)

var NodeQueries = map[string]parser.Expr{
	"ReceiveBandwidthByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_receive_bytes_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),
	"TransmitBandwidthByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_transmit_bytes_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),
	"ReceivedDropByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_receive_drop_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),
	"TransmittedDropByInstance": promqlbuilder.Sum(
		vector.New(
			vector.WithMetricName("instance:node_network_transmit_drop_excluding_lo:rate1m"),
			vector.WithLabelMatchers(
				label.New("cluster").Equal("$cluster"),
			),
		),
	).By("instance"),
}

// Pod queries (filtered by namespace and pod)

var PodQueries = map[string]parser.Expr{
	"ReceiveBandwidth": vector.New(
		vector.WithMetricName("namespace_pod:container_network_receive_bytes_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
			label.New("pod").EqualRegexp("$pod"),
		),
	),
	"TransmitBandwidth": vector.New(
		vector.WithMetricName("namespace_pod:container_network_transmit_bytes_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
			label.New("pod").EqualRegexp("$pod"),
		),
	),
	"ReceivedPackets": vector.New(
		vector.WithMetricName("namespace_pod:container_network_receive_packets_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
			label.New("pod").EqualRegexp("$pod"),
		),
	),
	"TransmittedPackets": vector.New(
		vector.WithMetricName("namespace_pod:container_network_transmit_packets_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
			label.New("pod").EqualRegexp("$pod"),
		),
	),
	"ReceivedPacketsDropped": vector.New(
		vector.WithMetricName("namespace_pod:container_network_receive_packets_dropped_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
			label.New("pod").EqualRegexp("$pod"),
		),
	),
	"TransmittedPacketsDropped": vector.New(
		vector.WithMetricName("namespace_pod:container_network_transmit_packets_dropped_total:sum"),
		vector.WithLabelMatchers(
			label.New("cluster").Equal("$cluster"),
			label.New("namespace").EqualRegexp("$namespace"),
			label.New("pod").EqualRegexp("$pod"),
		),
	),
}
