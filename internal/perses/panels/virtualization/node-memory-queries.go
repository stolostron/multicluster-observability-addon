package virtualization

// ClusterPressure
const (
	nodeMemoryClusterPressureWaiting = `sum(rate(node_pressure_memory_waiting_seconds_total{cluster=~"$cluster", instance=~"$node"}[10m]))`
	nodeMemoryClusterPressureStalled = `sum(rate(node_pressure_memory_stalled_seconds_total{cluster=~"$cluster", instance=~"$node"}[10m]))`
)

// ClusterUtilizationHistory
const (
	nodeMemoryClusterUtilizationHistoryCapacity = `sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))`

	nodeMemoryClusterUtilizationHistoryUtilWithVirt = `sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
)`

	nodeMemoryClusterUtilizationHistoryUtilWithoutVirt = `sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
) - sum(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster", node=~"$node"})`

	nodeMemoryClusterUtilizationHistoryPlanRequests = `(sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})) - sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})))
+
sum(kube_pod_resource_request{cluster=~"$cluster", resource="memory", node=~"$node"})`
)

// ClusterUtilizationHistorySummary
const (
	nodeMemoryClusterUtilizationHistorySummaryAllocatablePlusSwap = `sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})) + sum(label_replace(node_memory_SwapTotal_bytes{cluster=~"$cluster", instance=~"$node"}, "node", "$1", "instance", "(.+)"))`

	nodeMemoryClusterUtilizationHistorySummaryAllocatable = `sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))`

	nodeMemoryClusterUtilizationHistorySummaryPlanVMAssigned = `# all used - system reserved + plan vm
sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
)
-
(sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})) - sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})))

-
sum(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster", node=~"$node"})

+
sum(kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster", node=~"$node"})`

	nodeMemoryClusterUtilizationHistorySummaryUtilizationCluster = `sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
(sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})) - sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})))`

	nodeMemoryClusterUtilizationHistorySummaryPlanMaxVMAssigned = `(kubevirt_hco_memory_overcommit_percentage{cluster=~"$cluster"} / 100)

*

sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))`
)

// ClusterUtilizationNow (physical memory utilization ratio)
const nodeMemoryClusterUtilizationNowPhysicalRatio = `(
sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
(sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})) - sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})))
)

/

(
sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)`

// ClusterVirtualCommittedHistory
const (
	nodeMemoryClusterVirtualCommittedHistoryNodeCapacity = `sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))`

	nodeMemoryClusterVirtualCommittedHistoryPlanVMAssigned = `sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
(sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})) - sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})))
+
sum(kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster", node=~"$node"})`

	nodeMemoryClusterVirtualCommittedHistoryUtilWithoutVirt = `sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster", node=~"$node"})`
)

// ClusterVirtualCommittedNow (virtual memory commitment ratio)
const nodeMemoryClusterVirtualCommittedNowRatio = `(
sum((label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
(sum((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})) - sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})))
+
sum(kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster", node=~"$node"})
)

/

(
sum((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)`

// NodePressureHistory
const nodeMemoryNodePressureHistory = `sum by (instance) (rate(node_pressure_memory_waiting_seconds_total{cluster=~"$cluster", instance=~"$node"}[10m]))`

// NodePressureMaxNow
const nodeMemoryNodePressureMaxNow = `topk(1, (sum by (instance) (rate(node_pressure_memory_waiting_seconds_total{cluster=~"$cluster", instance=~"$node"}[10m]@end()))))`

// NodeRequestsHistory
const nodeMemoryNodeRequestsHistory = `sum by (node) (kube_pod_resource_request{cluster=~"$cluster", resource="memory", node=~"$node"})

/ on (node)

(
(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})
)`

// NodeRequestsMinmaxNow
const (
	nodeMemoryNodeRequestsMinNow = `min(
sum by (node) (kube_pod_resource_request{cluster=~"$cluster", resource="memory", node=~"$node"})

/ on (node)

(
(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})
)
)`

	nodeMemoryNodeRequestsMaxNow = `max(
sum by (node) (kube_pod_resource_request{cluster=~"$cluster", resource="memory", node=~"$node"})

/ on (node)

(
(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})
)
)`
)

// NodeSystemExceedsReservationAlertNow
const nodeMemoryNodeSystemExceedsReservationAlertNow = `(count(ALERTS{cluster=~"$cluster", alertname="SystemMemoryExceedsReservation"}) or vector(0))
/
count(ALERTS{cluster=~"$cluster", alertname="Watchdog"})`

// NodeSystemReservedMinmaxHistory
const (
	nodeMemoryNodeSystemReservedMinHistory = `min(
sum by (node) (container_memory_rss{cluster=~"$cluster", cgroup_id="/system.slice"})

/ on (node)

(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
)`

	nodeMemoryNodeSystemReservedMaxHistory = `max(
sum by (node) (container_memory_rss{cluster=~"$cluster", cgroup_id="/system.slice"})

/ on (node)

(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)

)`
)

// NodeSystemReservedUtilizationHistory
const nodeMemoryNodeSystemReservedUtilizationHistory = `topk(5, (
sum by (node) (container_memory_rss{cluster=~"$cluster", cgroup_id="/system.slice"})

/ on (node)

(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
))`

// NodeUtilizationHistory
const nodeMemoryNodeUtilizationHistory = `(
label_replace((
(label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
), "node", "$1", "instance", "(.+)")
- on (node)
(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
)

/ on (node)

(
(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})
)`

// NodeUtilizationMaxNow
const nodeMemoryNodeUtilizationMaxNow = `(max by () ((
label_replace((
(label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
), "node", "$1", "instance", "(.+)")
- on (node)
(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
)

/ on (node)

(
(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})
))
)`

// NodeUtilizationMinNow
const nodeMemoryNodeUtilizationMinNow = `(min by () ((
label_replace((
(label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
), "node", "$1", "instance", "(.+)")
- on (node)
(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
)

/ on (node)

(
(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})
))
)`

// NumberofrunningVMs
const nodeMemoryNumberOfRunningVMs = `count(kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster"} * on(node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})`

// PlanMinmax (virtual commit level per node)
const (
	nodeMemoryPlanMinVirtualCommitLevel = `(min by () ((

(
(
label_replace((
(label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
), "node", "$1", "instance", "(.+)")
- on (node)
(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
)
+
sum by (node) (kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster"})
)


/

(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})

)))`

	nodeMemoryPlanMaxVirtualCommitLevel = `(max by () ((

(
(
label_replace((
(label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
), "node", "$1", "instance", "(.+)")
- on (node)
(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
)
+
sum by (node) (kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster"})
)


/

(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})

)))`
)

// Swap (total capacity and used bytes per dashboard)
const (
	nodeMemorySwapAvailableBytes = `sum((
label_replace(
  (node_memory_SwapTotal_bytes{cluster=~"$cluster", instance=~"$node"}),
  "node", "$1",
  "instance", "(.+)"
) * on (node) kube_node_role{cluster=~"$cluster", role="$role", node=~"$node"}
)
)`

	nodeMemorySwapUsedBytes = `sum((
label_replace(
  (node_memory_SwapTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_SwapFree_bytes{cluster=~"$cluster", instance=~"$node"}),
  "node", "$1",
  "instance", "(.+)"
) * on (node) kube_node_role{cluster=~"$cluster", role="$role", node=~"$node"}
)
)`
)

// UtilizationDistribution (virtual memory commit level per node)
const nodeMemoryUtilizationDistribution = `(
(
label_replace((
(label_replace((node_memory_MemTotal_bytes{cluster=~"$cluster", instance=~"$node"} - node_memory_MemAvailable_bytes{cluster=~"$cluster", instance=~"$node"}), "node", "$1", "instance", "(.+)") * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"})
), "node", "$1", "instance", "(.+)")
- on (node)
(
sum by (node) ((kube_node_status_capacity{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
-
sum by (node) ((kube_node_status_allocatable{resource="memory", cluster=~"$cluster", node=~"$node"} * on (node) kube_node_role{role="$role", cluster=~"$cluster", node=~"$node"}))
)
)
+
sum by (node) (kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster"})
)


/

(kube_node_status_allocatable{cluster=~"$cluster", resource="memory"} * on (node) kube_node_role{cluster=~"$cluster", node=~"$node", role=~"$role"})`

// VMs (VM overcommit ratio)
const nodeMemoryVMsOvercommitRatio = `(label_replace(kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster"}, "label_vm_kubevirt_io_name", "$1", "name", "(.+)"))

/ on (namespace, label_vm_kubevirt_io_name) group_left()

max by (namespace, label_vm_kubevirt_io_name) (
  kube_pod_resource_request{cluster=~"$cluster", resource="memory"}
  * on(namespace, pod) group_left(label_vm_kubevirt_io_name)
  group by(namespace, pod, label_vm_kubevirt_io_name) (
    kube_pod_labels{cluster=~"$cluster", label_vm_kubevirt_io_name!=""}
  )
)`

// VMvirtualmemoryutiliztaionhostvmurilization (top 10 VM memory used ratio)
const nodeMemoryVMVirtualMemoryUtilizationHostVMRatio = `topk(10, (label_replace(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster"}@end(), "label_vm_kubevirt_io_name", "$1", "name", "(.+)"))

/ on (namespace, label_vm_kubevirt_io_name) group_left()

sum by (namespace, label_vm_kubevirt_io_name) (
  container_memory_usage_bytes{cluster=~"$cluster"}
  * on(namespace, pod) group_left(label_vm_kubevirt_io_name)
  group by(namespace, pod, label_vm_kubevirt_io_name) (
    kube_pod_labels{cluster=~"$cluster", label_vm_kubevirt_io_name!=""}
  )
)
)`

// VmVirtualCommittedNow (average VM overcommit ratio)
const nodeMemoryVMVirtualCommittedNowAvgOvercommit = `avg(
(
(label_replace(kubevirt_vmi_memory_domain_bytes{cluster=~"$cluster"}, "label_vm_kubevirt_io_name", "$1", "name", "(.+)"))
+ on(name, namespace) group_left() kubevirt_vmi_launcher_memory_overhead_bytes{cluster=~"$cluster"}
)

/ on (namespace, label_vm_kubevirt_io_name) group_left()

avg by (namespace, label_vm_kubevirt_io_name) (
  kube_pod_resource_request{cluster=~"$cluster", resource="memory"}
  * on(namespace, pod) group_left(label_vm_kubevirt_io_name)
  group by(namespace, pod, label_vm_kubevirt_io_name) (
    kube_pod_labels{cluster=~"$cluster", label_vm_kubevirt_io_name!=""}
  )

)
)`
