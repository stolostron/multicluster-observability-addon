package acm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildACMMetricsCardinalityOverview(t *testing.T) {
	_, err := BuildACMMetricsCardinalityOverview("test-project", "test-datasource", "")
	require.NoError(t, err)

	_, err = BuildACMMetricsCardinalityOverview("test-project", "test-datasource", "")
	require.NoError(t, err)
}

func TestBuildACMMetricsCardinalityCluster(t *testing.T) {
	_, err := BuildACMMetricsCardinalityCluster("test-project", "test-datasource", "")
	require.NoError(t, err)

	_, err = BuildACMMetricsCardinalityCluster("test-project", "test-datasource", "")
	require.NoError(t, err)
}

func TestBuildACMMetricsCardinalityName(t *testing.T) {
	_, err := BuildACMMetricsCardinalityName("test-project", "test-datasource", "")
	require.NoError(t, err)

	_, err = BuildACMMetricsCardinalityName("test-project", "test-datasource", "")
	require.NoError(t, err)
}
