package acm

import (
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	"github.com/perses/perses/go-sdk/prometheus/query"

	"github.com/perses/community-dashboards/pkg/dashboards"
	"github.com/perses/community-dashboards/pkg/promql"

	"github.com/perses/perses/go-sdk/panel/table"
	tablePanel "github.com/perses/perses/go-sdk/panel/table"
)

func Top50MaxLatencyAPIServer(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("Top 50 Max Latency API Server",
		panel.Description("Shows the top 50 clusters with highest API server latency, their API server status, and error rates."),
		tablePanel.Table(
			tablePanel.WithColumnSettings([]tablePanel.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
				},
				{
					Name:   "value",
					Header: "Max Latency (99th percentile)",
				},
				{
					Name:   "api_up",
					Header: "API Server UP",
				},
				{
					Name:   "error_rate",
					Header: "API Error[1h]",
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"topk(50, max(apiserver_request_duration_seconds:histogram_quantile_99{cluster=~'$cluster',clusterType!=\"ocp3\"}) by (cluster)) * on(cluster) group_left(api_up) count_values without() (\"api_up\", (sum(up{cluster=~'$cluster',job=\"apiserver\",clusterType!=\"ocp3\"} == 1) by (cluster) / count(up{cluster=~'$cluster',job=\"apiserver\",clusterType!=\"ocp3\"}) by (cluster)))",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum by (cluster)(sum:apiserver_request_total:1h{cluster=~'$cluster',code=~\"5..\",clusterType!=\"ocp3\"})",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}

func EtcdHealth(datasourceName string, labelMatchers ...promql.LabelMatcher) panelgroup.Option {
	return panelgroup.AddPanel("etcd Health",
		panel.Description("Shows etcd health metrics including leader status, leader changes, and database size."),
		table.Table(
			table.WithColumnSettings([]table.ColumnSettings{
				{
					Name:   "cluster",
					Header: "Cluster",
				},
				{
					Name:   "has_leader",
					Header: "Has a leader",
				},
				{
					Name:   "value",
					Header: "Leader election change",
				},
				{
					Name:   "db_size",
					Header: "DB size",
				},
			}),
		),
		panel.AddQuery(
			query.PromQL(
				promql.SetLabelMatchers(
					"sum(changes(etcd_server_leader_changes_seen_total{cluster=~'$cluster',job=\"etcd\"}[$__range])) by (cluster) * on(cluster) group_left(db_size) count_values without() (\"db_size\", max(etcd_debugging_mvcc_db_total_size_in_bytes{cluster=~'$cluster',job=\"etcd\"}) by (cluster)) * on(cluster) group_left(has_leader) count_values without() (\"has_leader\", max(etcd_server_has_leader{cluster=~'$cluster',job=\"etcd\"}) by (cluster))",
					labelMatchers,
				),
				dashboards.AddQueryDataSource(datasourceName),
			),
		),
	)
}
