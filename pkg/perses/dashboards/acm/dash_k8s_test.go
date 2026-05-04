package acm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildK8sDashboards(t *testing.T) {
	// Call it once
	_, err := BuildK8sDashboards("test-project", "test-datasource", "test-cluster-label")
	require.NoError(t, err)

	// Call it again - this should not panic
	_, err = BuildK8sDashboards("test-project", "test-datasource", "test-cluster-label")
	require.NoError(t, err)
}

func TestBuildETCDDashboards(t *testing.T) {
	// Call it once
	_, err := BuildETCDDashboards("test-project", "test-datasource", "test-cluster-label")
	require.NoError(t, err)
}
