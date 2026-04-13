package virtualization

// Shared service-level sub-expressions.
const serviceLevelTotalSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > bool 0) 
    [24h:5m]
    ))
    +${status:raw}`

const serviceLevelRunningSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"running"} > bool 0) 
    [24h:5m]
    ))
    +${status:raw}`

const serviceLevelPlannedDowntimeSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|migrating|non_running"} > bool 0) 
    [24h:5m]
    ))
    +${status:raw}`

const serviceLevelUnplannedDowntimeSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"error"} > bool 0) 
    [24h:5m]
    ))
    +${status:raw}`

const serviceLevelUptimePercentQuery = `sum(` + serviceLevelRunningSamplesExpr + `)/ 
sum(` + serviceLevelTotalSamplesExpr + `)`

const serviceLevelPlannedDowntimePercentQuery = `sum(` + serviceLevelPlannedDowntimeSamplesExpr + `)/ 
sum(` + serviceLevelTotalSamplesExpr + `)`

const serviceLevelUnplannedDowntimePercentQuery = `sum(` + serviceLevelUnplannedDowntimeSamplesExpr + `)/ 
sum(` + serviceLevelTotalSamplesExpr + `)`

const serviceLevelUptimeHoursQuery = `sum(
(` + serviceLevelRunningSamplesExpr + ` *300 )/3600
)`

const serviceLevelPlannedDowntimeHoursQuery = `sum(
(` + serviceLevelPlannedDowntimeSamplesExpr + ` *300 )/3600
)`

const serviceLevelUnplannedDowntimeHoursQuery = `sum(
(` + serviceLevelUnplannedDowntimeSamplesExpr + ` *300 )/3600
)`

const serviceLevelTableVMInfoQuery = `sum by (cluster, namespace, name, status_group)(kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})+${status:raw}`

const serviceLevelTablePlannedDowntimeHoursQuery = `(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|migrating|non_running"} > bool 0)
    [24h:5m]
    )
  ) * 300
) / 3600
+${status:raw}`

const serviceLevelTablePlannedDowntimePercentQuery = `(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|migrating|non_running"} > bool 0)
    [24h:5m]
    )
  ) * 300
)
/
(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > bool 0)
    [24h:5m]
    )
  ) * 300
)
+${status:raw}`

const serviceLevelTableUnplannedDowntimeHoursQuery = `(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"error"} > bool 0)
    [24h:5m]
    )
  ) * 300
) / 3600
+${status:raw}`

const serviceLevelTableUnplannedDowntimePercentQuery = `(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"error"} > bool 0)
    [24h:5m]
    )
  ) * 300
)
/
(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > bool 0)
    [24h:5m]
    )
  ) * 300
)
+${status:raw}`

const serviceLevelTableCreateDateQuery = `sum by (cluster, namespace, name)((kubevirt_vm_create_date_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}>0)*1000)+${status:raw}`

const serviceLevelTableUptimeHoursQuery = `(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"running"} > bool 0)
    [24h:5m]
    )
  ) * 300
) / 3600
+${status:raw}`

const serviceLevelTableUptimePercentQuery = `(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"running"} > bool 0)
    [24h:5m]
    )
  ) * 300
)
/
(
  sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > bool 0)
    [24h:5m]
    )
  ) * 300
)
+${status:raw}`

const serviceLevelTableErrorStatusQuery = `sum by (cluster, namespace, name, status)(kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="error"})+${status:raw}`
