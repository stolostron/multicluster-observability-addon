package exporter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWorkloadKey(t *testing.T) {
	k, err := parseWorkloadKey("local-cluster/hive//cpu")
	require.NoError(t, err)
	require.Equal(t, "local-cluster", k.Cluster)
	require.Equal(t, "hive", k.Namespace)
	require.Equal(t, "", k.Workload)
	require.Equal(t, "cpu", k.Resource)

	k2, err := parseWorkloadKey("vm-spoke/ns-1/vm-a/memory")
	require.NoError(t, err)
	require.Equal(t, "vm-a", k2.Workload)
	require.Equal(t, "memory", k2.Resource)
}
