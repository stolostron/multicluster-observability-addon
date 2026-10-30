//go:build promql

package workload

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/require"
)

// Run with make test-promql. Keeping the evaluator in a prebuilt tool avoids
// adding Prometheus's TSDB and cloud SDK test dependencies to the addon module.
func TestPodMappingWithPromtool(t *testing.T) {
	runPromtoolTest(t, "pod_mapping.yaml", podWorkloadRelabelExpr(`namespace!=""`))
}

func TestRollingWindowWithPromtool(t *testing.T) {
	config := rightsizing.GetDefaultWorkloadPrometheusRuleConfig()
	config.EvaluationInterval = "10m"
	for _, gen := range []struct {
		name string
		fn   func(rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error)
	}{
		{"workload", GenerateWorkloadPrometheusRule},
		{"pod", GeneratePodPrometheusRule},
	} {
		rule, err := gen.fn(rightsizing.RSConfigMapData{PrometheusRuleConfig: config})
		require.NoError(t, err)
		tested := 0
		for _, r := range rule.Spec.Groups[0].Rules {
			if strings.HasSuffix(r.Record, ":cpu_request:5m") || strings.HasSuffix(r.Record, ":memory_usage:5m") {
				t.Run(gen.name+"/"+r.Record, func(t *testing.T) {
					// The fixture peaks after the mapping's default lookback has expired,
					// but before the next evaluation records a fresh mapping sample.
					runPromtoolTest(t, "rolling_window.yaml", "sum("+r.Expr.String()+")")
				})
				tested++
			}
		}
		require.Equal(t, 2, tested, "must evaluate both resource and usage queries")
	}
}

func runPromtoolTest(t *testing.T, fixtureName, expr string) {
	t.Helper()
	promtool := os.Getenv("PROMTOOL")
	if promtool == "" {
		promtool = "promtool"
	}
	tool, err := exec.LookPath(promtool)
	require.NoError(t, err, "promtool is required; run make test-promql to install the pinned version")

	fixture, err := os.ReadFile(filepath.Join("testdata", fixtureName))
	require.NoError(t, err)
	placeholder := []byte("PROMQL_EXPRESSION")
	require.Equal(t, 1, bytes.Count(fixture, placeholder), "fixture must contain exactly one expression placeholder")

	// JSON string quoting is valid YAML and preserves the generated expression's
	// quotes and whitespace without maintaining a second copy of the query.
	expression, err := json.Marshal(expr)
	require.NoError(t, err)
	fixture = bytes.ReplaceAll(fixture, placeholder, expression)
	testFile := filepath.Join(t.TempDir(), fixtureName)
	require.NoError(t, os.WriteFile(testFile, fixture, 0o600))

	cmd := exec.CommandContext(t.Context(), tool, "test", "rules", testFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "promtool evaluation failed:\n%s", output)
}
