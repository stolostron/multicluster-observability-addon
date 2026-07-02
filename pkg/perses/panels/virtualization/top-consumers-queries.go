package virtualization

// Top Consumers queries adapted from the Grafana
// "KubeVirt / Infrastructure Resources / Top Consumers" dashboard.
//
// Each resource category has a hidden PrometheusPromQLVariable that
// evaluates topk($topn, <base_expr>) and exposes the resulting VM
// names as $topn_<resource>. Both the table and time-series panels
// for that resource filter by name=~"$topn_<resource>" so they always
// show the same set of VMs.

// clusterNSMatchers is the label matcher fragment shared by all
// top-consumer queries.
const clusterNSMatchers = `cluster=~"$cluster", namespace=~"$namespace"`

// --- Metric builder helpers ---

// topnNameFilter builds a name=~"$<varName>" matcher fragment to inject
// into a metric selector so only the VMs chosen by the hidden variable
// are returned.
func topnNameFilter(varName string) string {
	return `, name=~"$` + varName + `"`
}

// cpuMetric returns the kubevirt_vmi_cpu_usage_seconds_total selector
// with optional extra matchers appended inside the braces.
func cpuMetric(extra string) string {
	return `sum by (cluster,namespace,name)(rate(kubevirt_vmi_cpu_usage_seconds_total{` + clusterNSMatchers + extra + `}[10m]))`
}

func networkMetric(extra string) string {
	return `sum by (cluster,namespace,name)(rate(kubevirt_vmi_network_receive_bytes_total{` + clusterNSMatchers + extra + `}[10m])) + sum by (cluster,namespace,name)(rate(kubevirt_vmi_network_transmit_bytes_total{` + clusterNSMatchers + extra + `}[10m]))`
}

func storageTrafficMetric(extra string) string {
	return `sum by (cluster,namespace,name)(rate(kubevirt_vmi_storage_read_traffic_bytes_total{` + clusterNSMatchers + extra + `}[10m])) + sum by (cluster,namespace,name)(rate(kubevirt_vmi_storage_write_traffic_bytes_total{` + clusterNSMatchers + extra + `}[10m]))`
}

func storageIOPSMetric(extra string) string {
	return `sum by (cluster,namespace,name)(rate(kubevirt_vmi_storage_iops_read_total{` + clusterNSMatchers + extra + `}[10m])) + sum by (cluster,namespace,name)(rate(kubevirt_vmi_storage_iops_write_total{` + clusterNSMatchers + extra + `}[10m]))`
}

func vcpuWaitMetric(extra string) string {
	return `sum by (cluster,namespace,name)(rate(kubevirt_vmi_vcpu_wait_seconds_total{` + clusterNSMatchers + extra + `}[10m]))`
}

func memorySwapMetric(extra string) string {
	return `sum by (cluster,namespace,name)(avg_over_time(kubevirt_vmi_memory_swap_in_traffic_bytes{` + clusterNSMatchers + extra + `}[10m])) + sum by (cluster,namespace,name)(avg_over_time(kubevirt_vmi_memory_swap_out_traffic_bytes{` + clusterNSMatchers + extra + `}[10m]))`
}

// memoryUsageMetric uses memory_used_bytes (MCO allowlisted, no guest agent).
func memoryUsageMetric(extra string) string {
	return `avg by (cluster,namespace,name)(avg_over_time(kubevirt_vmi_memory_used_bytes{` + clusterNSMatchers + extra + `}[10m]))`
}

// Instant-vector metric helpers for hidden-variable ranking queries.
//
// In multi-replica Thanos receive setups, range functions (rate,
// avg_over_time) over broad selectors trigger "vector cannot contain
// metrics with the same labelset" because duplicate series from
// different store replicas are not merged before the range function
// evaluates.  Instant-vector queries are immune because Thanos dedup
// collapses identical labelsets at query time.
//
// These produce the same top-N VM ranking while remaining Thanos-safe.

func cpuMetricInstant(extra string) string {
	return `sum by (cluster,namespace,name)(kubevirt_vmi_cpu_usage_seconds_total{` + clusterNSMatchers + extra + `})`
}

func networkMetricInstant(extra string) string {
	return `sum by (cluster,namespace,name)(kubevirt_vmi_network_receive_bytes_total{` + clusterNSMatchers + extra + `}) + sum by (cluster,namespace,name)(kubevirt_vmi_network_transmit_bytes_total{` + clusterNSMatchers + extra + `})`
}

func storageTrafficMetricInstant(extra string) string {
	return `sum by (cluster,namespace,name)(kubevirt_vmi_storage_read_traffic_bytes_total{` + clusterNSMatchers + extra + `}) + sum by (cluster,namespace,name)(kubevirt_vmi_storage_write_traffic_bytes_total{` + clusterNSMatchers + extra + `})`
}

func storageIOPSMetricInstant(extra string) string {
	return `sum by (cluster,namespace,name)(kubevirt_vmi_storage_iops_read_total{` + clusterNSMatchers + extra + `}) + sum by (cluster,namespace,name)(kubevirt_vmi_storage_iops_write_total{` + clusterNSMatchers + extra + `})`
}

func vcpuWaitMetricInstant(extra string) string {
	return `sum by (cluster,namespace,name)(kubevirt_vmi_vcpu_wait_seconds_total{` + clusterNSMatchers + extra + `})`
}

func memorySwapMetricInstant(extra string) string {
	return `max by (cluster,namespace,name)(kubevirt_vmi_memory_swap_in_traffic_bytes{` + clusterNSMatchers + extra + `}) + max by (cluster,namespace,name)(kubevirt_vmi_memory_swap_out_traffic_bytes{` + clusterNSMatchers + extra + `})`
}

func memoryUsageMetricInstant(extra string) string {
	return `avg by (cluster,namespace,name)(kubevirt_vmi_memory_used_bytes{` + clusterNSMatchers + extra + `})`
}

// --- Hidden-variable expressions ---
// Each evaluates topk($topn, <base_expr>) without the name filter.
// The PrometheusPromQLVariable extracts the "name" label, producing
// a pipe-separated regex used by both table and time-series panels.

const TopNMemoryVarName = "topn_memory"

var TopNMemoryVarExpr = `topk($topn, ` + memoryUsageMetricNoFilter + `)`

const TopNCPUVarName = "topn_cpu"

var TopNCPUVarExpr = `topk($topn, ` + cpuMetricNoFilter + `)`

const TopNStorageTrafficVarName = "topn_storage_traffic"

var TopNStorageTrafficVarExpr = `topk($topn, ` + storageTrafficMetricNoFilter + `)`

const TopNStorageIOPSVarName = "topn_storage_iops"

var TopNStorageIOPSVarExpr = `topk($topn, ` + storageIOPSMetricNoFilter + `)`

const TopNNetworkTrafficVarName = "topn_network_traffic"

var TopNNetworkTrafficVarExpr = `topk($topn, ` + networkMetricNoFilter + `)`

const TopNVCPUWaitVarName = "topn_vcpu_wait"

var TopNVCPUWaitVarExpr = `topk($topn, ` + vcpuWaitMetricNoFilter + `)`

const TopNMemorySwapVarName = "topn_memory_swap"

var TopNMemorySwapVarExpr = `topk($topn, ` + memorySwapMetricNoFilter + `)`

// Pre-computed base expressions without the name filter (for variables).
// These use instant-vector helpers so the topk ranking query is safe
// in multi-replica Thanos environments (see comment above).
var (
	memoryUsageMetricNoFilter    = memoryUsageMetricInstant("")
	cpuMetricNoFilter            = cpuMetricInstant("")
	storageTrafficMetricNoFilter = storageTrafficMetricInstant("")
	storageIOPSMetricNoFilter    = storageIOPSMetricInstant("")
	networkMetricNoFilter        = networkMetricInstant("")
	vcpuWaitMetricNoFilter       = vcpuWaitMetricInstant("")
	memorySwapMetricNoFilter     = memorySwapMetricInstant("")
)

// --- Table queries (instant) ---
// Tables filter by the hidden variable's name list to show exactly N rows.

var topConsumersMemoryTableQuery = memoryUsageMetric(topnNameFilter(TopNMemoryVarName)) + ` > 0`

var topConsumersCPUTableQuery = cpuMetric(topnNameFilter(TopNCPUVarName)) + ` > 0`

var topConsumersStorageTrafficTableQuery = storageTrafficMetric(topnNameFilter(TopNStorageTrafficVarName)) + ` > 0`

var topConsumersStorageIOPSTableQuery = storageIOPSMetric(topnNameFilter(TopNStorageIOPSVarName)) + ` > 0`

var topConsumersNetworkTrafficTableQuery = networkMetric(topnNameFilter(TopNNetworkTrafficVarName)) + ` > 0`

var topConsumersVCPUWaitTableQuery = vcpuWaitMetric(topnNameFilter(TopNVCPUWaitVarName)) + ` > 0`

var topConsumersMemorySwapTableQuery = memorySwapMetric(topnNameFilter(TopNMemorySwapVarName)) + ` > 0`

// --- Time-series queries (range) ---
// Time-series filter by the same hidden variable so they plot the
// exact same VMs as the paired table.

var topConsumersMemoryTimeSeriesQuery = memoryUsageMetric(topnNameFilter(TopNMemoryVarName))

var topConsumersCPUTimeSeriesQuery = cpuMetric(topnNameFilter(TopNCPUVarName))

var topConsumersStorageTrafficTimeSeriesQuery = storageTrafficMetric(topnNameFilter(TopNStorageTrafficVarName))

var topConsumersStorageIOPSTimeSeriesQuery = storageIOPSMetric(topnNameFilter(TopNStorageIOPSVarName))

var topConsumersNetworkTrafficTimeSeriesQuery = networkMetric(topnNameFilter(TopNNetworkTrafficVarName))

var topConsumersVCPUWaitTimeSeriesQuery = vcpuWaitMetric(topnNameFilter(TopNVCPUWaitVarName))

var topConsumersMemorySwapTimeSeriesQuery = memorySwapMetric(topnNameFilter(TopNMemorySwapVarName))
