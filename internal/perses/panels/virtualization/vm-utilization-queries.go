package virtualization

const utilizationInfoQuery = vmInfoStatusExpr

const utilizationCPUUsageQuery = `sum by (cluster,namespace,name)(rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationAllocatedVCPUQuery = allocatedCPUMultiVMExpr + statusFilterJoin

const utilizationCPUUsagePercentQuery = `(sum by (cluster,namespace,name)(rate(kubevirt_vmi_cpu_usage_seconds_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))>0) / ` + allocatedCPUMultiVMExpr + statusFilterJoin

const utilizationMemoryUsageQuery = `avg by (cluster,namespace,name) (avg_over_time(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster",namespace=~"$namespace", name=~"$name"}[10m]))` + statusFilterJoin

const utilizationAllocatedMemoryQuery = allocatedMemoryMultiVMExpr + statusFilterJoin

const utilizationMemoryUsagePercentQuery = `(avg by (cluster,namespace,name) (avg_over_time(kubevirt_vmi_memory_used_bytes{cluster=~"$cluster",namespace=~"$namespace", name=~"$name"}[10m]))) / ` + allocatedMemoryMultiVMExpr + statusFilterJoin

const utilizationNetworkTrafficQuery = `sum by (cluster,namespace,name)(rate(kubevirt_vmi_network_transmit_bytes_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]) + rate(kubevirt_vmi_network_receive_bytes_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationStorageTrafficQuery = `sum by (cluster, namespace, name)(rate(kubevirt_vmi_storage_read_traffic_bytes_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]) + rate(kubevirt_vmi_storage_write_traffic_bytes_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationStorageIOPsQuery = `sum by (cluster,namespace,name) (rate(kubevirt_vmi_storage_iops_read_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]) + rate(kubevirt_vmi_storage_iops_write_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationCPUDelayQuery = `sum by (cluster,namespace,name) (rate(kubevirt_vmi_vcpu_delay_seconds_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationCPUDelayPercentQuery = `(sum by (name, namespace, cluster)(rate(kubevirt_vmi_vcpu_delay_seconds_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))>0) / ` + allocatedCPUMultiVMExpr + statusFilterJoin

const utilizationCPUIOWaitQuery = `sum by (cluster,namespace,name)(rate(kubevirt_vmi_vcpu_wait_seconds_total{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationMemorySwapQuery = `sum by (cluster,namespace,name) (avg_over_time(kubevirt_vmi_memory_swap_in_traffic_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]) + avg_over_time(kubevirt_vmi_memory_swap_out_traffic_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationFSCapacityQuery = `avg by (cluster, namespace, name) (avg_over_time(kubevirt_vmi_filesystem_capacity_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationFSUsageQuery = `avg by (cluster, namespace, name) (avg_over_time(kubevirt_vmi_filesystem_used_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m]))` + statusFilterJoin

const utilizationFSUsageMaxPercentQuery = `max by (cluster, namespace, name) (
  (max by (cluster, namespace, name, disk_name) (avg_over_time(kubevirt_vmi_filesystem_used_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m])))
  /
  (max by (cluster, namespace, name, disk_name) (avg_over_time(kubevirt_vmi_filesystem_capacity_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}[10m])))
)` + statusFilterJoin

const utilizationGuestAgentQuery = `sum by (cluster, namespace, name)(kubevirt_vmi_memory_available_bytes{cluster=~"$cluster", namespace=~"$namespace", name=~"$name"}*0+1)` + statusFilterJoin
