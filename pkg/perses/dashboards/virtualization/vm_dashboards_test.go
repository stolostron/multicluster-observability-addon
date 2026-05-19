package virtualization

import (
	"encoding/json"
	"testing"

	"github.com/perses/perses/pkg/model/api/v1/variable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVMInventory(t *testing.T) {
	_, err := BuildVMInventory("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildVMUtilization(t *testing.T) {
	_, err := BuildVMUtilization("test-project", "test-datasource")
	require.NoError(t, err)
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
	_, err := BuildSingleVMView("test-project", "test-datasource")
	require.NoError(t, err)
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
