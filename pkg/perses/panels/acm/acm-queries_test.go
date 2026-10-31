package acm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACMCommonPanelQueriesCPUHaveClusterFilter(t *testing.T) {
	keysToCheck := []string{
		"CPUUtilization",
		"CPUOverestimation",
	}

	for _, key := range keysToCheck {
		t.Run(key, func(t *testing.T) {
			expr, ok := ACMCommonPanelQueries[key]
			require.True(t, ok, "ACMCommonPanelQueries[%q] not found", key)

			promql := expr.Pretty(0)
			assert.True(t, strings.Contains(promql, `node_cpu_seconds_total:mode_idle:avg_rate5m{cluster="$cluster"}`),
				"ACMCommonPanelQueries[%q] idle CPU metric is missing cluster=\"$cluster\" label matcher; got: %s", key, promql)
		})
	}
}
