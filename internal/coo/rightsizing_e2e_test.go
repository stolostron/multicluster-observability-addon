package coo

import (
	"encoding/json"
	"slices"
	"testing"

	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// expectedRSDashboards are the dashboard IDs produced by the right-sizing builders.
var (
	namespaceDashboardID = "acm-rs-namespace-overview"
	vmDashboardIDs       = []string{
		"acm-rightsizing-openshift-virtualization",
		"acm-rightsizing-vm-overestimation",
		"acm-rightsizing-vm-underestimation",
	}
	allRSDashboardIDs = append([]string{namespaceDashboardID}, vmDashboardIDs...)
)

func renderRSManifests(t *testing.T, isHub bool, cv []addonapiv1beta1.CustomizedVariable) []runtime.Object {
	t.Helper()

	mc := addontesting.NewManagedCluster("cluster-1")
	if isHub {
		mc.Labels = map[string]string{"local-cluster": "true"}
	}

	mcao := addontesting.NewAddon("test", "cluster-1")
	mcao.Status.ConfigReferences = []addonapiv1beta1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1beta1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			DesiredConfig: &addonapiv1beta1.ConfigSpecHash{
				ConfigReferent: addonapiv1beta1.ConfigReferent{
					Namespace: addoncfg.InstallNamespace,
					Name:      addoncfg.Name,
				},
				SpecHash: "fake-spec-hash",
			},
		},
	}

	addc := &addonapiv1beta1.AddOnDeploymentConfig{
		ObjectMeta: mcao.ObjectMeta,
		Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: cv,
		},
	}
	addc.Name = addoncfg.Name
	addc.Namespace = addoncfg.InstallNamespace

	cooAgent := newCOOAgentAddon([]client.Object{mcao}, addc)
	objects, err := cooAgent.Manifests(t.Context(), mc, mcao)
	require.NoError(t, err)
	return objects
}

// classifyObjects separates rendered manifests into typed buckets for assertions.
type renderedObjects struct {
	namespaces  []*corev1.Namespace
	datasources []*persesv1.PersesDatasource
	dashboards  []*persesv1.PersesDashboard
	all         []runtime.Object
}

func classify(objects []runtime.Object) renderedObjects {
	var r renderedObjects
	r.all = objects
	for _, o := range objects {
		switch obj := o.(type) {
		case *corev1.Namespace:
			r.namespaces = append(r.namespaces, obj)
		case *persesv1.PersesDatasource:
			r.datasources = append(r.datasources, obj)
		case *persesv1.PersesDashboard:
			r.dashboards = append(r.dashboards, obj)
		}
	}
	return r
}

// --- End-to-End Tests ---

func TestRightSizing_HubCluster_BothEnabled(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	t.Run("creates analytics namespace", func(t *testing.T) {
		var found bool
		for _, ns := range r.namespaces {
			if ns.Name == addoncfg.AnalyticsNamespace {
				found = true
				break
			}
		}
		require.True(t, found, "observability-analytics namespace must be created")
	})

	t.Run("creates analytics datasource", func(t *testing.T) {
		var found bool
		for _, ds := range r.datasources {
			if ds.Namespace == addoncfg.AnalyticsNamespace && ds.Name == "rbac-query-proxy-datasource" {
				found = true
				break
			}
		}
		require.True(t, found, "rbac-query-proxy-datasource must exist in analytics namespace")
	})

	t.Run("creates all 4 right-sizing dashboards", func(t *testing.T) {
		dashNames := dashboardNames(r.dashboards)
		for _, expected := range allRSDashboardIDs {
			assert.Contains(t, dashNames, expected, "dashboard %q should be rendered", expected)
		}
	})

	t.Run("all RS dashboards are in the analytics namespace", func(t *testing.T) {
		for _, db := range r.dashboards {
			if contains(allRSDashboardIDs, db.Name) {
				assert.Equal(t, addoncfg.AnalyticsNamespace, db.Namespace,
					"dashboard %q must be in %s", db.Name, addoncfg.AnalyticsNamespace)
			}
		}
	})

	t.Run("no RS dashboards in the install namespace", func(t *testing.T) {
		for _, db := range r.dashboards {
			if db.Namespace == addoncfg.InstallNamespace {
				assert.NotContains(t, allRSDashboardIDs, db.Name,
					"RS dashboard %q should NOT be in install namespace", db.Name)
			}
		}
	})

	t.Run("dashboard specs contain valid JSON with expected metrics", func(t *testing.T) {
		for _, db := range r.dashboards {
			if !contains(allRSDashboardIDs, db.Name) {
				continue
			}
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err, "dashboard %q spec must serialize", db.Name)
			assert.Greater(t, len(raw), 100, "dashboard %q spec should be non-trivial", db.Name)

			specStr := string(raw)
			if db.Name == namespaceDashboardID {
				assert.Contains(t, specStr, "acm_rs:cluster:cpu_recommendation")
				assert.Contains(t, specStr, "acm_rs:namespace:cpu_usage")
			} else {
				assert.Contains(t, specStr, "acm_rs_vm:namespace:cpu_request")
				assert.Contains(t, specStr, "acm_rs_vm:namespace:memory_request")
			}
		}
	})
}

func TestRightSizing_HubCluster_NamespaceOnly(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "disabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	t.Run("creates namespace RS dashboard only", func(t *testing.T) {
		dashNames := dashboardNames(r.dashboards)
		assert.Contains(t, dashNames, namespaceDashboardID)
		for _, vmID := range vmDashboardIDs {
			assert.NotContains(t, dashNames, vmID, "VM dashboard %q should not be rendered", vmID)
		}
	})

	t.Run("analytics namespace and datasource still created", func(t *testing.T) {
		require.GreaterOrEqual(t, len(r.namespaces), 1)
		require.GreaterOrEqual(t, len(r.datasources), 1)
	})
}

func TestRightSizing_HubCluster_VirtualizationOnly(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "disabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	t.Run("creates 3 VM RS dashboards, no namespace dashboard", func(t *testing.T) {
		dashNames := dashboardNames(r.dashboards)
		assert.NotContains(t, dashNames, namespaceDashboardID)
		for _, vmID := range vmDashboardIDs {
			assert.Contains(t, dashNames, vmID, "VM dashboard %q should be rendered", vmID)
		}
	})
}

func TestRightSizing_HubCluster_BothDisabled(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "disabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "disabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	t.Run("no RS dashboards rendered", func(t *testing.T) {
		for _, db := range r.dashboards {
			assert.NotContains(t, allRSDashboardIDs, db.Name,
				"no RS dashboards should exist when both are disabled")
		}
	})

	t.Run("no analytics namespace", func(t *testing.T) {
		for _, ns := range r.namespaces {
			assert.NotEqual(t, addoncfg.AnalyticsNamespace, ns.Name)
		}
	})
}

func TestRightSizing_NonHubCluster_NoRSDashboards(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, false, cv)
	r := classify(objects)

	t.Run("no RS dashboards on spoke cluster", func(t *testing.T) {
		for _, db := range r.dashboards {
			assert.NotContains(t, allRSDashboardIDs, db.Name,
				"RS dashboards should only be on the hub cluster")
		}
	})
}

func TestRightSizing_DashboardSpecStructure(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	for _, db := range r.dashboards {
		if !contains(allRSDashboardIDs, db.Name) {
			continue
		}

		t.Run(db.Name+"/spec_has_layouts", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)

			var spec map[string]any
			require.NoError(t, json.Unmarshal(raw, &spec))

			layouts, ok := spec["layouts"]
			require.True(t, ok, "dashboard spec must have layouts")
			layoutSlice, ok := layouts.([]any)
			require.True(t, ok)
			assert.NotEmpty(t, layoutSlice, "dashboard must have at least one layout")
		})

		t.Run(db.Name+"/spec_has_panels", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)

			var spec map[string]any
			require.NoError(t, json.Unmarshal(raw, &spec))

			panels, ok := spec["panels"]
			require.True(t, ok, "dashboard spec must have panels")
			panelMap, ok := panels.(map[string]any)
			require.True(t, ok)
			assert.NotEmpty(t, panelMap, "dashboard must have at least one panel")
		})

		t.Run(db.Name+"/spec_has_variables", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)

			var spec map[string]any
			require.NoError(t, json.Unmarshal(raw, &spec))

			variables, ok := spec["variables"]
			require.True(t, ok, "dashboard spec must have variables")
			varSlice, ok := variables.([]any)
			require.True(t, ok)
			assert.GreaterOrEqual(t, len(varSlice), 4, "all RS dashboards have at least cluster, cpu_profile, memory_profile, days")

			varNames := extractDashboardVarNames(varSlice)
			assert.Contains(t, varNames, "cluster")
			assert.Contains(t, varNames, "cpu_profile")
			assert.Contains(t, varNames, "memory_profile")
			assert.Contains(t, varNames, "days")
			assert.NotContains(t, varNames, "profile", "shared profile variable should be split into cpu/memory profiles")
		})

		t.Run(db.Name+"/spec_uses_split_profile_vars", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)
			specStr := string(raw)
			assert.Contains(t, specStr, "$cpu_profile")
			assert.Contains(t, specStr, "$memory_profile")
			assert.NotContains(t, specStr, `profile="$profile"`)
		})

		t.Run(db.Name+"/spec_references_datasource", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)
			assert.Contains(t, string(raw), "rbac-query-proxy-datasource")
		})
	}
}

func TestRightSizing_CombinedWithIncidentDetection(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformIncidentDetection, Value: "uiplugins.v1alpha1.observability.openshift.io"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	t.Run("RS dashboards coexist with incident detection", func(t *testing.T) {
		dashNames := dashboardNames(r.dashboards)
		for _, expected := range allRSDashboardIDs {
			assert.Contains(t, dashNames, expected, "RS dashboard %q present alongside incident detection", expected)
		}
	})

	t.Run("RS dashboards are in analytics namespace", func(t *testing.T) {
		for _, db := range r.dashboards {
			if contains(allRSDashboardIDs, db.Name) {
				assert.Equal(t, addoncfg.AnalyticsNamespace, db.Namespace,
					"RS dashboard %q must be in analytics namespace", db.Name)
			}
		}
	})

	t.Run("single analytics namespace object", func(t *testing.T) {
		count := 0
		for _, ns := range r.namespaces {
			if ns.Name == addoncfg.AnalyticsNamespace {
				count++
			}
		}
		assert.Equal(t, 1, count, "exactly one analytics namespace object should be rendered")
	})
}

// TestRightSizing_ProfileVariableMatchers verifies that cpu_profile and memory_profile variables
// query the correct label_values from CPU and memory metrics respectively, preventing cross-contamination.
func TestRightSizing_ProfileVariableMatchers(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	for _, db := range r.dashboards {
		if !contains(allRSDashboardIDs, db.Name) {
			continue
		}

		t.Run(db.Name+"/cpu_profile_queries_cpu_metric", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)

			var spec map[string]any
			require.NoError(t, json.Unmarshal(raw, &spec))

			varSlice := extractVariableSlice(spec)
			cpuProfileVar := findVariable(varSlice, "cpu_profile")
			require.NotNil(t, cpuProfileVar, "cpu_profile variable must exist")

			varJSON, _ := json.Marshal(cpuProfileVar)
			varStr := string(varJSON)
			assert.Contains(t, varStr, "cpu_usage",
				"cpu_profile variable must query from a cpu_usage metric")
			assert.NotContains(t, varStr, "memory_usage",
				"cpu_profile variable must NOT query from a memory_usage metric")
		})

		t.Run(db.Name+"/memory_profile_queries_memory_metric", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)

			var spec map[string]any
			require.NoError(t, json.Unmarshal(raw, &spec))

			varSlice := extractVariableSlice(spec)
			memProfileVar := findVariable(varSlice, "memory_profile")
			require.NotNil(t, memProfileVar, "memory_profile variable must exist")

			varJSON, _ := json.Marshal(memProfileVar)
			varStr := string(varJSON)
			assert.Contains(t, varStr, "memory_usage",
				"memory_profile variable must query from a memory_usage metric")
			assert.NotContains(t, varStr, "cpu_usage",
				"memory_profile variable must NOT query from a cpu_usage metric")
		})
	}
}

// TestRightSizing_NoCrossContamination verifies that CPU panel queries use $cpu_profile
// and memory panel queries use $memory_profile, never the opposite.
func TestRightSizing_NoCrossContamination(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	for _, db := range r.dashboards {
		if !contains(allRSDashboardIDs, db.Name) {
			continue
		}

		t.Run(db.Name+"/cpu_queries_do_not_use_memory_profile", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)
			specStr := string(raw)

			assert.NotContains(t, specStr, `cpu_recommendation{cluster=\"$cluster\", profile=\"$memory_profile\"`,
				"CPU recommendation queries must not use $memory_profile")
			assert.NotContains(t, specStr, `cpu_usage{cluster=\"$cluster\", profile=\"$memory_profile\"`,
				"CPU usage queries must not use $memory_profile")
		})

		t.Run(db.Name+"/memory_queries_do_not_use_cpu_profile", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)
			specStr := string(raw)

			assert.NotContains(t, specStr, `memory_recommendation{cluster=\"$cluster\", profile=\"$cpu_profile\"`,
				"Memory recommendation queries must not use $cpu_profile")
			assert.NotContains(t, specStr, `memory_usage{cluster=\"$cluster\", profile=\"$cpu_profile\"`,
				"Memory usage queries must not use $cpu_profile")
		})
	}
}

// TestRightSizing_DrillDownLinksPassBothProfiles verifies that VM drill-down links
// include both cpu_profile and memory_profile URL parameters.
func TestRightSizing_DrillDownLinksPassBothProfiles(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	vmOverviewDB := findDashboard(r.dashboards, "acm-rightsizing-openshift-virtualization")
	require.NotNil(t, vmOverviewDB, "VM overview dashboard must exist")

	raw, err := json.Marshal(vmOverviewDB.Spec)
	require.NoError(t, err)
	specStr := string(raw)

	t.Run("drill-down links carry cpu_profile", func(t *testing.T) {
		assert.Contains(t, specStr, "cpu_profile=$cpu_profile",
			"drill-down URL must pass cpu_profile variable")
	})

	t.Run("drill-down links carry memory_profile", func(t *testing.T) {
		assert.Contains(t, specStr, "memory_profile=$memory_profile",
			"drill-down URL must pass memory_profile variable")
	})

	t.Run("no stale profile= in drill-down links", func(t *testing.T) {
		assert.NotContains(t, specStr, "var-profile=",
			"drill-down URL must not use old 'profile' variable")
	})
}

// TestRightSizing_MCOMode_NoDashboardsWhenNotDelegated verifies no RS dashboards render in MCO mode.
func TestRightSizing_MCOMode_NoDashboardsWhenNotDelegated(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	objects := renderRSManifests(t, true, cv)
	r := classify(objects)

	t.Run("no RS dashboards without delegation", func(t *testing.T) {
		for _, db := range r.dashboards {
			assert.NotContains(t, allRSDashboardIDs, db.Name,
				"RS dashboards must not render in MCO mode (rightSizingDelegated absent)")
		}
	})

	t.Run("no analytics namespace without delegation", func(t *testing.T) {
		for _, ns := range r.namespaces {
			assert.NotEqual(t, addoncfg.AnalyticsNamespace, ns.Name,
				"analytics namespace must not be created in MCO mode")
		}
	})
}

// --- Helpers ---

func dashboardNames(dbs []*persesv1.PersesDashboard) []string {
	names := make([]string, 0, len(dbs))
	for _, db := range dbs {
		names = append(names, db.Name)
	}
	return names
}

func contains(slice []string, val string) bool {
	return slices.Contains(slice, val)
}

func extractVariableSlice(spec map[string]any) []any {
	variables, ok := spec["variables"]
	if !ok {
		return nil
	}
	varSlice, ok := variables.([]any)
	if !ok {
		return nil
	}
	return varSlice
}

func findVariable(vars []any, name string) map[string]any {
	for _, v := range vars {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		spec, ok := m["spec"].(map[string]any)
		if !ok {
			continue
		}
		if n, ok := spec["name"].(string); ok && n == name {
			return m
		}
	}
	return nil
}

func findDashboard(dbs []*persesv1.PersesDashboard, name string) *persesv1.PersesDashboard {
	for _, db := range dbs {
		if db.Name == name {
			return db
		}
	}
	return nil
}

func extractDashboardVarNames(vars []any) []string {
	names := make([]string, 0, len(vars))
	for _, v := range vars {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		spec, ok := m["spec"].(map[string]any)
		if !ok {
			continue
		}
		if name, ok := spec["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}
