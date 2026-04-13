package virtualization

// Join suffix: filter VMs by time-in-status window and optional status template,
// and attach the printable status label for group_left merges (see ACM Grafana
// dash-acm-virtual-machines-by-time-in-status).
const vmByTimeInStatusStatusJoin = `
  + on(cluster, name, namespace) group_left(status)
  0*(
    (
      (time() - label_replace(kubevirt_vm_starting_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "starting", "", ""))
      > ($days_in_status_gt * 24 * 60 * 60)
    ) and (
      (time() - label_replace(kubevirt_vm_starting_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "starting", "", ""))
      < ($days_in_status_lt * 24 * 60 * 60)
    )
    or
    (
      (time() - label_replace(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "running", "", ""))
      > ($days_in_status_gt * 24 * 60 * 60)
    ) and (
      (time() - label_replace(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "running", "", ""))
      < ($days_in_status_lt * 24 * 60 * 60)
    )
    or
    (
      (time() - label_replace(kubevirt_vm_non_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "stopped", "", ""))
      > ($days_in_status_gt * 24 * 60 * 60)
    ) and (
      (time() - label_replace(kubevirt_vm_non_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "stopped", "", ""))
      < ($days_in_status_lt * 24 * 60 * 60)
    )
    or
    (
      (time() - label_replace(kubevirt_vm_error_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "error", "", ""))
      > ($days_in_status_gt * 24 * 60 * 60)
    ) and (
      (time() - label_replace(kubevirt_vm_error_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "error", "", ""))
      < ($days_in_status_lt * 24 * 60 * 60)
    )
    or
    (
      (time() - label_replace(kubevirt_vm_migrating_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "migrating", "", ""))
      > ($days_in_status_gt * 24 * 60 * 60)
    ) and (
      (time() - label_replace(kubevirt_vm_migrating_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "migrating", "", ""))
      < ($days_in_status_lt * 24 * 60 * 60)
    )
  )
  +${status:raw}`

const vmByTimeInStatusTotalAllocatedCPUStatQuery = `sum (
` + allocatedCPUMultiVMExpr + `
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTotalAllocatedMemoryStatQuery = `sum(
max by (cluster, namespace, name, status)(
  ` + allocatedMemoryMultiVMExpr + `
` + vmByTimeInStatusStatusJoin + `
))`

const vmByTimeInStatusTotalAllocatedDiskStatQuery = `sum (
  (kubevirt_vm_disk_allocated_size_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableDiskQuery = `sum by (cluster, namespace, name, status)(
  (kubevirt_vm_disk_allocated_size_bytes{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableTimeInStatusQuery = `  (
    (
      (time() - label_replace(kubevirt_vm_starting_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "starting", "", "")) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - label_replace(kubevirt_vm_starting_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "starting", "", "")) < $days_in_status_lt * 24 * 60 * 60
    )
  ) +${status:raw}
  or
  (
    (
      (time() - label_replace(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "running", "", "")) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - label_replace(kubevirt_vm_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "running", "", "")) < $days_in_status_lt * 24 * 60 * 60
    )
  ) +${status:raw}
  or
  (
    (
      (time() - label_replace(kubevirt_vm_non_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "stopped", "", "")) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - label_replace(kubevirt_vm_non_running_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "stopped", "", "")) < $days_in_status_lt * 24 * 60 * 60
    )
  ) +${status:raw}
  or
  (
    (
      (time() - label_replace(kubevirt_vm_error_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "error", "", "")) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - label_replace(kubevirt_vm_error_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "error", "", "")) < $days_in_status_lt * 24 * 60 * 60
    )
  ) +${status:raw}
  or
  (
    (
      (time() - label_replace(kubevirt_vm_migrating_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "migrating", "", "")) > $days_in_status_gt * 24 * 60 * 60
    ) and (
      (time() - label_replace(kubevirt_vm_migrating_status_last_transition_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > 0, "status", "migrating", "", "")) < $days_in_status_lt * 24 * 60 * 60
    )
  ) +${status:raw}`

const vmByTimeInStatusTableTimeSinceLastMigrationQuery = `sum by (cluster, namespace, name, status)(
  (time() - kubevirt_vmi_migration_end_time_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableMigrationEndMsQuery = `sum by (cluster, namespace, name, status)(
  (kubevirt_vmi_migration_end_time_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}*1000)
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableMemoryQuery = `max by (cluster, namespace, name, status)(
  ` + allocatedMemoryMultiVMExpr + `
` + vmByTimeInStatusStatusJoin + `
)`

const vmByTimeInStatusTableCPUQuery = allocatedCPUMultiVMExpr + `
` + vmByTimeInStatusStatusJoin
