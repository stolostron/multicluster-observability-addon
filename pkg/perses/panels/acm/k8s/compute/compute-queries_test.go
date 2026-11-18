package compute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterQueriesHaveClusterFilter(t *testing.T) {
	statPanelKeys := []string{
		"CPUUtilisation",
		"CPURequestsCommitment",
		"CPULimitsCommitment",
		"MemoryUtilisation",
		"MemoryRequestsCommitment",
		"MemoryLimitsCommitment",
	}

	for _, key := range statPanelKeys {
		t.Run(key, func(t *testing.T) {
			expr, ok := ClusterQueries[key]
			require.True(t, ok, "ClusterQueries[%q] not found", key)

			promql := expr.Pretty(0)
			assert.Contains(t, promql, `cluster="$cluster"`,
				"ClusterQueries[%q] is missing cluster=\"$cluster\" label matcher; got: %s", key, promql)
		})
	}
}
