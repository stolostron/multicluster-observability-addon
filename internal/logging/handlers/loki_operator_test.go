package handlers

import (
	"testing"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newOLMTestScheme(t *testing.T) *runtime.Scheme {
	scheme := newTestScheme(t)
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, operatorsv1.AddToScheme(scheme))
	require.NoError(t, operatorsv1alpha1.AddToScheme(scheme))
	return scheme
}

func TestReconcileLokiOperator(t *testing.T) {
	ctx := t.Context()

	t.Run("applies the Namespace, OperatorGroup and Subscription when enabled", func(t *testing.T) {
		scheme := newOLMTestScheme(t)
		cmao := newTestCMAO()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()

		require.NoError(t, ReconcileLokiOperator(ctx, fakeClient, cmao, true))

		require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{Name: manifests.LokiOperatorNamespace}, &corev1.Namespace{}))
		require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{
			Name:      manifests.LokiOperatorPackageName,
			Namespace: manifests.LokiOperatorNamespace,
		}, &operatorsv1.OperatorGroup{}))
		require.NoError(t, fakeClient.Get(ctx, client.ObjectKey{
			Name:      manifests.LokiOperatorPackageName,
			Namespace: manifests.LokiOperatorNamespace,
		}, &operatorsv1alpha1.Subscription{}))
	})

	t.Run("applies nothing when disabled", func(t *testing.T) {
		scheme := newOLMTestScheme(t)
		cmao := newTestCMAO()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cmao).Build()

		require.NoError(t, ReconcileLokiOperator(ctx, fakeClient, cmao, false))

		err := fakeClient.Get(ctx, client.ObjectKey{Name: manifests.LokiOperatorNamespace}, &corev1.Namespace{})
		require.Error(t, err, "Namespace should not be created when disabled")
	})
}
