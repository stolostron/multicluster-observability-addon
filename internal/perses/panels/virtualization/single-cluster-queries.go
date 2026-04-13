package virtualization

// Cluster-scoped allocated resource expressions (used only by this dashboard).
const allocatedCPUClusterExpr = `(
sum by (cluster, namespace, name)(kubevirt_vm_resource_requests{cluster=~"$cluster", resource="cpu", unit="cores", source="guest_effective"})
or
sum by (cluster, namespace, name)(
kubevirt_vm_resource_requests{cluster=~"$cluster", resource="cpu", unit="cores", source=~"default|domain"}
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", resource="cpu", unit="sockets", source=~"default|domain"})
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", resource="cpu", unit="threads", source=~"default|domain"})
) or
sum by (cluster, namespace, name)(
kubevirt_vm_resource_requests{cluster=~"$cluster", resource="cpu", unit="cores", source=~"default|domain"}
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", resource="cpu", unit="sockets", source=~"default|domain"})
) or
sum by (cluster, namespace, name)(
kubevirt_vm_resource_requests{cluster=~"$cluster", resource="cpu", unit="cores", source=~"default|domain"})
)`

const allocatedMemoryClusterExpr = `(
sum by (cluster, namespace, name)(kubevirt_vm_resource_requests{cluster=~"$cluster", resource="memory", source="guest_effective"})
or
max by (cluster, namespace, name)(kubevirt_vm_resource_requests{cluster=~"$cluster", resource="memory", source=~"default|domain"})
)`

// PromQL queries for acm-openshift-virtualization-single-cluster-view.

// Cluster / inventory
const (
	singleClusterClusterName          = `kubevirt_hyperconverged_operator_health_status{name=~".*hyperconverged.*", cluster=~"$cluster"}`
	singleClusterOpenshiftVirtVersion = `csv_succeeded{name=~".*hyperconverged.*", cluster=~"$cluster"}`
	singleClusterTotalNodes           = `sum(count by (cluster) (count(kube_node_status_allocatable{resource=~".*kubevirt.*", cluster=~"$cluster"}) by (cluster,node)))`
	singleClusterTotalVMs             = totalVMsExpr
	singleClusterVMStatusRunning      = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="running"}>0) by (name, namespace) or vector(0))`
	singleClusterVMStatusStopped      = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="non_running"}>0) by (name, namespace) or vector(0))`
	singleClusterVMStatusError        = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="error"}>0) by (name, namespace) or vector(0))`
	singleClusterVMStatusStarting     = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="starting"}>0) by (name, namespace) or vector(0))`
	singleClusterVMStatusMigrating    = `sum(sum(kubevirt_vm_info{cluster=~"$cluster", status_group="migrating"}>0) by (name, namespace) or vector(0))`
	singleClusterProviderByCloud      = `count(acm_managed_cluster_labels{name=~"$cluster"}) by (cloud)`
	singleClusterOpenshiftVersion     = `count by (version)(count by (cluster, version) (csv_succeeded{name=~".*hyperconverged.*",cluster=~"$cluster"}>0))
or(count by (version)(count by (cluster, version) (csv_abnormal{name=~".*hyperconverged.*",cluster=~"$cluster"}>0)))`
	singleClusterOperatorHealthStatus  = `sum by (cluster)(kubevirt_hyperconverged_operator_health_status{cluster=~"$cluster"})`
	singleClusterHCOSystemHealthStatus = `sum by (cluster) (kubevirt_hco_system_health_status{cluster=~"$cluster"})`
	singleClusterRunningVMsByOS1       = `sum by (os)(sum by (cluster, os)(label_replace(kubevirt_vmi_info{cluster=~"$cluster", guest_os_name!="", phase="running"}, "os", "$1", "guest_os_name", "(.*)")
+ on (cluster, namespace, name) group_left()(0*(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster"}>0))))`
	singleClusterRunningVMsByOS2 = `sum by (os)(sum by (cluster, os)(label_replace(kubevirt_vmi_info{cluster=~"$cluster", guest_os_name="", phase="running"}, "os", "unknown", "guest_os_name", "")
+ on (cluster, namespace, name) group_left()(0*(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster"}>0))))`
	singleClusterRecentVMsStarted = `bottomk(5, sum by (name, namespace) (sort(time() - (kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster"}>0)) / 1 ))`
)

// VM details
const (
	singleClusterVMsRunningByNode     = `topk(20, sum(kubevirt_vmi_phase_count{cluster=~"$cluster",namespace=~".*", phase=~"running"}) by (cluster,phase,node))`
	singleClusterVMsByStatusStarting  = `sum(count(kubevirt_vm_starting_status_last_transition_timestamp_seconds{cluster=~"$cluster"} > 0)) or vector(0)`
	singleClusterVMsByStatusRunning   = `sum(count(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster"} > 0)) or vector(0)`
	singleClusterVMsByStatusMigrating = `sum(count(kubevirt_vm_migrating_status_last_transition_timestamp_seconds{cluster=~"$cluster"} > 0)) or vector(0)`
	singleClusterVMsByStatusError     = `sum(count(kubevirt_vm_error_status_last_transition_timestamp_seconds{cluster=~"$cluster"} > 0)) or vector(0)`
	singleClusterVMsByStatusStopped   = `sum(count(kubevirt_vm_non_running_status_last_transition_timestamp_seconds{cluster=~"$cluster"} > 0)) or vector(0)`
)

// CPU
const (
	singleClusterNodesCPUUtilizationRate1m = `topk(20,sum(label_replace(instance:node_cpu_utilisation:rate1m{cluster=~"$cluster"}, "node", "$1", "instance", "(.*)")) by (node, cluster)
+ on(node, cluster) group_left()
0*sum(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster"}) by (node, cluster))`
	singleClusterVMsTotalCPUUsage     = `topk(20,sum by (namespace, name) (rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster",namespace=~".*"}[10m])))`
	singleClusterNodesCPUUsagePercent = `topk(20, (sum by (node, cluster)(label_replace(rate(node_cpu_seconds_total{cluster=~"$cluster", mode!="idle"}[10m]), "node", "$1", "instance", "(.*)")) 
  / 
  sum by (node, cluster)(label_replace(rate(node_cpu_seconds_total{cluster=~"$cluster"}[10m]), "node", "$1", "instance", "(.*)"))
+ on(node, cluster) group_left()
0*sum by (node, cluster)(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster"}))
)`
	singleClusterVMsCPUUsagePercent = `topk(20, 
  sum by (cluster, name, namespace) (rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster"}[10m]))
  / ` + allocatedCPUClusterExpr + `
)`
	singleClusterNodesCPUStealPercent = `topk(20, (sum by (node, cluster)(label_replace(rate(node_cpu_seconds_total{cluster=~"$cluster", mode="steal"}[10m]), "node", "$1", "instance", "(.*)")) 
  / 
  sum by (node, cluster)(label_replace(rate(node_cpu_seconds_total{cluster=~"$cluster"}[10m]), "node", "$1", "instance", "(.*)"))
+ on(node, cluster) group_left()
0*sum by (node, cluster)(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster"}))
)`
	singleClusterVMsCPUReadyPercent = `topk(20, sum by (name, namespace, cluster) (
  sum by (name, namespace, cluster)(rate(kubevirt_vmi_vcpu_delay_seconds_total{cluster=~"$cluster", namespace=~".*"}[10m]))
  / ` + allocatedCPUClusterExpr + `
  )
)`
)

// Memory
const (
	singleClusterNodesMemoryUsageBytes = `topk(20,sum(label_replace(instance:node_memory_utilisation:ratio{cluster=~"$cluster"}, "node", "$1", "instance", "(.*)")) by (node, cluster)
* sum by (node, cluster)(label_replace(node_memory_MemTotal_bytes{cluster=~"$cluster"}, "node", "$1", "instance", "(.*)"))
+ on(node, cluster) group_left()
0*sum(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster"}) by (node, cluster)
)`
	singleClusterVMsMemoryUsageBytes = `topk(20,(sum by (namespace, name) (
kubevirt_vmi_memory_available_bytes{cluster=~"$cluster",namespace=~".*"} - kubevirt_vmi_memory_unused_bytes{cluster=~"$cluster",namespace=~".*"} -kubevirt_vmi_memory_cached_bytes{cluster=~"$cluster",namespace=~".*"})))`
	singleClusterNodesMemoryUsagePercent = `topk(20,(
  sum(label_replace(instance:node_memory_utilisation:ratio{cluster=~"$cluster"}, "node", "$1", "instance", "(.*)")) by (node, cluster)
+ on(node, cluster) group_left()
0*sum(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster"}) by (node, cluster)
 )
)`
	singleClusterVMsMemoryUsagePercent = `topk(20,(
  sum by (namespace, name) (
    kubevirt_vmi_memory_available_bytes{cluster=~"$cluster",namespace=~".*"} -        
    kubevirt_vmi_memory_unused_bytes{cluster=~"$cluster",namespace=~".*"} - 
    kubevirt_vmi_memory_cached_bytes{cluster=~"$cluster",namespace=~".*"}
  )
  / ` + allocatedMemoryClusterExpr + `
 )
)`
)

// Network
const (
	singleClusterNodesNetworkReceivedRate = `topk(20,sum(label_replace(instance:node_network_receive_bytes_excluding_lo:rate1m{cluster=~"$cluster"}, "node", "$1", "instance", "(.*)")) by (node, cluster)
+ on(node, cluster) group_left()
0*sum(kubevirt_vmi_network_receive_packets_total{cluster=~"$cluster"}) by (node, cluster))`
	singleClusterVMsNetworkReceivedRate   = `topk(20,sum(rate(kubevirt_vmi_network_receive_bytes_total{cluster=~"$cluster"}[10m])) by (namespace, name))`
	singleClusterNodesNetworkTransmitRate = `topk(20,sum(label_replace(instance:node_network_transmit_bytes_excluding_lo:rate1m{cluster=~"$cluster"}, "node", "$1", "instance", "(.*)")) by (node, cluster)
+ on(node, cluster) group_left()
0*sum(kubevirt_vmi_network_transmit_packets_total{cluster=~"$cluster"}) by (node, cluster))`
	singleClusterVMsNetworkTransmitRate = `topk(20,sum(rate(kubevirt_vmi_network_transmit_bytes_total{cluster=~"$cluster"}[10m])) by (namespace, name))`
)

// Storage
const (
	singleClusterNodesStorageIOPS    = `topk(20,sum by (node) (rate(kubevirt_vmi_storage_iops_read_total{cluster=~"$cluster"}[10m]) + rate(kubevirt_vmi_storage_iops_write_total{cluster=~"$cluster"}[10m])))`
	singleClusterVMsStorageIOPS      = `topk(20,sum by (namespace, name) (rate(kubevirt_vmi_storage_iops_read_total{cluster=~"$cluster"}[10m]) + rate(kubevirt_vmi_storage_iops_write_total{cluster=~"$cluster"}[10m])))`
	singleClusterNodesStorageTraffic = `topk(20,sum by (node) (rate(kubevirt_vmi_storage_read_traffic_bytes_total{cluster=~"$cluster"}[10m]) + rate(kubevirt_vmi_storage_write_traffic_bytes_total{cluster=~"$cluster"}[10m])))`
	singleClusterVMsStorageTraffic   = `topk(20,sum by (namespace, name) (rate(kubevirt_vmi_storage_read_traffic_bytes_total{cluster=~"$cluster"}[10m]) + rate(kubevirt_vmi_storage_write_traffic_bytes_total{cluster=~"$cluster"}[10m])))`
)

// Alerts
const (
	singleClusterAlertsCritical             = `sum (ALERTS{kubernetes_operator_part_of="kubevirt", alertstate="firing",cluster=~"$cluster",severity="critical"}) or vector(0)`
	singleClusterAlertsWarning              = `sum (ALERTS{kubernetes_operator_part_of="kubevirt", alertstate="firing",cluster=~"$cluster",severity="warning",operator_health_impact=~"$health_impact"}) or vector(0)`
	singleClusterAlertsInfo                 = `sum (ALERTS{kubernetes_operator_part_of="kubevirt", alertstate="firing",cluster=~"$cluster",severity="info"}) or vector(0)`
	singleClusterOperatorHealthImpactAlerts = `sum(ALERTS{cluster=~"$cluster",alertstate="firing",kubernetes_operator_part_of="kubevirt",operator_health_impact=~"$health_impact",operator_health_impact!="none", severity=~"$severity", severity!="info"}) by (cluster,alertname,alertstate,severity,namespace,name,operator_health_impact)`
	singleClusterOperatorCSVAbnormal        = `count by (version,phase, reason) (csv_abnormal{name=~".*hyperconverged.*",cluster=~"$cluster"}>0)`
	singleClusterAllAlerts                  = `sum(ALERTS{cluster=~"$cluster",alertstate="firing",kubernetes_operator_part_of="kubevirt",operator_health_impact=~"$health_impact", severity=~"$severity"}) by (cluster,alertname,alertstate,severity,namespace,name,operator_health_impact)`
)
