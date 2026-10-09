package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsNetworkPoliciesEnabled(t *testing.T) {
	t.Run("nil object returns false", func(t *testing.T) {
		assert.False(t, IsNetworkPoliciesEnabled(nil))
	})

	t.Run("missing spec returns false", func(t *testing.T) {
		u := NewMultiClusterHub()
		assert.False(t, IsNetworkPoliciesEnabled(u))
	})

	t.Run("missing networkPolicies returns false", func(t *testing.T) {
		u := NewMultiClusterHub()
		u.Object["spec"] = map[string]any{}
		assert.False(t, IsNetworkPoliciesEnabled(u))
	})

	t.Run("networkPolicies enabled false returns false", func(t *testing.T) {
		u := NewMultiClusterHub()
		require.NoError(t, unstructured.SetNestedField(u.Object, false, "spec", "networkPolicies", "enabled"))
		assert.False(t, IsNetworkPoliciesEnabled(u))
	})

	t.Run("networkPolicies enabled true returns true", func(t *testing.T) {
		u := NewMultiClusterHub()
		require.NoError(t, unstructured.SetNestedField(u.Object, true, "spec", "networkPolicies", "enabled"))
		assert.True(t, IsNetworkPoliciesEnabled(u))
	})
}

func TestGetNetworkPoliciesEnabled(t *testing.T) {
	s := runtime.NewScheme()

	t.Run("returns false when no MCH exists", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(s).Build()
		enabled, err := GetNetworkPoliciesEnabled(t.Context(), cl)
		require.NoError(t, err)
		assert.False(t, enabled)
	})

	t.Run("returns true when MCH has networkPolicies enabled", func(t *testing.T) {
		mch := NewMultiClusterHub()
		mch.SetName("multiclusterhub")
		mch.SetNamespace("open-cluster-management")
		require.NoError(t, unstructured.SetNestedField(mch.Object, true, "spec", "networkPolicies", "enabled"))

		cl := fake.NewClientBuilder().WithScheme(s).WithObjects(mch).Build()
		enabled, err := GetNetworkPoliciesEnabled(t.Context(), cl)
		require.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("returns false when MCH has networkPolicies disabled", func(t *testing.T) {
		mch := NewMultiClusterHub()
		mch.SetName("multiclusterhub")
		mch.SetNamespace("open-cluster-management")
		require.NoError(t, unstructured.SetNestedField(mch.Object, false, "spec", "networkPolicies", "enabled"))

		cl := fake.NewClientBuilder().WithScheme(s).WithObjects(mch).Build()
		enabled, err := GetNetworkPoliciesEnabled(t.Context(), cl)
		require.NoError(t, err)
		assert.False(t, enabled)
	})
}
