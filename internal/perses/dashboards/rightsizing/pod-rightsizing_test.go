// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPodRightSizing(t *testing.T) {
	db, err := BuildPodRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)
	assert.Equal(t, "acm-rs-pod-overview", db.Dashboard.Metadata.Name)
	raw, err := json.Marshal(db.Dashboard.Spec)
	require.NoError(t, err)
	specStr := string(raw)
	assert.Contains(t, specStr, "acm_rs:pod:cpu_usage")
	assert.Contains(t, specStr, "acm_rs:pod:memory_usage")
	assert.Contains(t, specStr, "acm-rs-pod-detail")
	assert.NotContains(t, specStr, "acm_rs:workload:cpu_usage")
	assert.NotContains(t, specStr, `"text":"N/A"`, "missing pod fields must stay blank")
}

func TestPodOverviewTablesKeepInventoryRowsWithoutResourceSamples(t *testing.T) {
	db, err := BuildPodRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)
	require.Len(t, db.Dashboard.Spec.Panels, 2, "pod overview retains both resource tables")
	for name, p := range db.Dashboard.Spec.Panels {
		require.Equal(t, "Table", p.Spec.Plugin.Kind)
		t.Run(name, func(t *testing.T) {
			require.Len(t, p.Spec.Queries, 6)
			raw, err := json.Marshal(p.Spec.Queries[5].Spec.Plugin.Spec)
			require.NoError(t, err)
			assert.Contains(t, string(raw), "acm_rs:pod:info")
			assert.NotContains(t, string(raw), "$cpu_profile")
			assert.NotContains(t, string(raw), "$memory_profile")
			raw, err = json.Marshal(p.Spec.Plugin.Spec)
			require.NoError(t, err)
			assert.Contains(t, string(raw), `"name":"value #6","hide":true`)
			assert.NotContains(t, string(raw), `"text":"N/A"`)
		})
	}
}

func TestBuildPodDetail(t *testing.T) {
	db, err := BuildPodDetail(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)
	assert.Equal(t, "acm-rs-pod-detail", db.Dashboard.Metadata.Name)
	assert.Len(t, db.Dashboard.Spec.Layouts, 3, "back link, CPU group, Memory group")
	assert.Len(t, db.Dashboard.Spec.Panels, 13, "back link, ten stats, and two time-series charts")
	for name, p := range db.Dashboard.Spec.Panels {
		assert.NotEqual(t, "Table", p.Spec.Plugin.Kind, "pod detail panel %s must not be a table", name)
	}
	varNames := extractVarNames(db.Dashboard.Spec.Variables)
	assert.Contains(t, varNames, "cluster")
	assert.Contains(t, varNames, "namespace")
	assert.Contains(t, varNames, "pod")
	assert.NotContains(t, varNames, "workload", "pod detail should not expose unused workload variable")
	assert.NotContains(t, varNames, "workload_type", "pod detail should not expose unused workload_type variable")
	raw, err := json.Marshal(db.Dashboard.Spec)
	require.NoError(t, err)
	specStr := string(raw)
	assert.Contains(t, specStr, "acm_rs:pod:cpu_recommendation")
	assert.Contains(t, specStr, "acm_rs:pod:memory_recommendation")
	assert.Contains(t, specStr, "CPU Over Time - Pod")
	assert.Contains(t, specStr, "Memory Over Time - Pod")
	assert.NotContains(t, specStr, "acm_rs:workload:cpu_usage")
}
