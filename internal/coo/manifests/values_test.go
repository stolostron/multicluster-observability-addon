package manifests

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var virtualizationDashboardNames = []string{
	"acm-virtual-machines-inventory",
	"acm-virtual-machines-utilization",
	"acm-virtual-machines-service-level",
	"acm-virtual-machines-by-time-in-status",
	"acm-node-memory-overview",
	"acm-openshift-virtualization-overview",
	"acm-openshift-virtualization-single-cluster-view",
	"acm-openshift-virtualization-single-vm-view",
	"acm-virtual-machines-top-consumers",
}

func TestBuildVirtualizationDashboards(t *testing.T) {
	dashboards := buildVirtualizationDashboards()
	require.Len(t, dashboards, len(virtualizationDashboardNames))

	byName := map[string]DashboardValue{}
	for _, db := range dashboards {
		byName[db.Name] = db
	}
	for _, name := range virtualizationDashboardNames {
		require.Contains(t, byName, name)
		assert.NotEmpty(t, byName[name].Data)
	}
}

func TestBuildValuesIncludesVirtualizationWhenMetricsUIEnabled(t *testing.T) {
	values := BuildValues(metricsUIOptions(), false, true, false)

	names := dashboardNameSet(values.Dashboards)
	for _, name := range virtualizationDashboardNames {
		assert.Contains(t, names, name)
	}
	for _, db := range values.AnalyticsDashboards {
		assert.NotContains(t, virtualizationDashboardNames, db.Name)
	}
}

func TestBuildValuesOmitsVirtualizationWhenMetricsUIDisabled(t *testing.T) {
	values := BuildValues(addon.Options{}, false, true, false)
	names := dashboardNameSet(values.Dashboards)
	for _, name := range virtualizationDashboardNames {
		assert.NotContains(t, names, name)
	}
}

func TestBuildValuesOmitsVirtualizationOnSpoke(t *testing.T) {
	values := BuildValues(metricsUIOptions(), false, false, false)
	names := dashboardNameSet(values.Dashboards)
	for _, name := range virtualizationDashboardNames {
		assert.NotContains(t, names, name)
	}
}

func metricsUIOptions() addon.Options {
	return addon.Options{
		Platform: addon.PlatformOptions{
			Metrics: addon.MetricsOptions{
				CollectionEnabled: true,
				UI:                addon.MetricsUIOptions{Enabled: true},
			},
		},
	}
}

func dashboardNameSet(dashboards []DashboardValue) map[string]struct{} {
	names := make(map[string]struct{}, len(dashboards))
	for _, db := range dashboards {
		names[db.Name] = struct{}{}
	}
	return names
}
