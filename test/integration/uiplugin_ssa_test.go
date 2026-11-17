package integration

import (
	"context"
	"encoding/json"
	"testing"

	uiplugin "github.com/rhobs/observability-operator/pkg/apis/uiplugin/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var uiPluginGVK = schema.GroupVersionKind{
	Group:   "observability.openshift.io",
	Version: "v1alpha1",
	Kind:    "UIPlugin",
}

// mcoaUIPlugin returns the UIPlugin carrying only the fields MCOA renders:
// spec.type, spec.monitoring.incidents, spec.monitoring.acm (incl. the
// alertmanager/thanosQuerier URLs) and spec.monitoring.perses. It deliberately
// cannot express spec.monitoring.clusterHealthAnalyzer -- the vendored API has
// no such field -- which is exactly why MCOA must not clobber it.
func mcoaUIPlugin(name string) *uiplugin.UIPlugin {
	return &uiplugin.UIPlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "observability.openshift.io/v1alpha1",
			Kind:       "UIPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: uiplugin.UIPluginSpec{
			Type: uiplugin.TypeMonitoring,
			Monitoring: &uiplugin.MonitoringConfig{
				Incidents: &uiplugin.IncidentsReference{Enabled: true},
				ACM: &uiplugin.AdvancedClusterManagementReference{
					Enabled:       true,
					Alertmanager:  uiplugin.AlertmanagerReference{Url: "https://alertmanager.open-cluster-management-observability.svc:9095"},
					ThanosQuerier: uiplugin.ThanosQuerierReference{Url: "https://rbac-query-proxy.open-cluster-management-observability.svc:8443"},
				},
				Perses: &uiplugin.PersesReference{Enabled: true},
			},
		},
	}
}

// applyExternalUIPlugin creates the monitoring UIPlugin on behalf of an external
// manager (admin or a newer COO), owning clusterHealthAnalyzer and an explicit
// incidents=false under a field manager that is NOT MCOA.
func applyExternalUIPlugin(t *testing.T, ctx context.Context, c client.Client, name string) {
	t.Helper()
	admin := &unstructured.Unstructured{}
	admin.SetGroupVersionKind(uiPluginGVK)
	admin.SetName(name)
	require.NoError(t, unstructured.SetNestedField(admin.Object, "Monitoring", "spec", "type"))
	require.NoError(t, unstructured.SetNestedField(admin.Object, true, "spec", "monitoring", "clusterHealthAnalyzer", "enabled"))
	require.NoError(t, unstructured.SetNestedField(admin.Object, false, "spec", "monitoring", "incidents", "enabled"))
	require.NoError(t, c.Apply(ctx, client.ApplyConfigurationFromUnstructured(admin),
		client.FieldOwner("external-admin"), client.ForceOwnership))
}

func getUIPlugin(t *testing.T, ctx context.Context, c client.Client, name string) *unstructured.Unstructured {
	t.Helper()
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(uiPluginGVK)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: name}, got))
	return got
}

// TestUIPlugin_ServerSideApply validates the Kubernetes server-side-apply
// ownership semantics that the uiplugins ManifestConfigOption relies on (see
// addon.ManifestConfigs, which selects UpdateStrategyTypeServerSideApply with
// Force: true for the monitoring UIPlugin). It does NOT exercise the OCM Work
// Agent that consumes that option -- TestManifestConfigs_UIPluginServerSideApply
// covers the config wiring; this test covers the behavior that wiring delegates
// to. The negative control sub-test demonstrates that the addon-framework
// default (full-object Update) is what strips clusterHealthAnalyzer, i.e. the
// regression this fix prevents.
func TestUIPlugin_ServerSideApply(t *testing.T) {
	name := addoncfg.IDetectionUIPluginName // "monitoring"

	// Server-side apply of the fields MCOA renders must preserve an externally
	// owned clusterHealthAnalyzer, and ForceOwnership must let MCOA assert the
	// fields it owns over a pre-existing UIPlugin without an apply conflict.
	t.Run("ssa preserves externally-owned fields and force-wins on MCOA fields", func(t *testing.T) {
		te := SetupTestEnv(t)
		ctx := context.Background()
		c := te.K8sClient

		applyExternalUIPlugin(t, ctx, c, name)

		// MCOA applies through the shared SSA helper (FieldOwner=addoncfg.Name,
		// ForceOwnership) -- the same ownership model the Work Agent's SSA update
		// strategy uses.
		require.NoError(t, common.ServerSideApply(ctx, c, mcoaUIPlugin(name), nil))

		got := getUIPlugin(t, ctx, c, name)

		// The field MCOA never renders survives the apply.
		cha, found, err := unstructured.NestedBool(got.Object, "spec", "monitoring", "clusterHealthAnalyzer", "enabled")
		require.NoError(t, err)
		require.True(t, found, "spec.monitoring.clusterHealthAnalyzer must be preserved by server-side apply")
		assert.True(t, cha, "clusterHealthAnalyzer.enabled must keep the external manager's value")

		// ForceOwnership: MCOA's incidents value (true) wins over the admin's false.
		inc, found, err := unstructured.NestedBool(got.Object, "spec", "monitoring", "incidents", "enabled")
		require.NoError(t, err)
		require.True(t, found)
		assert.True(t, inc, "MCOA must assert its own incidents value via ForceOwnership")

		// Field ownership is tracked per manager (the "SSA tracks ownership per
		// field" guarantee): clusterHealthAnalyzer.enabled stays with external-admin
		// while incidents.enabled transfers to MCOA.
		assert.Equal(t, "external-admin", fieldManagerFor(t, got, "f:monitoring", "f:clusterHealthAnalyzer", "f:enabled"),
			"clusterHealthAnalyzer must remain owned by the external manager")
		assert.Equal(t, addoncfg.Name, fieldManagerFor(t, got, "f:monitoring", "f:incidents", "f:enabled"),
			"incidents must be owned by MCOA after ForceOwnership")

		// MCOA's other owned fields are applied.
		amURL, _, _ := unstructured.NestedString(got.Object, "spec", "monitoring", "acm", "alertmanager", "url")
		assert.Equal(t, "https://alertmanager.open-cluster-management-observability.svc:9095", amURL)
		perses, _, _ := unstructured.NestedBool(got.Object, "spec", "monitoring", "perses", "enabled")
		assert.True(t, perses)
		typ, _, _ := unstructured.NestedString(got.Object, "spec", "type")
		assert.Equal(t, "Monitoring", typ)
	})

	// Negative control: the addon-framework default (full-object Update), which
	// is what the fix replaces, prunes the externally-owned clusterHealthAnalyzer.
	// This proves the test above would have caught the original bug and that the
	// CRD schema does not mask the pruning.
	t.Run("negative control: full-object update strips clusterHealthAnalyzer", func(t *testing.T) {
		te := SetupTestEnv(t)
		ctx := context.Background()
		c := te.K8sClient

		applyExternalUIPlugin(t, ctx, c, name)

		// Mimic a full-replace update: Get into the typed object (which cannot
		// hold clusterHealthAnalyzer), overwrite the spec with MCOA's fields and
		// PUT. This is the pre-fix behavior.
		existing := &uiplugin.UIPlugin{}
		require.NoError(t, c.Get(ctx, client.ObjectKey{Name: name}, existing))
		existing.Spec = mcoaUIPlugin(name).Spec
		require.NoError(t, c.Update(ctx, existing))

		got := getUIPlugin(t, ctx, c, name)
		_, found, err := unstructured.NestedBool(got.Object, "spec", "monitoring", "clusterHealthAnalyzer", "enabled")
		require.NoError(t, err)
		assert.False(t, found, "full-object update must strip clusterHealthAnalyzer (the regression the SSA fix prevents)")
	})

	// A UIPlugin that does not exist yet is created by the first apply, and a
	// second identical apply is a no-op (idempotent).
	t.Run("ssa creates when absent and is idempotent", func(t *testing.T) {
		te := SetupTestEnv(t)
		ctx := context.Background()
		c := te.K8sClient

		require.NoError(t, common.ServerSideApply(ctx, c, mcoaUIPlugin(name), nil))
		first := getUIPlugin(t, ctx, c, name)
		firstRV := first.GetResourceVersion()

		require.NoError(t, common.ServerSideApply(ctx, c, mcoaUIPlugin(name), nil))
		second := getUIPlugin(t, ctx, c, name)
		assert.Equal(t, firstRV, second.GetResourceVersion(), "a repeated identical apply must not mutate the object")
	})
}

// fieldManagerFor returns the name of the manager that owns the leaf addressed
// by the given key path under f:spec, read from metadata.managedFields. The full
// path must be present (an empty intermediate map left behind after another
// manager took the leaf does not count as ownership).
func fieldManagerFor(t *testing.T, obj *unstructured.Unstructured, specPath ...string) string {
	t.Helper()
	for _, mf := range obj.GetManagedFields() {
		if mf.FieldsV1 == nil {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal(mf.FieldsV1.GetRawBytes(), &fields); err != nil {
			continue
		}
		node, ok := fields["f:spec"].(map[string]any)
		if !ok {
			continue
		}
		owns := true
		for _, key := range specPath {
			child, ok := node[key].(map[string]any)
			if !ok {
				owns = false
				break
			}
			node = child
		}
		if owns {
			return mf.Manager
		}
	}
	return ""
}
