package handlers

import (
	"context"
	"testing"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestDefaultStackResourcesSurviveDeleteOrphan(t *testing.T) {
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

	objs, _, err := BuildDefaultStackResources(t.Context(), fakeClient, cmao, addon.LogsOptions{DefaultStack: true}, addon.LogsOptions{}, "hub.example.com")
	require.NoError(t, err)

	var clf *loggingv1.ClusterLogForwarder
	var ls *lokiv1.LokiStack
	for _, obj := range objs {
		switch o := obj.(type) {
		case *loggingv1.ClusterLogForwarder:
			clf = o
		case *lokiv1.LokiStack:
			ls = o
		}
	}
	require.NotNil(t, clf, "expected a default-stack ClusterLogForwarder")
	require.NotNil(t, ls, "expected a default-stack LokiStack")

	wantPlacement := addoncfg.GlobalPlacementNamespace + "/" + addoncfg.GlobalPlacementName
	assert.Equal(t, wantPlacement, clf.Annotations[addoncfg.PlacementAnnotationKey], "CLF must carry the placement annotation so DeleteOrphanResources keeps it")
	assert.Equal(t, wantPlacement, ls.Annotations[addoncfg.PlacementAnnotationKey], "LokiStack must carry the placement annotation so DeleteOrphanResources keeps it")

	for _, obj := range []client.Object{clf, ls} {
		require.NoError(t, controllerutil.SetControllerReference(cmao, obj, scheme))
		require.NoError(t, fakeClient.Create(context.Background(), obj))
	}

	require.NoError(t, common.DeleteOrphanResources(context.Background(), klog.Background(), fakeClient, cmao, &loggingv1.ClusterLogForwarderList{}))
	require.NoError(t, common.DeleteOrphanResources(context.Background(), klog.Background(), fakeClient, cmao, &lokiv1.LokiStackList{}))

	gotCLF := &loggingv1.ClusterLogForwarder{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: clf.Name, Namespace: clf.Namespace}, gotCLF)
	assert.NoError(t, err, "CMAO-owned ClusterLogForwarder with a matching placement annotation must not be deleted")

	gotLS := &lokiv1.LokiStack{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: ls.Name, Namespace: ls.Namespace}, gotLS)
	assert.NoError(t, err, "CMAO-owned LokiStack with a matching placement annotation must not be deleted")
}
