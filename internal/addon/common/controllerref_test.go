package common

import (
	"errors"
	"testing"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const testConfigMapsResource = "configmaps"

func TestIsAbsentOwnedConfig(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "missing resource", err: errMissingResource, want: true},
		{name: "missing owner ref", err: errMissingOwnerRef, want: true},
		{name: "wrapped missing resource", err: errors.Join(errMissingResource), want: true},
		{name: "no refs is not absent-owned", err: errMissingResourceRefs, want: false},
		{name: "unrelated error", err: errors.New("boom"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsAbsentOwnedConfig(tt.err))
		})
	}
}

func TestGetResourceWithOwnerRef(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, addonv1beta1.Install(scheme))

	cmao := &addonv1beta1.ClusterManagementAddOn{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterManagementAddOn",
			APIVersion: addonv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
			UID:  "cmao-uid",
		},
	}

	owned := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "owned-cm", Namespace: "ns"},
	}
	require.NoError(t, controllerutil.SetOwnerReference(cmao, owned, scheme))

	unowned := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "user-cm", Namespace: "ns"},
	}

	configRef := func(names ...string) []addonv1beta1.ConfigReference {
		refs := make([]addonv1beta1.ConfigReference, 0, len(names))
		for _, name := range names {
			refs = append(refs, addonv1beta1.ConfigReference{
				ConfigGroupResource: addonv1beta1.ConfigGroupResource{
					Group:    "",
					Resource: testConfigMapsResource,
				},
				DesiredConfig: &addonv1beta1.ConfigSpecHash{
					ConfigReferent: addonv1beta1.ConfigReferent{
						Name:      name,
						Namespace: "ns",
					},
				},
			})
		}
		return refs
	}

	mcAddon := func(names ...string) *addonv1beta1.ManagedClusterAddOn {
		return &addonv1beta1.ManagedClusterAddOn{
			Status: addonv1beta1.ManagedClusterAddOnStatus{
				ConfigReferences: configRef(names...),
			},
		}
	}

	get := func(t *testing.T, objs []client.Object, addon *addonv1beta1.ManagedClusterAddOn) (*corev1.ConfigMap, error) {
		t.Helper()
		k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
		return GetResourceWithOwnerRef(t.Context(), k8s, addon, "", testConfigMapsResource, &corev1.ConfigMap{})
	}

	t.Run("no config references", func(t *testing.T) {
		_, err := get(t, nil, mcAddon())
		require.ErrorIs(t, err, errMissingResourceRefs)
		assert.False(t, IsAbsentOwnedConfig(err))
	})

	t.Run("skips NotFound keys and returns owned object", func(t *testing.T) {
		got, err := get(t, []client.Object{owned}, mcAddon("gone-cm", "owned-cm"))
		require.NoError(t, err)
		assert.Equal(t, "owned-cm", got.Name)
	})

	t.Run("all keys NotFound", func(t *testing.T) {
		_, err := get(t, nil, mcAddon("gone-cm"))
		require.ErrorIs(t, err, errMissingResource)
		assert.True(t, IsAbsentOwnedConfig(err))
	})

	t.Run("present without owner ref", func(t *testing.T) {
		_, err := get(t, []client.Object{unowned}, mcAddon("user-cm"))
		require.ErrorIs(t, err, errMissingOwnerRef)
		assert.True(t, IsAbsentOwnedConfig(err))
	})

	t.Run("returns owned object", func(t *testing.T) {
		got, err := get(t, []client.Object{owned}, mcAddon("owned-cm"))
		require.NoError(t, err)
		assert.Equal(t, "owned-cm", got.Name)
	})
}
