package compute

import (
	"strings"
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
			assert.True(t, strings.Contains(promql, `cluster="$cluster"`),
				"ClusterQueries[%q] is missing cluster=\"$cluster\" label matcher; got: %s", key, promql)
		})
	}
}
