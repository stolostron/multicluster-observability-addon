package handlers

import (
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

// buildTestScheme registers all types required by the logging handler tests.
func buildTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))
	require.NoError(t, lokiv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

// buildTestCMAO returns a CMAO with the given placement strategies.
func buildTestCMAO(placements ...addonapiv1beta1.PlacementStrategy) *addonapiv1beta1.ClusterManagementAddOn {
	return &addonapiv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  "cmao-uid",
		},
		Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonapiv1beta1.InstallStrategy{
				Placements: placements,
			},
		},
	}
}

// globalPlacement returns a PlacementStrategy referencing the global placement.
func globalPlacement() addonapiv1beta1.PlacementStrategy {
	return addonapiv1beta1.PlacementStrategy{
		PlacementRef: addoncfg.GlobalPlacementRef,
	}
}
