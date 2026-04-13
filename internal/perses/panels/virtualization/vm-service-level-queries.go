package virtualization

// Step/scalar coupling: the subquery step [24h:5m] (5 minutes = 300 seconds)
// is paired with the multiplier *300 and divisor /3600 to convert samples to
// hours. These three values must stay in sync: step_seconds × scalar = 3600.
// If the step changes, update *300 in every hours query accordingly.

// serviceLevelStatusFilter is appended to every per-VM expression to apply
// the Status dropdown. $status comes from VMStatusVariableStaticSingleSelect
// (values: running / non_running / starting / migrating / error / .* for All).
const serviceLevelStatusFilter = ` and on(cluster, name, namespace)
  (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"$status"} > 0)`

// Shared service-level sub-expressions.
const serviceLevelTotalSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > bool 0) 
    [24h:5m]
    ))` + serviceLevelStatusFilter

const serviceLevelRunningSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"running"} > bool 0) 
    [24h:5m]
    ))` + serviceLevelStatusFilter

const serviceLevelPlannedDowntimeSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|migrating|non_running"} > bool 0) 
    [24h:5m]
    ))` + serviceLevelStatusFilter

const serviceLevelUnplannedDowntimeSamplesExpr = `sum by (cluster, name , namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"error"} > bool 0) 
    [24h:5m]
    ))` + serviceLevelStatusFilter

const serviceLevelUptimePercentQuery = `sum(` + serviceLevelRunningSamplesExpr + `)/ 
sum(` + serviceLevelTotalSamplesExpr + `) or vector(0)`

const serviceLevelPlannedDowntimePercentQuery = `sum(` + serviceLevelPlannedDowntimeSamplesExpr + `)/ 
sum(` + serviceLevelTotalSamplesExpr + `) or vector(0)`

const serviceLevelUnplannedDowntimePercentQuery = `sum(` + serviceLevelUnplannedDowntimeSamplesExpr + `)/ 
sum(` + serviceLevelTotalSamplesExpr + `) or vector(0)`

const serviceLevelUptimeHoursQuery = `sum(
(` + serviceLevelRunningSamplesExpr + ` *300 )/3600
) or vector(0)`

const serviceLevelPlannedDowntimeHoursQuery = `sum(
(` + serviceLevelPlannedDowntimeSamplesExpr + ` *300 )/3600
) or vector(0)`

const serviceLevelUnplannedDowntimeHoursQuery = `sum(
(` + serviceLevelUnplannedDowntimeSamplesExpr + ` *300 )/3600
) or vector(0)`

// Per-VM table sample sub-expressions (no status filter appended — composed below).
// These count 5-minute samples over 24 h for each status bucket per VM.
const (
	tableTotalSamples = `sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"} > bool 0)
    [24h:5m]
    )
  )`

	tableRunningSamples = `sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"running"} > bool 0)
    [24h:5m]
    )
  )`

	tablePlannedSamples = `sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"starting|migrating|non_running"} > bool 0)
    [24h:5m]
    )
  )`

	tableUnplannedSamples = `sum by (cluster, name, namespace)(
    sum_over_time(
    (kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"error"} > bool 0)
    [24h:5m]
    )
  )`
)

const serviceLevelTableVMInfoQuery = `label_replace(label_replace(label_replace(label_replace(label_replace(
  sum by (cluster, namespace, name, status_group)(kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"})` + serviceLevelStatusFilter + `,
  "status_group","Running","status_group","running"),
  "status_group","Stopped","status_group","non_running"),
  "status_group","Starting","status_group","starting"),
  "status_group","Migrating","status_group","migrating"),
  "status_group","Error","status_group","error")`

const serviceLevelTablePlannedDowntimeHoursQuery = `(` + tablePlannedSamples + ` * 300
) / 3600` + serviceLevelStatusFilter

const serviceLevelTablePlannedDowntimePercentQuery = `(
  (` + tablePlannedSamples + ` * 300)
  /
  (` + tableTotalSamples + ` * 300)
)` + serviceLevelStatusFilter

const serviceLevelTableUnplannedDowntimeHoursQuery = `(` + tableUnplannedSamples + ` * 300
) / 3600` + serviceLevelStatusFilter

const serviceLevelTableUnplannedDowntimePercentQuery = `(
  (` + tableUnplannedSamples + ` * 300)
  /
  (` + tableTotalSamples + ` * 300)
)` + serviceLevelStatusFilter

const serviceLevelTableCreateDateQuery = `sum by (cluster, namespace, name)((kubevirt_vm_create_date_timestamp_seconds{cluster=~"$cluster", name=~"$name", namespace=~"$namespace"}>0)*1000)` + serviceLevelStatusFilter

const serviceLevelTableUptimeHoursQuery = `(` + tableRunningSamples + ` * 300
) / 3600` + serviceLevelStatusFilter

const serviceLevelTableUptimePercentQuery = `(
  (` + tableRunningSamples + ` * 300)
  /
  (` + tableTotalSamples + ` * 300)
)` + serviceLevelStatusFilter

const serviceLevelTableErrorStatusQuery = `sum by (cluster, namespace, name, status)(kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group="error"})` + serviceLevelStatusFilter
