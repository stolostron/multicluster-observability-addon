package virtualization

// vmStatusLastTransitionExpr deduplicates kubevirt_vm_*_status_last_transition_timestamp_seconds
// with max by (cluster, name, namespace) before label_replace to avoid many-to-many
// join errors when multiple virt-controller pods emit the same series simultaneously.
const (
	vmStartingLastTransitionExpr  = `label_replace(max by (cluster, name, namespace)(kubevirt_vm_starting_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}) > 0, "status", "Starting", "", "")`
	vmRunningLastTransitionExpr   = `label_replace(max by (cluster, name, namespace)(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}) > 0, "status", "Running", "", "")`
	vmStoppedLastTransitionExpr   = `label_replace(max by (cluster, name, namespace)(kubevirt_vm_non_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}) > 0, "status", "Stopped", "", "")`
	vmErrorLastTransitionExpr     = `label_replace(max by (cluster, name, namespace)(kubevirt_vm_error_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}) > 0, "status", "Error", "", "")`
	vmMigratingLastTransitionExpr = `label_replace(max by (cluster, name, namespace)(kubevirt_vm_migrating_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}) > 0, "status", "Migrating", "", "")`
)

const vmByTimeInStatusStatusJoin = `
  + on(cluster, name, namespace) group_left(status)
  (
    (
      (
        (time() - ` + vmStartingLastTransitionExpr + `)
        > ($days_in_status_gt * 24 * 60 * 60)
      ) and (
        (time() - ` + vmStartingLastTransitionExpr + `)
        < ($days_in_status_lt * 24 * 60 * 60)
      ) and on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="starting"} > 0)
      unless on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"running|non_running|error|migrating"} > 0)
    )
    or
    (
      (
        (time() - ` + vmRunningLastTransitionExpr + `)
        > ($days_in_status_gt * 24 * 60 * 60)
      ) and (
        (time() - ` + vmRunningLastTransitionExpr + `)
        < ($days_in_status_lt * 24 * 60 * 60)
      ) and on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="running"} > 0)
      unless on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|non_running|error|migrating"} > 0)
    )
    or
    (
      (
        (time() - ` + vmStoppedLastTransitionExpr + `)
        > ($days_in_status_gt * 24 * 60 * 60)
      ) and (
        (time() - ` + vmStoppedLastTransitionExpr + `)
        < ($days_in_status_lt * 24 * 60 * 60)
      ) and on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="non_running"} > 0)
      unless on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|running|error|migrating"} > 0)
    )
    or
    (
      (
        (time() - ` + vmErrorLastTransitionExpr + `)
        > ($days_in_status_gt * 24 * 60 * 60)
      ) and (
        (time() - ` + vmErrorLastTransitionExpr + `)
        < ($days_in_status_lt * 24 * 60 * 60)
      ) and on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="error"} > 0)
      unless on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|running|non_running|migrating"} > 0)
    )
    or
    (
      (
        (time() - ` + vmMigratingLastTransitionExpr + `)
        > ($days_in_status_gt * 24 * 60 * 60)
      ) and (
        (time() - ` + vmMigratingLastTransitionExpr + `)
        < ($days_in_status_lt * 24 * 60 * 60)
      ) and on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="migrating"} > 0)
      unless on(cluster, name, namespace)
      (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|running|non_running|error"} > 0)
    )
  ) and on(cluster, name, namespace)
  (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"$status"} > 0)`

const vmByTimeInStatusTotalAllocatedCPUStatQuery = `sum (
(` + allocatedCPUMultiVMExpr + `)
` + vmByTimeInStatusStatusJoin + `
) or vector(0)`

const vmByTimeInStatusTotalAllocatedMemoryStatQuery = `sum(
max by (cluster, namespace, name, status)(
  (` + allocatedMemoryMultiVMExpr + `)
` + vmByTimeInStatusStatusJoin + `
)) or vector(0)`

const vmByTimeInStatusTotalAllocatedDiskStatQuery = `sum (
  (kubevirt_vm_disk_allocated_size_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})
` + vmByTimeInStatusStatusJoin + `
) or vector(0)`

const vmByTimeInStatusTableDiskQuery = `sum by (cluster, namespace, name, status)(
  (kubevirt_vm_disk_allocated_size_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableTimeInStatusQuery = `(
  (
    (
      (time() - ` + vmStartingLastTransitionExpr + `) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - ` + vmStartingLastTransitionExpr + `) < $days_in_status_lt * 24 * 60 * 60
    ) and on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="starting"} > 0)
    unless on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"running|non_running|error|migrating"} > 0)
  )
  or
  (
    (
      (time() - ` + vmRunningLastTransitionExpr + `) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - ` + vmRunningLastTransitionExpr + `) < $days_in_status_lt * 24 * 60 * 60
    ) and on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="running"} > 0)
    unless on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|non_running|error|migrating"} > 0)
  )
  or
  (
    (
      (time() - ` + vmStoppedLastTransitionExpr + `) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - ` + vmStoppedLastTransitionExpr + `) < $days_in_status_lt * 24 * 60 * 60
    ) and on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="non_running"} > 0)
    unless on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|running|error|migrating"} > 0)
  )
  or
  (
    (
      (time() - ` + vmErrorLastTransitionExpr + `) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - ` + vmErrorLastTransitionExpr + `) < $days_in_status_lt * 24 * 60 * 60
    ) and on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="error"} > 0)
    unless on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|running|non_running|migrating"} > 0)
  )
  or
  (
    (
      (time() - ` + vmMigratingLastTransitionExpr + `) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - ` + vmMigratingLastTransitionExpr + `) < $days_in_status_lt * 24 * 60 * 60
    ) and on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="migrating"} > 0)
    unless on(cluster, name, namespace)
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|running|non_running|error"} > 0)
  )
) and on(cluster, name, namespace)
(kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"$status"} > 0)`

const vmByTimeInStatusTableTimeSinceLastMigrationQuery = `sum by (cluster, namespace, name, status)(
  (time() - kubevirt_vmi_migration_end_time_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableMigrationEndMsQuery = `sum by (cluster, namespace, name, status)(
  (kubevirt_vmi_migration_end_time_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}*1000)
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableMemoryQuery = `max by (cluster, namespace, name, status)(
  (` + allocatedMemoryMultiVMExpr + `)
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableCPUQuery = `(` + allocatedCPUMultiVMExpr + `)
` + vmByTimeInStatusStatusJoin
