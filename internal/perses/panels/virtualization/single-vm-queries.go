package virtualization

const singleVMMemoryUsedExpr = `sum by (name) (
kubevirt_vmi_memory_available_bytes{cluster=~"$cluster",namespace="$namespace", name="$name"} - kubevirt_vmi_memory_unused_bytes{cluster=~"$cluster",namespace="$namespace", name="$name"} -kubevirt_vmi_memory_cached_bytes{cluster=~"$cluster",namespace="$namespace", name="$name"})`

// Status & alert stats
const (
	singleVMStatusQuery = `label_replace(sum(kubevirt_vm_info{cluster="$cluster",namespace="$namespace",name="$name",status_group="running"}>0) * 0 + 1, "status", "Running", "", "")
or label_replace(sum(kubevirt_vm_info{cluster="$cluster",namespace="$namespace",name="$name",status_group="non_running"}>0) * 0 + 1, "status", "Stopped", "", "")
or label_replace(sum(kubevirt_vm_info{cluster="$cluster",namespace="$namespace",name="$name",status_group="starting"}>0) * 0 + 1, "status", "Starting", "", "")
or label_replace(sum(kubevirt_vm_info{cluster="$cluster",namespace="$namespace",name="$name",status_group="migrating"}>0) * 0 + 1, "status", "Migrating", "", "")
or label_replace(sum(kubevirt_vm_info{cluster="$cluster",namespace="$namespace",name="$name",status_group="error"}>0) * 0 + 1, "status", "Error", "", "")`

	singleVMCriticalAlertsQuery = `sum(sum by (cluster,alertname,alertstate,severity,namespace,name,operator_health_impact )(ALERTS{cluster=~"$cluster",kubernetes_operator_part_of="kubevirt", alertstate="firing", pod=~"virt-launcher-$name.*", severity="critical"})) or vector(0)`

	singleVMWarningAlertsQuery = `sum(sum by (cluster,alertname,alertstate,severity,namespace,name,operator_health_impact )(ALERTS{cluster=~"$cluster",kubernetes_operator_part_of="kubevirt", alertstate="firing", pod=~"virt-launcher-$name.*", severity="warning"}))  or vector(0)`

	singleVMInfoAlertsQuery = `sum(sum by (cluster,alertname,alertstate,severity,namespace,name,operator_health_impact )(ALERTS{cluster=~"$cluster",kubernetes_operator_part_of="kubevirt", alertstate="firing", pod=~"virt-launcher-$name.*", severity="info"})) or vector(0)`
)

// Shared allocation expressions: prefer guest_effective, fall back to old source labels.
const singleVMAllocatedCPUExpr = `(
sum by (name)(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="cpu", unit="cores", source="guest_effective"})
or
sum by (name)(
(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="cpu", unit="cores", source=~"default|domain"})
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="cpu", unit="sockets", source=~"default|domain"})
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="cpu", unit="threads", source=~"default|domain"})
) or
sum by (name)(
(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="cpu", unit="cores", source=~"default|domain"})
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="cpu", unit="sockets", source=~"default|domain"})
) or
sum by (name)(
kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="cpu", unit="cores", source=~"default|domain"})
)`

const singleVMAllocatedMemoryExpr = `(
sum by (name)(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="memory", source="guest_effective"})
or
sum by (name)(kubevirt_vm_resource_requests{cluster=~"$cluster", name="$name", namespace="$namespace", resource="memory", source=~"default|domain"})
)`

// Gauge / ratio queries
const (
	singleVMMemoryUsagePercentGaugeQuery = `(` + singleVMMemoryUsedExpr + `)
/` + singleVMAllocatedMemoryExpr

	singleVMCPUUsagePercentRatioQuery = `(sum by (name)(rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster", name="$name", namespace="$namespace"}[10m]))>0)
/` + singleVMAllocatedCPUExpr

	singleVMCPUDelayPercentRatioQuery = `(sum by (name)(rate(kubevirt_vmi_vcpu_delay_seconds_total{cluster=~"$cluster", name="$name", namespace="$namespace"}[10m]))>0)
/` + singleVMAllocatedCPUExpr
)

// VM Information table (0_6)
const (
	singleVMVMInformationNameQuery = `label_replace(sum by (name)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name"}), "Field", "Name", "name", ".*")`

	singleVMVMInformationStatusQuery = `label_replace(sum by (status)(label_replace(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster="$cluster",namespace="$namespace",name="$name"}>0,"status", "Running", "", "") or label_replace(kubevirt_vm_non_running_status_last_transition_timestamp_seconds{cluster="$cluster",namespace="$namespace",name="$name"}>0,"status", "Stopped", "", "") or label_replace(kubevirt_vm_error_status_last_transition_timestamp_seconds{cluster="$cluster",namespace="$namespace",name="$name"}>0,"status", "Error", "", "") or label_replace(kubevirt_vm_starting_status_last_transition_timestamp_seconds{cluster="$cluster",namespace="$namespace",name="$name"}>0,"status", "Starting", "", "") or label_replace(kubevirt_vm_migrating_status_last_transition_timestamp_seconds{cluster="$cluster",namespace="$namespace",name="$name"}>0,"status", "Migrating", "", "") ), "Field", "Status", "name", ".*")`

	singleVMVMInformationGuestOSQuery = `label_replace(sum by (guest_os_name)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name", guest_os_name!=""} or kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name", guest_os_name!=""}), "Field", "Operating System", "name", ".*")`

	singleVMVMInformationGuestOSVersionQuery = `label_replace(sum by (guest_os_version_id)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name", guest_os_version_id!=""} or kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name", guest_os_version_id!=""}), "Field", "Operating System Version", "name", ".*")`

	singleVMVMInformationAllocatedCPUQuery = `label_replace(` + singleVMAllocatedCPUExpr + `, "Field", "Allocated CPU", "", ".*")`

	singleVMVMInformationAllocatedMemoryQuery = `label_replace(` + singleVMAllocatedMemoryExpr + ` / (1024^3), "Field", "Allocated Memory (GB)", "", ".*")`

	singleVMVMInformationAllocatedDiskQuery = `label_replace(sum(kubevirt_vm_disk_allocated_size_bytes{cluster="$cluster", namespace="$namespace", name="$name"})/ (1024^3), "Field", "Allocated Disk (GB)", "", ".*")`

	singleVMVMInformationInstanceTypeQuery = `label_replace(sum by (instance_type)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name", instance_type!="<none>"} or kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name", instance_type!="<none>"}), "Field", "Instance Type", "name", ".*")`

	singleVMVMInformationWorkloadQuery = `label_replace(sum by (workload)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name", workload!="<none>"}) or sum by (workload)(kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name", workload!="<none>"}), "Field", "Template Workload", "name", ".*")`

	singleVMVMInformationFlavorQuery = `label_replace(sum by (flavor)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name", flavor!="<none>"}) or sum by (flavor)(kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name", flavor!="<none>"}), "Field", "Template Flavor", "name", ".*")`
)

// General Information table (0_7)
const (
	singleVMGeneralInformationNamespaceQuery = `label_replace(sum by (namespace)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name"}), "Field", "Namespace", "name", ".*")`

	singleVMGeneralInformationNodeQuery = `label_replace(sum by (node)(kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name"}), "Field", "Node", "name", ".*")`

	singleVMGeneralInformationPodQuery = `label_replace(sum by(pod)(kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name"}), "Field", "Pod", "name", ".*")`

	singleVMGeneralInformationEvictableQuery = `label_replace(sum by (evictable)(kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name"}), "Field", "Evictable", "name", ".*")`

	singleVMGeneralInformationMachineTypeQuery = `label_replace(sum by (machine_type)(kubevirt_vm_info{cluster="$cluster", namespace="$namespace", name="$name", machine_type!=""}) or sum by (machine_type)(kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name", machine_type!=""}), "Field", "Machine Type", "name", ".*")`

	singleVMGeneralInformationOutdatedQuery = `label_replace(sum by (outdated)(kubevirt_vmi_info{cluster="$cluster", namespace="$namespace", name="$name"}), "Field", "Outdated", "name", ".*")`
)

// Network & snapshots
const (
	singleVMNetworkAddressesQuery = `sum by(cluster,name,namespace,address,network_name, type)(kubevirt_vmi_status_addresses{cluster="$cluster",namespace="$namespace",name="$name"})`

	singleVMSnapshotsQuery = `kubevirt_vmsnapshot_succeeded_timestamp_seconds{cluster=~"$cluster",namespace="$namespace", name="$name"}*1000`
)

// CPU time series
const (
	singleVMTotalCPUUsageQuery = `sum by (name) (rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster",namespace="$namespace", name="$name"}[10m]))`

	singleVMCPUReadyTimeQuery = `(sum by (name, namespace, cluster)(rate(kubevirt_vmi_vcpu_delay_seconds_total{cluster=~"$cluster", namespace="$namespace", name="$name"}[10m]))>0)
/` + singleVMAllocatedCPUExpr
)

// Memory time series
const (
	singleVMMemoryUsageBytesQuery = singleVMMemoryUsedExpr

	singleVMMemoryUsagePercentTimeSeriesQuery = `(` + singleVMMemoryUsedExpr + `)/` + singleVMAllocatedMemoryExpr
)

// Network time series
const (
	singleVMNetworkTransmitQuery = `sum(rate(kubevirt_vmi_network_transmit_bytes_total{cluster=~"$cluster",namespace="$namespace", name="$name"}[10m])) by (name)`

	singleVMNetworkReceiveQuery = `sum(rate(kubevirt_vmi_network_receive_bytes_total{cluster=~"$cluster",namespace="$namespace", name="$name"}[10m])) by (name)`

	singleVMNetworkTransmitPacketsDroppedQuery = `sum(rate(kubevirt_vmi_network_transmit_packets_dropped_total{cluster=~"$cluster",namespace="$namespace", name="$name"}[10m])) by (name)`

	singleVMNetworkReceivePacketsDroppedQuery = `sum(rate(kubevirt_vmi_network_receive_packets_dropped_total{cluster=~"$cluster",namespace="$namespace", name="$name"}[10m])) by (name)`
)

// Storage time series
const (
	singleVMStorageTrafficQuery = `sum by (name)(rate(kubevirt_vmi_storage_read_traffic_bytes_total{cluster="$cluster",namespace="$namespace", name="$name"}[10m]) + rate(kubevirt_vmi_storage_write_traffic_bytes_total{cluster="$cluster",namespace="$namespace", name="$name"}[10m]))`

	singleVMStorageIOPsQuery = `sum by (name) (rate(kubevirt_vmi_storage_iops_read_total{cluster=~"$cluster",namespace="$namespace", name="$name"}[10m]) + rate(kubevirt_vmi_storage_iops_write_total{cluster=~"$cluster",namespace="$namespace", name="$name"}[10m]))`
)

// File system
const (
	singleVMFilesystemUsedBytesQuery = `max by (disk_name) (
  avg_over_time(kubevirt_vmi_filesystem_used_bytes{
    cluster="$cluster", 
    namespace="$namespace", 
    name="$name"
  }[10m])
)`

	singleVMFilesystemUsagePercentQuery = `(max by (disk_name) (
  avg_over_time(kubevirt_vmi_filesystem_used_bytes{
    cluster="$cluster", 
    namespace="$namespace", 
    name="$name"
  }[10m])
))
/
(max by (disk_name) (
  avg_over_time(kubevirt_vmi_filesystem_capacity_bytes{
    cluster="$cluster", 
    namespace="$namespace", 
    name="$name"
  }[10m])
))`
)

// VM Alerts table
const singleVMVMAlertsTableQuery = `sum by (cluster,alertname,alertstate,severity,namespace,name,operator_health_impact )(ALERTS{cluster=~"$cluster",kubernetes_operator_part_of="kubevirt", alertstate="firing", pod=~"virt-launcher-$name.*"})`
