package thanos

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildThanosDashboards(t *testing.T) {
	objs, err := BuildThanosDashboards("test-project", "test-datasource", "")
	require.NoError(t, err)
	require.Len(t, objs, 6)

	// Call it again - this should not panic (init() rewrite only runs once).
	objs, err = BuildThanosDashboards("test-project", "test-datasource", "")
	require.NoError(t, err)
	require.Len(t, objs, 6)
}
