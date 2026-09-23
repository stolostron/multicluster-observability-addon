package resource

import (
	"testing"

	"github.com/go-logr/logr"
	persesv1 "github.com/perses/perses-operator/api/v1alpha1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = persesv1.AddToScheme(scheme)
	return scheme
}

func newDashboard(name, namespace string) *persesv1.PersesDashboard {
	return &persesv1.PersesDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestCleanupOrphanDashboards(t *testing.T) {
	knownNames := allPossibleDashboardNames()
	require.NotEmpty(t, knownNames, "allPossibleDashboardNames should return at least one name")

	var knownName string
	for name := range knownNames {
		knownName = name
		break
	}

	t.Run("deletes orphan dashboard with known name", func(t *testing.T) {
		orphan := newDashboard(knownName, addoncfg.InstallNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(orphan).Build()

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), map[string]struct{}{})
		require.NoError(t, err)

		list := &persesv1.PersesDashboardList{}
		require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(addoncfg.InstallNamespace)))
		assert.Empty(t, list.Items)
	})

	t.Run("preserves user-created dashboard with unknown name", func(t *testing.T) {
		userDashboard := newDashboard("my-custom-dashboard", addoncfg.InstallNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(userDashboard).Build()

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), map[string]struct{}{})
		require.NoError(t, err)

		list := &persesv1.PersesDashboardList{}
		require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(addoncfg.InstallNamespace)))
		assert.Len(t, list.Items, 1)
		assert.Equal(t, "my-custom-dashboard", list.Items[0].Name)
	})

	t.Run("preserves currently desired dashboard", func(t *testing.T) {
		desired := newDashboard(knownName, addoncfg.InstallNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(desired).Build()

		desiredNames := map[string]struct{}{
			addoncfg.InstallNamespace + "/" + knownName: {},
		}

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), desiredNames)
		require.NoError(t, err)

		list := &persesv1.PersesDashboardList{}
		require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(addoncfg.InstallNamespace)))
		assert.Len(t, list.Items, 1)
	})

	t.Run("cleans up across both namespaces", func(t *testing.T) {
		orphan1 := newDashboard(knownName, addoncfg.InstallNamespace)
		orphan2 := newDashboard(knownName, addoncfg.AnalyticsNamespace)
		fakeClient := fake.NewClientBuilder().WithScheme(newTestScheme()).
			WithObjects(orphan1, orphan2).Build()

		r := &HubResourceReconciler{
			Client: fakeClient,
			Logger: logr.Discard(),
		}

		err := r.cleanupOrphanDashboards(t.Context(), map[string]struct{}{})
		require.NoError(t, err)

		for _, ns := range []string{addoncfg.InstallNamespace, addoncfg.AnalyticsNamespace} {
			list := &persesv1.PersesDashboardList{}
			require.NoError(t, fakeClient.List(t.Context(), list, client.InNamespace(ns)))
			assert.Empty(t, list.Items, "namespace %s should be empty", ns)
		}
	})
}

func TestAllPossibleDashboardNames(t *testing.T) {
	names := allPossibleDashboardNames()
	assert.NotEmpty(t, names)

	second := allPossibleDashboardNames()
	assert.Equal(t, names, second, "should return the same set on subsequent calls")
}
