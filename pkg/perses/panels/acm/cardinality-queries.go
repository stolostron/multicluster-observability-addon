package acm

var CardinalityQueries = map[string]string{
	// Overview - Outliers
	"ClusterOutliersCount": `count(last_over_time(cluster:cardinality[35m]) > ignoring(cluster) group_left avg(last_over_time(cluster:cardinality[35m])) + 3 * stddev(last_over_time(cluster:cardinality[35m])))`,
	"ClusterOutliersTable": `last_over_time(cluster:cardinality[35m]) > ignoring(cluster) group_left avg(last_over_time(cluster:cardinality[35m])) + 3 * stddev(last_over_time(cluster:cardinality[35m]))`,
	"MetricOutliersCount":  `count(last_over_time(name:cardinality[35m]) > ignoring(metric_name) group_left avg(last_over_time(name:cardinality[35m])) + 3 * stddev(last_over_time(name:cardinality[35m])))`,
	"MetricOutliersTable":  `last_over_time(name:cardinality[35m]) > ignoring(metric_name) group_left avg(last_over_time(name:cardinality[35m])) + 3 * stddev(last_over_time(name:cardinality[35m]))`,

	// Overview - Cluster Cardinality
	"ClusterCardinalityOverTime": `topk(8, cluster:cardinality)`,
	"ClusterCardinalityNow":     `last_over_time(cluster:cardinality[35m])`,
	"ClusterCardinality7dAgo":   `last_over_time(cluster:cardinality[35m] offset 7d)`,
	"ClusterCardinality30dAgo":  `last_over_time(cluster:cardinality[35m] offset 30d)`,

	// Overview - Metric Cardinality
	"MetricCardinalityOverTime": `topk(8, name:cardinality)`,
	"MetricCardinalityNow":     `last_over_time(name:cardinality[35m])`,
	"MetricCardinality7dAgo":   `last_over_time(name:cardinality[35m] offset 7d)`,
	"MetricCardinality30dAgo":  `last_over_time(name:cardinality[35m] offset 30d)`,

	// Overview - Global Recording Rules
	"GlobalRulesOverTime": `topk(8, name:no_cluster:cardinality)`,
	"GlobalRulesNow":      `last_over_time(name:no_cluster:cardinality[35m])`,
	"GlobalRules7dAgo":    `last_over_time(name:no_cluster:cardinality[35m] offset 7d)`,
	"GlobalRules30dAgo":   `last_over_time(name:no_cluster:cardinality[35m] offset 30d)`,

	// Overview - Total Cardinality
	"TotalCardinalityOverTime": `sum(cluster:cardinality) + sum(name:no_cluster:cardinality)`,

	// Cluster Dashboard - By Namespace
	"ClusterByNamespaceOverTime":    `topk(8, sum(last_over_time(cluster_namespace:cardinality{cluster="$cluster",namespace!=""}[35m])) by (namespace))`,
	"ClusterNonNamespacedOverTime":  `sum(last_over_time(cluster_namespace:cardinality{cluster="$cluster",namespace=""}[35m]))`,
	"ClusterByNamespaceTable":       `sum(last_over_time(cluster_namespace:cardinality{cluster="$cluster"}[35m])) by (namespace)`,
	"ClusterByNamespaceTotalTable":  `sum(last_over_time(cluster_namespace:cardinality{cluster="$cluster"}[35m]))`,

	// Cluster Dashboard - By Pod
	"ClusterByPodOverTime":    `topk(8, count({__name__=~".+", namespace="$namespace", cluster="$cluster", pod!=""}) by (pod))`,
	"ClusterNoPodOverTime":    `count({__name__=~".+", namespace="$namespace", cluster="$cluster", pod=""})`,
	"ClusterByPodTable":       `count({__name__=~".+", namespace="$namespace", cluster="$cluster"}) by (pod)`,
	"ClusterByPodTotalTable":  `count({__name__=~".+", namespace="$namespace", cluster="$cluster"})`,

	// Cluster Dashboard - In Pod (by metric name)
	"ClusterInPodOverTime":   `topk(8, count({__name__=~".+", cluster="$cluster", namespace="$namespace", pod="$pod"}) by (__name__))`,
	"ClusterInPodTable":      `count({__name__=~".+", cluster="$cluster", namespace="$namespace", pod="$pod"}) by (__name__)`,
	"ClusterInPodTotalTable": `count({__name__=~".+", cluster="$cluster", namespace="$namespace", pod="$pod"})`,

	// Cluster Dashboard - Raw Timeseries
	"ClusterRawTimeseries": `topk(100, {__name__="$metric_name", cluster="$cluster", namespace="$namespace", pod="$pod"})`,

	// Name Dashboard - By Cluster for Metric
	"NameByClusterOverTime":   `sum(last_over_time(cluster_name:cardinality{metric_name="$metric_name"}[35m])) by (cluster)`,
	"NameByClusterTable":      `sum(last_over_time(cluster_name:cardinality{metric_name="$metric_name"}[35m])) by (cluster)`,
	"NameByClusterTotalTable": `sum(last_over_time(cluster_name:cardinality{metric_name="$metric_name"}[35m]))`,

	// Name Dashboard - By Namespace
	"NameByNamespaceOverTime":      `topk(8, count({__name__="$metric_name", cluster="$cluster", namespace!=""}) by (namespace))`,
	"NameNonNamespacedOverTime":    `count({__name__="$metric_name", cluster="$cluster", namespace=""})`,
	"NameByNamespaceTable":         `count({__name__="$metric_name", cluster="$cluster"}) by (namespace)`,
	"NameByNamespaceTotalTable":    `count({__name__="$metric_name", cluster="$cluster"})`,

	// Name Dashboard - By Pod
	"NameByPodOverTime":   `topk(8, count({__name__="$metric_name", cluster="$cluster", namespace="$namespace", pod!=""}) by (pod))`,
	"NameNoPodOverTime":   `count({__name__="$metric_name", cluster="$cluster", namespace="$namespace", pod=""})`,
	"NameByPodTable":      `count({__name__="$metric_name", cluster="$cluster", namespace="$namespace"}) by (pod)`,
	"NameByPodTotalTable": `count({__name__="$metric_name", cluster="$cluster", namespace="$namespace"})`,

	// Name Dashboard - Raw Timeseries
	"NameRawTimeseries": `topk(100, {__name__="$metric_name", cluster="$cluster", namespace="$namespace", pod="$pod"})`,
}
