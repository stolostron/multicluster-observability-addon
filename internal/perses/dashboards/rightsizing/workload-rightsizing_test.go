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

func TestBuildWorkloadRightSizing(t *testing.T) {
	db, err := BuildWorkloadRightSizing(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rs-workload-overview", db.Dashboard.Metadata.Name)
	assert.Equal(t, "ACM Right-Sizing Workloads", spec.Display.Name)

	t.Run("has expected variables", func(t *testing.T) {
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "cpu_profile")
		assert.Contains(t, varNames, "memory_profile")
		assert.Contains(t, varNames, "days")
		assert.Contains(t, varNames, "namespace")
	})

	t.Run("panels query workload metrics including limits", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs:workload:cpu_recommendation")
		assert.Contains(t, specStr, "acm_rs:workload:cpu_limit")
		assert.Contains(t, specStr, "acm_rs:workload:memory_limit")
		assert.NotContains(t, specStr, "acm_rs:pod:cpu_limit")
		assert.Contains(t, specStr, "$cpu_profile")
		assert.Contains(t, specStr, "$memory_profile")
		assert.NotContains(t, specStr, `profile="$profile"`)
		assert.Contains(t, specStr, "acm-rs-workload-detail")
		assert.Contains(t, specStr, "start=$__range", "drill-down links must pass the current time range")
		assert.Contains(t, specStr, "var-cluster=$cluster", "tables drop the cluster label; links must use the dashboard variable")
		assert.Contains(t, specStr, `topk(20, sum by (namespace, workload, workload_type)`)
		assert.NotContains(t, specStr, "topk(20, max_over_time")
		assert.Contains(t, specStr, "profile=")
	})
}
