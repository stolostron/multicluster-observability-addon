package handlers

import (
	"testing"

	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestBuildAlertmanagerEndpoints verifies that mcoa-gateway is populated with
// one Alertmanager pod URL per StatefulSet replica. StatefulSets are discovered
// via the "alertmanager-operated" governing Service set by Prometheus Operator,
// but URLs are built using the "alertmanager" headless Service.
func TestBuildAlertmanagerEndpoints(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))

	alertmanager := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-platform", Namespace: mconfig.HubInstallNamespace},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: alertmanagerOperatedService,
			Replicas:    ptr.To(int32(2)),
		},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(alertmanager).Build()

	endpoints, err := BuildAlertmanagerEndpoints(t.Context(), k8s)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"https://alertmanager-platform-0.alertmanager-operated." + mconfig.HubInstallNamespace + ".svc:9095",
		"https://alertmanager-platform-1.alertmanager-operated." + mconfig.HubInstallNamespace + ".svc:9095",
	}, endpoints)
}

// TestBuildAlertmanagerEndpoints_NoStatefulSet verifies that mcoa-gateway gets
// no Alertmanager endpoints (instead of an error) when Alertmanager hasn't
// been deployed yet.
func TestBuildAlertmanagerEndpoints_NoStatefulSet(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	k8s := fake.NewClientBuilder().WithScheme(scheme).Build()

	endpoints, err := BuildAlertmanagerEndpoints(t.Context(), k8s)
	require.NoError(t, err)
	assert.Empty(t, endpoints)
}
