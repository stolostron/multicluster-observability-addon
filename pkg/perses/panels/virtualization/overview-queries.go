package virtualization

// overviewHealthJoin filters series to only clusters matching the $operator_health variable.
// Appended as +on(cluster) group_left()(...) to per-cluster queries whose result
// already has a cluster label. The left side must end with a closing ) from an
// aggregation or explicit grouping — never after a bare comparison operator.
const overviewHealthJoin = `+on(cluster) group_left()(0*sum by (cluster)(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health))`

// overviewHealthAnd filters series via set intersection (and on(cluster)).
// Use instead of overviewHealthJoin when the left side ends with a comparison
// operator (==, !=, <, >, <=, >=) to avoid PromQL precedence issues, or when
// the original query counts raw series rather than per-cluster aggregates.
// "and" has lower precedence than all comparison and arithmetic operators, so
// it is always safe to append without extra parentheses.
const overviewHealthAnd = ` and on(cluster) sum by (cluster)(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health)`

// overviewHealthFilteredClusters is the set of clusters passing the health filter,
// used inside aggregate queries that need to restrict clusters before aggregation.
const overviewHealthFilteredClusters = `on(cluster) group_left()(0*sum by (cluster)(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health))`

// TotalClusters (0_0)
const overviewTotalClusters = `count (
label_replace(sum by (name, openshiftVersion) (acm_managed_cluster_labels{name=~"$cluster"}), "cluster", "$1", "name", "(.*)") + 
` + overviewHealthFilteredClusters + `
)`

// ClustersCriticalHealth (0_1)
const overviewClustersCriticalHealth = `count(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"} == 2` + overviewHealthAnd + `) or vector(0)`

// TotalAllocatableNodes (0_2)
const overviewTotalAllocatableNodes = `sum(count by (cluster) (count(kube_node_status_allocatable{resource=~".*kubevirt.*", cluster=~"$cluster"}) by (cluster,node))` + overviewHealthJoin + `)`

// TotalVMs (0_3)
const overviewTotalVMs = `sum(count by (name, namespace, cluster)(kubevirt_vm_info{cluster=~"$cluster"})` + overviewHealthJoin + `)`

// VMsByStatusStat — Running (0_4, query 1)
const overviewVMsByStatusStatRunning = `sum(sum by (cluster, status_group)(kubevirt_vm_info{cluster=~"$cluster", status_group="running"}>0)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusStat — Stopped (0_4, query 2)
const overviewVMsByStatusStatStopped = `sum(sum by (cluster, status_group)(kubevirt_vm_info{cluster=~"$cluster", status_group="non_running"}>0)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusStat — Error (0_4, query 3)
const overviewVMsByStatusStatError = `sum(sum by (cluster, status_group)(kubevirt_vm_info{cluster=~"$cluster", status_group="error"}>0)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusStat — Starting (0_4, query 4)
const overviewVMsByStatusStatStarting = `sum(sum by (cluster, status_group)(kubevirt_vm_info{cluster=~"$cluster", status_group="starting"}>0)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusStat — Migrating (0_4, query 5)
const overviewVMsByStatusStatMigrating = `sum(sum by (cluster, status_group)(kubevirt_vm_info{cluster=~"$cluster", status_group="migrating"}>0)` + overviewHealthJoin + `) or vector(0)`

// ClustersWarningHealth (0_5)
const overviewClustersWarningHealth = `count(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"} == 1` + overviewHealthAnd + `) or vector(0)`

// VMsStartedLast7Days (0_6)
const overviewVMsStartedLast7Days = `count by (cluster)(time() - (label_replace(max by (cluster, name, namespace)(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster"})>0 ,"status","starting","","")) < 604800)` + overviewHealthAnd

// ClustersByOperatorVersion — counts (0_7, query 1)
const overviewClustersByOperatorVersionCounts = `count by (version)((count by (cluster, version) (csv_succeeded{name=~".*hyperconverged.*",cluster=~"$cluster"}>0))` + overviewHealthJoin + `) or (count by (version,phase, reason)((count by (cluster, version,phase, reason) (csv_abnormal{name=~".*hyperconverged.*",cluster=~"$cluster"}>0))` + overviewHealthJoin + `))`

// ClustersByOperatorVersion — percent (0_7, query 2)
const overviewClustersByOperatorVersionPercent = `count by (version)(
    (count by (cluster, version) (csv_succeeded{name=~".*hyperconverged.*",cluster=~"$cluster"}))` + overviewHealthJoin + `
) / scalar(count(count by (cluster)(csv_succeeded{name=~".*hyperconverged.*",cluster=~"$cluster"}) ` + overviewHealthAnd + `))`

// ClustersByOpenShiftVersion — counts (0_8, query 1)
const overviewClustersByOpenShiftVersionCounts = `count by (openshiftVersion)(
label_replace(sum by (name, openshiftVersion) (acm_managed_cluster_labels{name=~"$cluster"}), "cluster", "$1", "name", "(.*)") + 
` + overviewHealthFilteredClusters + `
)`

// ClustersByOpenShiftVersion — percent (0_8, query 2)
const overviewClustersByOpenShiftVersionPercent = `count by (openshiftVersion)(
label_replace(sum by (name, openshiftVersion) (acm_managed_cluster_labels{name=~"$cluster"}), "cluster", "$1", "name", "(.*)") + 
` + overviewHealthFilteredClusters + `
) / scalar(count(
label_replace(sum by (name, openshiftVersion) (acm_managed_cluster_labels{name=~"$cluster"}), "cluster", "$1", "name", "(.*)") + 
` + overviewHealthFilteredClusters + `
))`

// OperatorHealthByCluster — health status (1_0, query 1); $operator_health is substituted by Perses.
const overviewOperatorHealthByClusterStatus = `(sum(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health) by (cluster))`

// OperatorHealthByCluster — critical alerts (1_0, query 2)
const overviewOperatorHealthByClusterCriticalAlerts = `(sum(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health) by (cluster))*0 + on (cluster) group_left() (sum by (cluster)(ALERTS{kubernetes_operator_part_of="kubevirt", alertstate="firing",cluster=~"$cluster",operator_health_impact="critical"}) or 
 (sum by (cluster)(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}*0)))`

// OperatorHealthByCluster — warning alerts (1_0, query 3)
const overviewOperatorHealthByClusterWarningAlerts = `(sum(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health) by (cluster))*0 + on (cluster) group_left() (sum by (cluster)(ALERTS{kubernetes_operator_part_of="kubevirt", alertstate="firing",cluster=~"$cluster",operator_health_impact="warning"}) or (sum by (cluster)(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}*0)))`

// OperatorHealthByCluster — running VMs (1_0, query 4)
const overviewOperatorHealthByClusterRunningVMs = `(sum(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health) by (cluster))*0 + on (cluster) group_left() (sum by (cluster)(cnv:vmi_status_running:count{cluster=~"$cluster"})) or ((sum(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"}$operator_health) by (cluster))*0)`

// OperatorHealthByCluster — HCO system health (1_0, query 5)
const overviewOperatorHealthByClusterHCO = `sum by (cluster)(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"} $operator_health) * 0 
+ on(cluster) (sum by (cluster)(kubevirt_hco_system_health_status{cluster=~"$cluster"}))`

// RunningVMsByOS — known guest OS (2_0, query 1)
const overviewRunningVMsByOSKnown = `sum by (os)(sum by (cluster, os)(label_replace(kubevirt_vmi_info{cluster=~"$cluster", guest_os_name!="", phase="running"}, "os", "$1", "guest_os_name", "(.*)")
+ on (cluster, namespace, name) group_left()(0*(kubevirt_vm_info{cluster=~"$cluster", status_group="running"}>0)))` + overviewHealthJoin + `)`

// RunningVMsByOS — unknown guest OS (2_0, query 2)
const overviewRunningVMsByOSUnknown = `sum by (os)(sum by (cluster, os)(label_replace(kubevirt_vmi_info{cluster=~"$cluster", guest_os_name="", phase="running"}, "os", "unknown", "guest_os_name", "")
+ on (cluster, namespace, name) group_left()(0*(kubevirt_vm_info{cluster=~"$cluster", status_group="running"}>0)))` + overviewHealthJoin + `)`

// RunningVMsByClusterTop20 (2_1)
const overviewRunningVMsByClusterTop20 = `topk(20, sum by (cluster) (kubevirt_vm_info{cluster=~"$cluster", status_group="running"} > 0))` + overviewHealthJoin

// VMsByStatusTS — running (2_2, query 1)
const overviewVMsByStatusTSRunning = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="running"}>0) by (name, namespace, cluster)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusTS — starting (2_2, query 2)
const overviewVMsByStatusTSStarting = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="starting"}>0) by (name, namespace, cluster)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusTS — migrating (2_2, query 3)
const overviewVMsByStatusTSMigrating = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="migrating"}>0) by (name, namespace, cluster)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusTS — error (2_2, query 4)
const overviewVMsByStatusTSError = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="error"}>0) by (name, namespace, cluster)` + overviewHealthJoin + `) or vector(0)`

// VMsByStatusTS — stopped (2_2, query 5)
const overviewVMsByStatusTSStopped = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="non_running"}>0) by (name, namespace, cluster)` + overviewHealthJoin + `) or vector(0)`

// RunningVMsByNodeTop20 (2_3)
const overviewRunningVMsByNodeTop20 = `topk(20, sum(kubevirt_vmi_phase_count{cluster=~"$cluster",namespace=~".*", phase="running"}) by (cluster,phase,node))` + overviewHealthJoin

// CPUUsageByClusterTop20 (3_0)
const overviewCPUUsageByClusterTop20 = `topk(20, sum(rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster",namespace=~".*"}[10m])) by (cluster))` + overviewHealthJoin

// CPUUsagePercentByClusterTop20 (3_1)
const overviewCPUUsagePercentByClusterTop20 = `topk(20,sum by(cluster)(rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster"}[10m]))/
(sum by(cluster)(rate(node_cpu_seconds_total{cluster=~"$cluster"}[10m]))))` + overviewHealthJoin

// MemoryUsageByClusterTop20 (4_0)
const overviewMemoryUsageByClusterTop20 = `topk(20, sum by (cluster)(
  kubevirt_vmi_memory_available_bytes{cluster=~"$cluster"} -
  kubevirt_vmi_memory_unused_bytes{cluster=~"$cluster"} -
  kubevirt_vmi_memory_cached_bytes{cluster=~"$cluster"}
  )
)` + overviewHealthJoin

// MemoryUsagePercentByClusterTop20 (4_1)
const overviewMemoryUsagePercentByClusterTop20 = `topk(20,(
  sum by (cluster) (
    kubevirt_vmi_memory_available_bytes{cluster=~"$cluster"} -        
    kubevirt_vmi_memory_unused_bytes{cluster=~"$cluster"} - 
    kubevirt_vmi_memory_cached_bytes{cluster=~"$cluster"}
  )
  /
  (sum by (cluster)(label_replace(node_memory_MemTotal_bytes{cluster=~"$cluster"}, "node", "$1", "instance", "(.*)")))
 )
)` + overviewHealthJoin

// NetworkReceivedBytesByClusterTop20 (5_0)
const overviewNetworkReceivedBytesByClusterTop20 = `topk(20,sum(rate(kubevirt_vmi_network_receive_bytes_total{cluster=~"$cluster"}[10m])) by (cluster))` + overviewHealthJoin

// NetworkTransmittedBytesByClusterTop20 (5_1)
const overviewNetworkTransmittedBytesByClusterTop20 = `topk(20,sum(rate(kubevirt_vmi_network_transmit_bytes_total{cluster=~"$cluster"}[10m])) by (cluster))` + overviewHealthJoin

// StorageTrafficByClusterTop20 (6_0)
const overviewStorageTrafficByClusterTop20 = `topk(20, sum by (cluster)(rate(kubevirt_vmi_storage_read_traffic_bytes_total{cluster=~"$cluster"}[10m]) + rate(kubevirt_vmi_storage_write_traffic_bytes_total{cluster=~"$cluster"}[10m])))` + overviewHealthJoin

// StorageIOPsByClusterTop20 (6_1)
const overviewStorageIOPsByClusterTop20 = `topk(20, sum by (cluster) (rate(kubevirt_vmi_storage_iops_read_total{cluster=~"$cluster"}[10m]) + rate(kubevirt_vmi_storage_iops_write_total{cluster=~"$cluster"}[10m])))` + overviewHealthJoin
