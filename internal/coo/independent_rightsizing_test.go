package coo

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stretchr/testify/require"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
)

func TestIndependentWorkloadPodDashboardRendering(t *testing.T) {
	for _, workload := range []string{"enabled", "disabled"} {
		for _, pod := range []string{"enabled", "disabled"} {
			t.Run(fmt.Sprintf("workload=%s/pod=%s", workload, pod), func(t *testing.T) {
				cv := []addonv1beta1.CustomizedVariable{
					{Name: addon.KeyRightSizingDelegated, Value: "true"},
					{Name: addon.KeyPlatformNamespaceRightSizing, Value: "disabled"},
					{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "disabled"},
					{Name: addon.KeyPlatformWorkloadRightSizing, Value: workload},
					{Name: addon.KeyPlatformPodRightSizing, Value: pod},
				}
				k8s := reconcileHubResources(t, cv)
				dashboards := listDashboards(t, k8s, "observability-analytics")
				names := dashboardNames(dashboards)
				if pod == "disabled" {
					for _, db := range dashboards {
						if db.Name == "acm-rs-workload-detail" {
							raw, err := json.Marshal(db.Spec)
							require.NoError(t, err)
							require.NotContains(t, string(raw), "acm_rs:pod:")
							require.NotContains(t, string(raw), "acm-rs-pod-detail")
						}
					}
				}
				for _, name := range []string{"acm-rs-workload-overview", "acm-rs-workload-detail"} {
					require.Equal(t, workload == "enabled", slices.Contains(names, name), name)
				}
				for _, name := range []string{"acm-rs-pod-overview", "acm-rs-pod-detail"} {
					require.Equal(t, pod == "enabled", slices.Contains(names, name), name)
				}
			})
		}
	}
}

func TestWorkloadPodDashboardsExplicitlyEnabled(t *testing.T) {
	cv := []addonv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformWorkloadRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformPodRightSizing, Value: "enabled"},
	}
	k8s := reconcileHubResources(t, cv)
	dashboards := listDashboards(t, k8s, "observability-analytics")
	names := dashboardNames(dashboards)
	for _, name := range []string{"acm-rs-workload-overview", "acm-rs-workload-detail", "acm-rs-pod-overview", "acm-rs-pod-detail"} {
		require.Contains(t, names, name)
	}
}
