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
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
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

func renderRSManifests(t *testing.T, isHub bool, cv []addonapiv1alpha1.CustomizedVariable) []runtime.Object {
	t.Helper()

	mc := addontesting.NewManagedCluster("cluster-1")
	if isHub {
		mc.Labels = map[string]string{"local-cluster": "true"}
	}

	mcao := addontesting.NewAddon("test", "cluster-1")
	mcao.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: addoncfg.InstallNamespace,
				Name:      addoncfg.Name,
			},
			DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
				ConfigReferent: addonapiv1alpha1.ConfigReferent{
					Namespace: addoncfg.InstallNamespace,
					Name:      addoncfg.Name,
				},
				SpecHash: "fake-spec-hash",
			},
		},
	}

	addc := &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: mcao.ObjectMeta,
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: cv,
		},
	}
	addc.Name = addoncfg.Name
	addc.Namespace = addoncfg.InstallNamespace

	cooAgent := newCOOAgentAddon([]client.Object{mcao}, addc)
	objects, err := cooAgent.Manifests(mc, mcao)
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
	cv := []addonapiv1alpha1.CustomizedVariable{
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
	cv := []addonapiv1alpha1.CustomizedVariable{
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
	cv := []addonapiv1alpha1.CustomizedVariable{
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
	cv := []addonapiv1alpha1.CustomizedVariable{
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
	cv := []addonapiv1alpha1.CustomizedVariable{
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
	cv := []addonapiv1alpha1.CustomizedVariable{
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
			assert.GreaterOrEqual(t, len(varSlice), 3, "all RS dashboards have at least cluster, profile, days")
		})

		t.Run(db.Name+"/spec_references_datasource", func(t *testing.T) {
			raw, err := json.Marshal(db.Spec)
			require.NoError(t, err)
			assert.Contains(t, string(raw), "rbac-query-proxy-datasource")
		})
	}
}

func TestRightSizing_CombinedWithIncidentDetection(t *testing.T) {
	cv := []addonapiv1alpha1.CustomizedVariable{
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
