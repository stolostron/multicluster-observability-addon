package handlers

import (
	"fmt"
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestDefaultStackResourcesSurviveDeleteOrphan(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	require.NoError(t, addonapiv1beta1.Install(scheme))
	require.NoError(t, clusterv1.Install(scheme))
	require.NoError(t, loggingv1.AddToScheme(scheme))
	require.NoError(t, lokiv1.AddToScheme(scheme))

	cmao := &addonapiv1beta1.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  "cmao-uid",
		},
		Spec: addonapiv1beta1.ClusterManagementAddOnSpec{
			InstallStrategy: addonapiv1beta1.InstallStrategy{
				Placements: []addonapiv1beta1.PlacementStrategy{
					{
						PlacementRef: addoncfg.GlobalPlacementRef,
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()

	objs, _, err := BuildDefaultStackResources(ctx, fakeClient, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)

	for _, obj := range objs {
		require.NoError(t, controllerutil.SetControllerReference(cmao, obj, scheme))
		require.NoError(t, fakeClient.Create(ctx, obj))
	}

	require.NoError(t, common.DeleteOrphanResources(ctx, klog.Background(), fakeClient, cmao, &loggingv1.ClusterLogForwarderList{}))
	require.NoError(t, common.DeleteOrphanResources(ctx, klog.Background(), fakeClient, cmao, &lokiv1.LokiStackList{}))

	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName),
		Namespace: addoncfg.InstallNamespace,
	}
	require.NoError(t, fakeClient.Get(ctx, key, &loggingv1.ClusterLogForwarder{}), "CMAO-owned ClusterLogForwarder with a matching placement annotation must not be deleted")
	require.NoError(t, fakeClient.Get(ctx, key, &lokiv1.LokiStack{}), "CMAO-owned LokiStack with a matching placement annotation must not be deleted")
}
