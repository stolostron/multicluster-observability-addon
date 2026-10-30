//go:build promql

package workload

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Run with make test-promql. Keeping the evaluator in a prebuilt tool avoids
// adding Prometheus's TSDB and cloud SDK test dependencies to the addon module.
func TestPodMappingWithPromtool(t *testing.T) {
	promtool := os.Getenv("PROMTOOL")
	if promtool == "" {
		promtool = "promtool"
	}
	tool, err := exec.LookPath(promtool)
	require.NoError(t, err, "promtool is required; run make test-promql to install the pinned version")

	fixture, err := os.ReadFile("testdata/pod_mapping.yaml")
	require.NoError(t, err)
	placeholder := []byte("POD_WORKLOAD_MAPPING_EXPRESSION")
	require.Equal(t, 1, bytes.Count(fixture, placeholder), "fixture must contain exactly one expression placeholder")

	// JSON string quoting is valid YAML and preserves the generated expression's
	// quotes and whitespace without maintaining a second copy of the query.
	expression, err := json.Marshal(podWorkloadRelabelExpr(`namespace!=""`))
	require.NoError(t, err)
	fixture = bytes.ReplaceAll(fixture, placeholder, expression)
	testFile := filepath.Join(t.TempDir(), "pod_mapping.yaml")
	require.NoError(t, os.WriteFile(testFile, fixture, 0o600))

	cmd := exec.CommandContext(t.Context(), tool, "test", "rules", testFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "promtool evaluation failed:\n%s", output)
}
