package coo

import (
	"encoding/json"
	"slices"
	"testing"

	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	cooresource "github.com/stolostron/multicluster-observability-addon/internal/coo/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	namespaceDashboardID = "acm-rs-namespace-overview"
	vmDashboardIDs       = []string{
		"acm-rightsizing-openshift-virtualization",
		"acm-rightsizing-vm-overestimation",
		"acm-rightsizing-vm-underestimation",
	}
	allRSDashboardIDs = append([]string{namespaceDashboardID}, vmDashboardIDs...)
)

func buildOpts(cv []addonapiv1beta1.CustomizedVariable) addon.Options {
	aodc := &addonapiv1beta1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addoncfg.Name,
			Namespace: addoncfg.InstallNamespace,
		},
		Spec: addonapiv1beta1.AddOnDeploymentConfigSpec{
			CustomizedVariables: cv,
		},
	}
	opts, err := addon.BuildOptions(aodc)
	if err != nil {
		panic(err)
	}
	return opts
}

func reconcileHubResources(t *testing.T, cv []addonapiv1beta1.CustomizedVariable) client.Client {
	t.Helper()

	cmao := &addonapiv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{Name: addoncfg.Name},
	}

	k8s := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(cmao).
		Build()

	reconciler := &cooresource.HubResourceReconciler{
		Client: k8s,
		CMAO:   cmao,
		Logger: klog.Background(),
		Opts:   buildOpts(cv),
	}

	err := reconciler.Reconcile(t.Context(), false)
	require.NoError(t, err)
	return k8s
}

func listDashboards(t *testing.T, k8s client.Client, namespace string) []persesv1.PersesDashboard {
	t.Helper()
	list := &persesv1.PersesDashboardList{}
	require.NoError(t, k8s.List(t.Context(), list, client.InNamespace(namespace)))
	return list.Items
}

func dashboardNames(dbs []persesv1.PersesDashboard) []string {
	names := make([]string, 0, len(dbs))
	for _, db := range dbs {
		names = append(names, db.Name)
	}
	return names
}

func contains(slice []string, val string) bool {
	return slices.Contains(slice, val)
}

func TestRightSizing_HubCluster_BothEnabled(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	k8s := reconcileHubResources(t, cv)
	dashboards := listDashboards(t, k8s, addoncfg.AnalyticsNamespace)

	t.Run("creates analytics namespace", func(t *testing.T) {
		ns := &corev1.Namespace{}
		err := k8s.Get(t.Context(), client.ObjectKey{Name: addoncfg.AnalyticsNamespace}, ns)
		require.NoError(t, err, "observability-analytics namespace must be created")
	})

	t.Run("creates analytics datasource", func(t *testing.T) {
		ds := &persesv1.PersesDatasource{}
		err := k8s.Get(t.Context(), client.ObjectKey{
			Namespace: addoncfg.AnalyticsNamespace,
			Name:      "rbac-query-proxy-datasource",
		}, ds)
		require.NoError(t, err, "rbac-query-proxy-datasource must exist in analytics namespace")
	})

	t.Run("creates all 4 right-sizing dashboards", func(t *testing.T) {
		names := dashboardNames(dashboards)
		for _, expected := range allRSDashboardIDs {
			assert.Contains(t, names, expected, "dashboard %q should be rendered", expected)
		}
	})

	t.Run("all RS dashboards are in the analytics namespace", func(t *testing.T) {
		for _, db := range dashboards {
			if contains(allRSDashboardIDs, db.Name) {
				assert.Equal(t, addoncfg.AnalyticsNamespace, db.Namespace,
					"dashboard %q must be in %s", db.Name, addoncfg.AnalyticsNamespace)
			}
		}
	})

	t.Run("no RS dashboards in the install namespace", func(t *testing.T) {
		installDashboards := listDashboards(t, k8s, addoncfg.InstallNamespace)
		for _, db := range installDashboards {
			assert.NotContains(t, allRSDashboardIDs, db.Name,
				"RS dashboard %q should NOT be in install namespace", db.Name)
		}
	})

	t.Run("dashboard specs contain valid JSON with expected metrics", func(t *testing.T) {
		for _, db := range dashboards {
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

	k8s := reconcileHubResources(t, cv)
	dashboards := listDashboards(t, k8s, addoncfg.AnalyticsNamespace)

	t.Run("creates namespace RS dashboard only", func(t *testing.T) {
		names := dashboardNames(dashboards)
		assert.Contains(t, names, namespaceDashboardID)
		for _, vmID := range vmDashboardIDs {
			assert.NotContains(t, names, vmID, "VM dashboard %q should not be rendered", vmID)
		}
	})

	t.Run("analytics namespace and datasource still created", func(t *testing.T) {
		ns := &corev1.Namespace{}
		err := k8s.Get(t.Context(), client.ObjectKey{Name: addoncfg.AnalyticsNamespace}, ns)
		require.NoError(t, err)

		ds := &persesv1.PersesDatasource{}
		err = k8s.Get(t.Context(), client.ObjectKey{
			Namespace: addoncfg.AnalyticsNamespace,
			Name:      "rbac-query-proxy-datasource",
		}, ds)
		require.NoError(t, err)
	})
}

func TestRightSizing_HubCluster_VirtualizationOnly(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "disabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	k8s := reconcileHubResources(t, cv)
	dashboards := listDashboards(t, k8s, addoncfg.AnalyticsNamespace)

	t.Run("creates 3 VM RS dashboards, no namespace dashboard", func(t *testing.T) {
		names := dashboardNames(dashboards)
		assert.NotContains(t, names, namespaceDashboardID)
		for _, vmID := range vmDashboardIDs {
			assert.Contains(t, names, vmID, "VM dashboard %q should be rendered", vmID)
		}
	})
}

func TestRightSizing_HubCluster_BothDisabled(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "disabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "disabled"},
	}

	k8s := reconcileHubResources(t, cv)
	dashboards := listDashboards(t, k8s, addoncfg.AnalyticsNamespace)

	t.Run("no RS dashboards rendered", func(t *testing.T) {
		for _, db := range dashboards {
			assert.NotContains(t, allRSDashboardIDs, db.Name,
				"no RS dashboards should exist when both are disabled")
		}
	})
}

func TestRightSizing_DashboardSpecStructure(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyRightSizingDelegated, Value: "true"},
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	k8s := reconcileHubResources(t, cv)
	dashboards := listDashboards(t, k8s, addoncfg.AnalyticsNamespace)

	for _, db := range dashboards {
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

func TestRightSizing_MCOMode_NoDashboardsWhenNotDelegated(t *testing.T) {
	cv := []addonapiv1beta1.CustomizedVariable{
		{Name: addon.KeyPlatformNamespaceRightSizing, Value: "enabled"},
		{Name: addon.KeyPlatformVirtualizationRightSizing, Value: "enabled"},
	}

	k8s := reconcileHubResources(t, cv)
	dashboards := listDashboards(t, k8s, addoncfg.AnalyticsNamespace)

	t.Run("no RS dashboards without delegation", func(t *testing.T) {
		for _, db := range dashboards {
			assert.NotContains(t, allRSDashboardIDs, db.Name,
				"RS dashboards must not render in MCO mode (rightSizingDelegated absent)")
		}
	})
}
