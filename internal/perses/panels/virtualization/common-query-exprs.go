package virtualization

// Shared PromQL sub-expressions used across multiple dashboards.

// vmInfoStatusExpr converts status_group to a human-readable status label.
// Each label_replace step capitalizes and renames to match the Status filter options.
// Used by: inventory, utilization dashboards.
const vmInfoStatusExpr = `sum by (cluster, namespace, name, status) (
  label_replace(label_replace(label_replace(label_replace(label_replace(
    kubevirt_vm_info{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", status_group=~"$status"} > 0,
    "status", "Running",   "status_group", "running"),
    "status", "Stopped",   "status_group", "non_running"),
    "status", "Starting",  "status_group", "starting"),
    "status", "Migrating", "status_group", "migrating"),
    "status", "Error",     "status_group", "error")
)`

// totalVMsExpr counts the total number of distinct VMs.
// Used by: overview, single-cluster dashboards.
const totalVMsExpr = `sum(count(kubevirt_vm_info{cluster=~"$cluster"}) by (name, namespace))`

// Allocated CPU/memory expressions with guest_effective fallback.
// Each expression prefers source="guest_effective" (available on newer
// KubeVirt versions) and falls back to the legacy cores*sockets*threads
// calculation with source=~"default|domain".

// Multi-VM expressions: filter by cluster/namespace/name regex variables,
// aggregate by (cluster, namespace, name).
// Used by: inventory, utilization, time-in-status dashboards.
const allocatedCPUMultiVMExpr = `(
sum by (cluster, namespace, name)(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="cpu", unit="cores", source="guest_effective"})
or
sum by (cluster, namespace, name)(
(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="cpu", unit="cores", source=~"default|domain"})
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="cpu", unit="sockets", source=~"default|domain"})
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="cpu", unit="threads", source=~"default|domain"})
) or
sum by (cluster, namespace, name)(
(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="cpu", unit="cores", source=~"default|domain"})
* ignoring (unit)(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="cpu", unit="sockets", source=~"default|domain"})
) or
sum by (cluster, namespace, name)(
kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="cpu", unit="cores", source=~"default|domain"})
)`

const allocatedMemoryMultiVMExpr = `(
sum by (cluster, namespace, name)(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="memory", source="guest_effective"})
or
max by (cluster, namespace, name)(kubevirt_vm_resource_requests{cluster=~"$cluster", name=~"$name", namespace=~"$namespace", resource="memory", source=~"default|domain"})
)`
