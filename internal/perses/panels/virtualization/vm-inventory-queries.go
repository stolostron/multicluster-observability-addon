package virtualization

const statusFilterJoin = `+on(cluster,name,namespace) group_left()(0*(sum by(cluster,namespace,name)(kubevirt_vm_info{status_group=~"$status"})))`

const inventoryDimensionFilterJoin = `+on(cluster, namespace, name) group_left()(0*sum by (cluster, namespace, name)(kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"} or kubevirt_vmi_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}))`

const inventoryStatusAndDimensionFilterJoin = statusFilterJoin + `
` + inventoryDimensionFilterJoin

const inventoryInfoQuery = vmInfoStatusExpr + `
` + inventoryDimensionFilterJoin

// inventoryLiveMigratableQuery derives live_migratable from
// kubevirt_vmi_non_evictable: 1 = not migratable (false), 0 = migratable (true).
// Only running VMIs expose this metric; stopped VMs will show null in the column.
const inventoryLiveMigratableQuery = `(
  label_replace(sum by (cluster, namespace, name)(kubevirt_vmi_non_evictable{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} == 1), "live_migratable", "false", "__name__", ".*")
  or
  label_replace(sum by (cluster, namespace, name)(kubevirt_vmi_non_evictable{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} == 0), "live_migratable", "true",  "__name__", ".*")
)` + inventoryStatusAndDimensionFilterJoin

const inventoryCPUQuery = allocatedCPUMultiVMExpr + `
` + inventoryStatusAndDimensionFilterJoin

const inventoryMemoryQuery = allocatedMemoryMultiVMExpr + `
` + inventoryStatusAndDimensionFilterJoin

const inventoryGuestAgentQuery = `sum by (cluster, namespace, name)(kubevirt_vmi_memory_available_bytes{cluster=~"$cluster", namespace=~"$namespace", name=~"$name"}*0+1)` + inventoryStatusAndDimensionFilterJoin

const inventoryCreateDateQuery = `sum by (cluster, namespace, name)(kubevirt_vm_create_date_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}*1000)
` + inventoryStatusAndDimensionFilterJoin

const inventoryDiskQuery = `sum by (cluster, namespace, name)(kubevirt_vm_disk_allocated_size_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})` + inventoryStatusAndDimensionFilterJoin

const inventoryFlavorQuery = `sum by (cluster,namespace,name,flavor) (
  kubevirt_vmi_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", flavor!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
  or on(cluster,namespace,name) kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", flavor!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
)` + statusFilterJoin

const inventoryWorkloadQuery = `sum by (cluster,namespace,name,workload) (
  kubevirt_vmi_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", workload!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
  or on(cluster,namespace,name) kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", workload!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
)` + statusFilterJoin

const inventoryInstanceTypeQuery = `sum by (cluster,namespace,name,instance_type) (
  kubevirt_vmi_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", instance_type!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
  or on(cluster,namespace,name) kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", instance_type!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
)` + statusFilterJoin

const inventoryPreferenceQuery = `sum by (cluster,namespace,name,preference) (
  kubevirt_vmi_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", preference!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
  or on(cluster,namespace,name) kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", preference!="<none>",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
)` + statusFilterJoin

const inventoryGuestOSQuery = `sum by (cluster,namespace,name,guest_os_name ,guest_os_version_id, guest_os_arch) (
  kubevirt_vmi_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace",guest_os_name!="" ,guest_os_version_id!="",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
  or on(cluster,namespace,name) kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace",guest_os_name!="" ,guest_os_version_id!="",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
)` + statusFilterJoin

const inventoryEvictableOutdatedQuery = `sum by (cluster,namespace,name,outdated ,evictable) (
  kubevirt_vmi_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
  or on(cluster,namespace,name) kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace",flavor=~"$flavor",workload=~"$workload", instance_type=~"$instance_type", preference=~"$preference",guest_os_name=~"$guest_os_name" ,guest_os_version_id=~"$guest_os_version_id"}
)` + statusFilterJoin
