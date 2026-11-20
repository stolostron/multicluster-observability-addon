package virtualization

import (
	"encoding/json"
	"testing"

	"github.com/perses/perses/pkg/model/api/v1/variable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	// The Guest Agent column detects agent-reported guest OS info, which the
	// QEMU guest agent provides on both Linux and Windows.
	assert.Contains(t, string(raw), `guest_os_name!=\"\"`)
}

func TestBuildVMUtilization(t *testing.T) {
	b, err := BuildVMUtilization("test-project", "test-datasource")
	require.NoError(t, err)

	raw, err := json.Marshal(b.Dashboard.Spec)
	require.NoError(t, err)
	// The Guest Agent column detects agent-reported guest OS info, mirroring
	// the inventory table.
	assert.Contains(t, string(raw), `guest_os_name!=\"\"`)
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
