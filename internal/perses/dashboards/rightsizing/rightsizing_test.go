// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"encoding/json"
	"testing"

	"github.com/perses/perses/go-sdk/dashboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProject    = "observability-analytics"
	testDatasource = "rbac-query-proxy-datasource"
	testClusterLbl = ""
)

// --- Namespace Right-Sizing Dashboard ---

func TestBuildNamespaceRightSizing(t *testing.T) {
	db, err := BuildNamespaceRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rs-namespace-overview", db.Dashboard.Metadata.Name)
	assert.Equal(t, "ACM Right-Sizing Namespace", spec.Display.Name)

	t.Run("has expected variables", func(t *testing.T) {
		require.Len(t, spec.Variables, 3, "expected cluster, profile, days")
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "profile")
		assert.Contains(t, varNames, "days")
	})

	t.Run("has expected panel groups", func(t *testing.T) {
		require.Len(t, spec.Layouts, 2, "CPU section + Memory section")
	})

	t.Run("panels reference the datasource", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.Contains(t, string(raw), testDatasource)
	})

	t.Run("panels query acm_rs namespace metrics", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs:cluster:cpu_recommendation")
		assert.Contains(t, specStr, "acm_rs:cluster:cpu_usage")
		assert.Contains(t, specStr, "acm_rs:namespace:cpu_usage")
		assert.Contains(t, specStr, "acm_rs:namespace:memory_usage")
	})

	t.Run("spec serializes to valid JSON", func(t *testing.T) {
		data, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var roundTrip map[string]any
		require.NoError(t, json.Unmarshal(data, &roundTrip))
	})
}

func TestBuildNamespaceRightSizing_Idempotent(t *testing.T) {
	db1, err := BuildNamespaceRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	db2, err := BuildNamespaceRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	data1, _ := json.Marshal(db1.Dashboard.Spec)
	data2, _ := json.Marshal(db2.Dashboard.Spec)
	assert.Equal(t, string(data1), string(data2), "repeated builds should produce identical specs")
}

// --- VM Overview Dashboard ---

func TestBuildVMOverview(t *testing.T) {
	db, err := BuildVMOverview(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rightsizing-openshift-virtualization", db.Dashboard.Metadata.Name)
	assert.Equal(t, "ACM Right-Sizing OpenShift Virtualization", spec.Display.Name)

	t.Run("has expected variables", func(t *testing.T) {
		require.Len(t, spec.Variables, 4, "expected cluster, profile, days, namespace")
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "profile")
		assert.Contains(t, varNames, "days")
		assert.Contains(t, varNames, "namespace")
	})

	t.Run("has expected panel groups", func(t *testing.T) {
		require.Len(t, spec.Layouts, 5, "stats + 4 table groups")
	})

	t.Run("panels query acm_rs_vm metrics", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs_vm:namespace:cpu_request")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:cpu_usage")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:memory_request")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:memory_usage")
	})

	t.Run("table panels contain drill-down links", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm-rightsizing-vm-overestimation")
		assert.Contains(t, specStr, "acm-rightsizing-vm-underestimation")
	})
}

func TestBuildVMOverview_Idempotent(t *testing.T) {
	db1, err := BuildVMOverview(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	db2, err := BuildVMOverview(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	data1, _ := json.Marshal(db1.Dashboard.Spec)
	data2, _ := json.Marshal(db2.Dashboard.Spec)
	assert.Equal(t, string(data1), string(data2))
}

// --- VM Overestimation Dashboard ---

func TestBuildVMOverestimation(t *testing.T) {
	db, err := BuildVMOverestimation(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rightsizing-vm-overestimation", db.Dashboard.Metadata.Name)
	assert.Equal(t, "ACM Right-Sizing OpenShift Virtualization VM Overestimation", spec.Display.Name)

	t.Run("has expected variables", func(t *testing.T) {
		require.Len(t, spec.Variables, 5, "expected cluster, profile, days, namespace, vm")
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "profile")
		assert.Contains(t, varNames, "days")
		assert.Contains(t, varNames, "namespace")
		assert.Contains(t, varNames, "vm")
	})

	t.Run("has expected panel groups", func(t *testing.T) {
		require.Len(t, spec.Layouts, 3, "back-to-main + CPU stats+chart + Mem stats+chart")
	})

	t.Run("contains back to main dashboard link", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "acm-rightsizing-openshift-virtualization")
	})

	t.Run("overestimation queries use acm_rs_vm metrics", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs_vm:namespace:cpu_request")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:cpu_recommendation")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:memory_request")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:memory_recommendation")
	})
}

func TestBuildVMOverestimation_Idempotent(t *testing.T) {
	db1, err := BuildVMOverestimation(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	db2, err := BuildVMOverestimation(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	data1, _ := json.Marshal(db1.Dashboard.Spec)
	data2, _ := json.Marshal(db2.Dashboard.Spec)
	assert.Equal(t, string(data1), string(data2))
}

// --- VM Underestimation Dashboard ---

func TestBuildVMUnderestimation(t *testing.T) {
	db, err := BuildVMUnderestimation(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rightsizing-vm-underestimation", db.Dashboard.Metadata.Name)
	assert.Equal(t, "ACM Right-Sizing OpenShift Virtualization VM Underestimation", spec.Display.Name)

	t.Run("has expected variables", func(t *testing.T) {
		require.Len(t, spec.Variables, 5, "expected cluster, profile, days, namespace, vm")
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "profile")
		assert.Contains(t, varNames, "days")
		assert.Contains(t, varNames, "namespace")
		assert.Contains(t, varNames, "vm")
	})

	t.Run("has expected panel groups", func(t *testing.T) {
		require.Len(t, spec.Layouts, 3, "back-to-main + CPU stats+chart + Mem stats+chart")
	})

	t.Run("contains back to main dashboard link", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "acm-rightsizing-openshift-virtualization")
	})

	t.Run("underestimation queries use acm_rs_vm metrics", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs_vm:namespace:cpu_request")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:cpu_recommendation")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:memory_request")
		assert.Contains(t, specStr, "acm_rs_vm:namespace:memory_recommendation")
	})
}

func TestBuildVMUnderestimation_Idempotent(t *testing.T) {
	db1, err := BuildVMUnderestimation(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	db2, err := BuildVMUnderestimation(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	data1, _ := json.Marshal(db1.Dashboard.Spec)
	data2, _ := json.Marshal(db2.Dashboard.Spec)
	assert.Equal(t, string(data1), string(data2))
}

// --- Workload-Pod Right-Sizing Dashboard ---

func TestBuildWorkloadPodRightSizing(t *testing.T) {
	db, err := BuildWorkloadPodRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rs-workload-pod-overview", db.Dashboard.Metadata.Name)
	assert.Equal(t, "ACM Right-Sizing Workloads & Pods", spec.Display.Name)

	t.Run("has expected variables", func(t *testing.T) {
		require.Len(t, spec.Variables, 3, "expected cluster, profile, days")
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "profile")
		assert.Contains(t, varNames, "days")
	})

	t.Run("has expected panel groups", func(t *testing.T) {
		require.Len(t, spec.Layouts, 6, "CPU stats + CPU topK + CPU table + Mem stats + Mem topK + Mem table")
	})

	t.Run("panels reference the datasource", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.Contains(t, string(raw), testDatasource)
	})

	t.Run("panels query acm_rs workload metrics", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs:workload:cpu_recommendation")
		assert.Contains(t, specStr, "acm_rs:workload:cpu_usage")
		assert.Contains(t, specStr, "acm_rs:workload:memory_usage")
		assert.Contains(t, specStr, "acm_rs:workload:cpu_limit")
	})

	t.Run("spec serializes to valid JSON", func(t *testing.T) {
		data, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var roundTrip map[string]any
		require.NoError(t, json.Unmarshal(data, &roundTrip))
	})
}

func TestBuildWorkloadPodRightSizing_Idempotent(t *testing.T) {
	db1, err := BuildWorkloadPodRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	db2, err := BuildWorkloadPodRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	data1, _ := json.Marshal(db1.Dashboard.Spec)
	data2, _ := json.Marshal(db2.Dashboard.Spec)
	assert.Equal(t, string(data1), string(data2), "repeated builds should produce identical specs")
}

// --- GPU Right-Sizing Dashboard ---

func TestBuildGPUUtilization(t *testing.T) {
	db, err := BuildGPUUtilization(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rs-gpu-utilization", db.Dashboard.Metadata.Name)
	assert.Equal(t, "ACM GPU Right-Sizing", spec.Display.Name)

	t.Run("has expected variables", func(t *testing.T) {
		require.Len(t, spec.Variables, 3, "expected cluster, profile, days")
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "profile")
		assert.Contains(t, varNames, "days")
	})

	t.Run("has expected panel groups", func(t *testing.T) {
		require.Len(t, spec.Layouts, 7, "cluster stats + ns stats + ns details + trends + topK + ns table + wl table")
	})

	t.Run("panels reference the datasource", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.Contains(t, string(raw), testDatasource)
	})

	t.Run("panels query acm_rs GPU metrics", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs:namespace:gpu_request")
		assert.Contains(t, specStr, "acm_rs:namespace:gpu_usage")
		assert.Contains(t, specStr, "acm_rs:cluster:gpu_request")
		assert.Contains(t, specStr, "acm_rs:workload:gpu_usage")
	})

	t.Run("spec serializes to valid JSON", func(t *testing.T) {
		data, err := json.Marshal(spec)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var roundTrip map[string]any
		require.NoError(t, json.Unmarshal(data, &roundTrip))
	})
}

func TestBuildGPUUtilization_Idempotent(t *testing.T) {
	db1, err := BuildGPUUtilization(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	db2, err := BuildGPUUtilization(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	data1, _ := json.Marshal(db1.Dashboard.Spec)
	data2, _ := json.Marshal(db2.Dashboard.Spec)
	assert.Equal(t, string(data1), string(data2), "repeated builds should produce identical specs")
}

// --- Cross-Dashboard Consistency ---

func TestAllDashboards_ProjectAndDatasourceThreading(t *testing.T) {
	customProject := "custom-project"
	customDS := "custom-datasource"

	builders := []struct {
		name string
		fn   func(string, string, string) (dashboard.Builder, error)
	}{
		{"NamespaceRightSizing", BuildNamespaceRightSizing},
		{"WorkloadPodRightSizing", BuildWorkloadPodRightSizing},
		{"GPUUtilization", BuildGPUUtilization},
		{"VMOverview", BuildVMOverview},
		{"VMOverestimation", BuildVMOverestimation},
		{"VMUnderestimation", BuildVMUnderestimation},
	}

	for _, b := range builders {
		t.Run(b.name, func(t *testing.T) {
			db, err := b.fn(customProject, customDS, "")
			require.NoError(t, err)
			assert.Equal(t, customProject, db.Dashboard.Metadata.Project)

			raw, err := json.Marshal(db.Dashboard.Spec)
			require.NoError(t, err)
			assert.Contains(t, string(raw), customDS, "all queries should reference the provided datasource")
		})
	}
}

func TestVMDashboards_DrillDownLinksUseCorrectProject(t *testing.T) {
	customProject := "my-analytics-ns"

	for _, tc := range []struct {
		name   string
		fn     func(string, string, string) (dashboard.Builder, error)
		linkID string
	}{
		{"VMOverview drill-down to overestimation", BuildVMOverview, "acm-rightsizing-vm-overestimation"},
		{"VMOverview drill-down to underestimation", BuildVMOverview, "acm-rightsizing-vm-underestimation"},
		{"VMOverestimation back link", BuildVMOverestimation, "acm-rightsizing-openshift-virtualization"},
		{"VMUnderestimation back link", BuildVMUnderestimation, "acm-rightsizing-openshift-virtualization"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := tc.fn(customProject, testDatasource, "")
			require.NoError(t, err)

			raw, err := json.Marshal(db.Dashboard.Spec)
			require.NoError(t, err)
			specStr := string(raw)
			assert.Contains(t, specStr, tc.linkID)
			assert.Contains(t, specStr, "project="+customProject)
		})
	}
}

// --- helpers ---

func extractVarNames(vars any) []string {
	data, _ := json.Marshal(vars)
	var rawVars []map[string]any
	_ = json.Unmarshal(data, &rawVars)

	var names []string
	for _, v := range rawVars {
		if spec, ok := v["spec"].(map[string]any); ok {
			if name, ok := spec["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}
