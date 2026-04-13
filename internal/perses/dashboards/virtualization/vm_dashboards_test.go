package virtualization

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildVMInventory(t *testing.T) {
	_, err := BuildVMInventory("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildVMUtilization(t *testing.T) {
	_, err := BuildVMUtilization("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildVMServiceLevel(t *testing.T) {
	_, err := BuildVMServiceLevel("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildVMByTimeInStatus(t *testing.T) {
	_, err := BuildVMByTimeInStatus("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildNodeMemoryOverview(t *testing.T) {
	_, err := BuildNodeMemoryOverview("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildVirtOverview(t *testing.T) {
	_, err := BuildVirtOverview("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildSingleClusterView(t *testing.T) {
	_, err := BuildSingleClusterView("test-project", "test-datasource")
	require.NoError(t, err)
}

func TestBuildSingleVMView(t *testing.T) {
	_, err := BuildSingleVMView("test-project", "test-datasource")
	require.NoError(t, err)
}
