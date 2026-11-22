package virtualization

import (
	"encoding/json"
	"testing"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/variable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panelPromQL returns the PromQL expression of every query of the panel with
// the given display name, in panel order. Asserting on a specific query keeps
// unrelated dashboard content (for example the hidden Guest os query, which
// also matches guest_os_name) from satisfying a whole-dashboard check.
func panelPromQL(t *testing.T, spec v1.DashboardSpec, panelName string) []string {
	t.Helper()

	var tablePanel *v1.Panel
	for _, candidate := range spec.Panels {
		if candidate.Spec.Display != nil && candidate.Spec.Display.Name == panelName {
			tablePanel = candidate
			break
		}
	}
	require.NotNil(t, tablePanel, "panel %q not found", panelName)

	expressions := make([]string, 0, len(tablePanel.Spec.Queries))
	for _, panelQuery := range tablePanel.Spec.Queries {
		rawPluginSpec, err := json.Marshal(panelQuery.Spec.Plugin.Spec)
		require.NoError(t, err)
		var prometheusQuery struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.Unmarshal(rawPluginSpec, &prometheusQuery))
		expressions = append(expressions, prometheusQuery.Query)
	}
	return expressions
}

func TestBuildVMInventory(t *testing.T) {
	b, err := BuildVMInventory("test-project", "test-datasource")
	require.NoError(t, err)

	spec := b.Dashboard.Spec
	// Main inventory table only; the panic device is a column in it.
	require.Len(t, spec.Layouts, 1, "expected a single inventory group")

	raw, err := json.Marshal(spec)
	require.NoError(t, err)
	// The panic device column covers all VMs, not only Windows ones.
	assert.NotContains(t, string(raw), `os=~\"windows.*\"`)
	assert.NotContains(t, string(raw), `guest_os_name=~\"(?i).*windows.*\"`)
	// The main inventory table carries a Panic Device column (like Guest Agent).
	assert.Contains(t, string(raw), `"header":"Panic Device"`)
	// The column is fed by a query that carries has_panic_device for every
	// running VMI.
	assert.Contains(t, string(raw), `sum by (cluster, namespace, name, has_panic_device)(`)
	// The evictable/outdated query must not carry has_panic_device anymore.
	assert.Contains(t, string(raw), `sum by (cluster,namespace,name,outdated ,evictable) (`)
	// Query 4 feeds the "value #4" Guest Agent column: it detects
	// agent-reported guest OS info, which the QEMU guest agent provides on
	// both Linux and Windows.
	queries := panelPromQL(t, spec, "Virtual Machines Inventory")
	require.Len(t, queries, 14, "inventory table query count")
	assert.Contains(t, queries[3], `guest_os_name!=""`)
}

func TestBuildVMUtilization(t *testing.T) {
	b, err := BuildVMUtilization("test-project", "test-datasource")
	require.NoError(t, err)

	// Query 18 feeds the "value #18" Guest Agent column: it detects
	// agent-reported guest OS info, mirroring the inventory table.
	queries := panelPromQL(t, b.Dashboard.Spec, "Virtual Machines Utilization")
	require.Len(t, queries, 18, "utilization table query count")
	assert.Contains(t, queries[17], `guest_os_name!=""`)
}

func TestBuildVMServiceLevel(t *testing.T) {
	_, err := BuildVMServiceLevel("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildVMByTimeInStatus(t *testing.T) {
	_, err := BuildVMByTimeInStatus("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildNodeMemoryOverview(t *testing.T) {
	_, err := BuildNodeMemoryOverview("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildVirtOverview(t *testing.T) {
	_, err := BuildVirtOverview("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildSingleClusterView(t *testing.T) {
	_, err := BuildSingleClusterView("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildSingleVMView(t *testing.T) {
	b, err := BuildSingleVMView("test-project", "test-datasource")
	require.NoError(t, err)

	raw, err := json.Marshal(b.Dashboard.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "has_panic_device", "single VM view should surface the panic device label")
	assert.Contains(t, string(raw), "Panic Device")
	// The row covers all VMs, not only Windows ones.
	assert.Contains(t, string(raw), `has_panic_device!=\"\"`)
	assert.NotContains(t, string(raw), `os=~\"windows.*\"`)
	assert.NotContains(t, string(raw), `guest_os_name=~\"(?i).*windows.*\"`)
	// The Guest Agent row mirrors the inventory table's column: it reports
	// whether the QEMU guest agent has reported guest OS info, with
	// "Not Reporting" highlighted in red.
	assert.Contains(t, string(raw), "Guest Agent")
	assert.Contains(t, string(raw), "Not Reporting")
	assert.Contains(t, string(raw), `"textColor":"#F2495C"`)
}

func TestBuildTopConsumers(t *testing.T) {
	b, err := BuildTopConsumers("test-project", "test-datasource")
	require.NoError(t, err)

	spec := b.Dashboard.Spec

	// 7 collapsible groups (memory, CPU, storage traffic, storage IOPS,
	// network traffic, vCPU wait, memory swap).
	assert.Len(t, spec.Layouts, 7, "expected 7 layout groups")

	// Locate the topn list variable and verify its static values.
	var topnValues []string
	for _, v := range spec.Variables {
		if v.Kind != variable.KindList {
			continue
		}
		raw, marshalErr := json.Marshal(v.Spec)
		require.NoError(t, marshalErr)
		var parsed struct {
			Name   string `json:"name"`
			Plugin struct {
				Spec struct {
					Values []struct {
						Value string `json:"value"`
					} `json:"values"`
				} `json:"spec"`
			} `json:"plugin"`
		}
		require.NoError(t, json.Unmarshal(raw, &parsed))
		if parsed.Name == "topn" {
			for _, sv := range parsed.Plugin.Spec.Values {
				topnValues = append(topnValues, sv.Value)
			}
			break
		}
	}
	assert.Equal(t, []string{"5", "10", "20", "50"}, topnValues, "topn variable values")

	// 3 visible variables (cluster, namespace, topn) + 7 hidden topN
	// variables (one per resource category).
	assert.Len(t, spec.Variables, 10, "expected 10 variables (3 visible + 7 hidden)")

	// Verify all 7 hidden topN variables exist by name.
	hiddenVarNames := map[string]bool{}
	for _, v := range spec.Variables {
		raw, marshalErr := json.Marshal(v.Spec)
		require.NoError(t, marshalErr)
		var parsed struct {
			Name    string `json:"name"`
			Display struct {
				Hidden bool `json:"hidden"`
			} `json:"display"`
		}
		require.NoError(t, json.Unmarshal(raw, &parsed))
		if parsed.Display.Hidden {
			hiddenVarNames[parsed.Name] = true
		}
	}
	for _, name := range []string{
		"topn_memory", "topn_cpu", "topn_storage_traffic",
		"topn_storage_iops", "topn_network_traffic",
		"topn_vcpu_wait", "topn_memory_swap",
	} {
		assert.True(t, hiddenVarNames[name], "missing hidden variable %s", name)
	}
}
