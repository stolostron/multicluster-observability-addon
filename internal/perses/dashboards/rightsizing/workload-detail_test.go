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

func TestBuildWorkloadDetail(t *testing.T) {
	db, err := BuildWorkloadDetail(testProject, testDatasource, testClusterLbl)
	require.NoError(t, err)

	spec := db.Dashboard.Spec
	assert.Equal(t, "acm-rs-workload-detail", db.Dashboard.Metadata.Name)

	t.Run("has expected variables", func(t *testing.T) {
		varNames := extractVarNames(spec.Variables)
		assert.Contains(t, varNames, "cluster")
		assert.Contains(t, varNames, "cpu_profile")
		assert.Contains(t, varNames, "memory_profile")
		assert.Contains(t, varNames, "namespace")
		assert.Contains(t, varNames, "workload")
		assert.Contains(t, varNames, "workload_type")
	})

	t.Run("has expected panel groups", func(t *testing.T) {
		require.GreaterOrEqual(t, len(spec.Layouts), 3, "back link + CPU group + Memory group")
	})

	t.Run("includes request limit usage recommendation", func(t *testing.T) {
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		specStr := string(raw)
		assert.Contains(t, specStr, "acm_rs:workload:cpu_limit")
		assert.Contains(t, specStr, "acm_rs:workload:memory_limit")
		assert.Contains(t, specStr, "acm_rs:pod:cpu_limit")
		assert.Contains(t, specStr, "acm_rs:pod:memory_request")
		assert.Contains(t, specStr, "$cpu_profile")
		assert.Contains(t, specStr, "$memory_profile")
		assert.Contains(t, specStr, "acm-rs-workload-overview")
		assert.Contains(t, specStr, "start=$__range", "back link must pass the current time range")
		assert.Contains(t, specStr, "namespace=~")
		assert.Contains(t, specStr, "workload=~")
	})
}

func TestBuildWorkloadDetailForFeaturesOmitsPodTables(t *testing.T) {
	db, err := BuildWorkloadDetailForFeatures(testProject, testDatasource, testClusterLbl, false)
	require.NoError(t, err)
	raw, err := json.Marshal(db.Dashboard.Spec)
	require.NoError(t, err)
	specStr := string(raw)
	assert.NotContains(t, specStr, "acm_rs:pod:cpu_usage")
	assert.NotContains(t, specStr, "acm_rs:pod:memory_usage")
	assert.Contains(t, specStr, "acm_rs:workload:cpu_usage")
}
