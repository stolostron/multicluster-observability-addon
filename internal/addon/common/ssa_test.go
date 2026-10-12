package common_test

import (
	"context"
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type dummyUnregisteredObject struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (d *dummyUnregisteredObject) DeepCopyObject() runtime.Object {
	cp := *d
	return &cp
}

func TestServerSideApply(t *testing.T) {
	ctx := context.Background()

	t.Run("nil object returns error", func(t *testing.T) {
		scheme := runtime.NewScheme()
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		err := common.ServerSideApply(ctx, c, nil, nil)
		require.ErrorIs(t, err, common.ErrNilObject)
	})

	t.Run("typed nil object returns ErrNilObject", func(t *testing.T) {
		scheme := runtime.NewScheme()
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		var u *unstructured.Unstructured
		err := common.ServerSideApply(ctx, c, u, nil)
		require.ErrorIs(t, err, common.ErrNilObject)

		var cm *corev1.ConfigMap
		err = common.ServerSideApply(ctx, c, cm, nil)
		require.ErrorIs(t, err, common.ErrNilObject)
	})

	t.Run("typed nil owner does not attempt controller reference assignment", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "typed-nil-owner-cm",
				Namespace: "default",
			},
			Data: map[string]string{"foo": "bar"},
		}

		var nilOwner *corev1.ConfigMap
		err := common.ServerSideApply(ctx, c, cm, nilOwner)
		require.NoError(t, err)

		var retrieved corev1.ConfigMap
		err = c.Get(ctx, types.NamespacedName{Name: "typed-nil-owner-cm", Namespace: "default"}, &retrieved)
		require.NoError(t, err)
		assert.Empty(t, retrieved.GetOwnerReferences())
	})

	t.Run("applies typed object and synchronizes metadata and GVK", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "default",
				Labels: map[string]string{
					"app": "mcoa",
				},
				Annotations: map[string]string{
					"note": "synced",
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		err := common.ServerSideApply(ctx, c, cm, nil)
		require.NoError(t, err)

		// Assert server-managed metadata, labels, annotations and GVK are synchronized back onto the typed object
		assert.NotEmpty(t, cm.GetResourceVersion())
		assert.Equal(t, "mcoa", cm.Labels["app"])
		assert.Equal(t, "synced", cm.Annotations["note"])
		assert.Equal(t, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, cm.GetObjectKind().GroupVersionKind())

		// Verify object exists in client store
		var retrieved corev1.ConfigMap
		err = c.Get(ctx, types.NamespacedName{Name: "test-cm", Namespace: "default"}, &retrieved)
		require.NoError(t, err)
		assert.Equal(t, "value", retrieved.Data["key"])
		assert.Equal(t, cm.GetResourceVersion(), retrieved.GetResourceVersion())
		assert.Equal(t, "mcoa", retrieved.Labels["app"])
		assert.Equal(t, "synced", retrieved.Annotations["note"])
	})

	t.Run("applies unstructured object and updates in place", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		u := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "unstructured-cm",
					"namespace": "default",
				},
				"data": map[string]any{
					"message": "hello-world",
				},
			},
		}

		err := common.ServerSideApply(ctx, c, u, nil)
		require.NoError(t, err)

		// In-place update checks
		assert.NotEmpty(t, u.GetResourceVersion())
		assert.Equal(t, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, u.GroupVersionKind())

		var retrieved corev1.ConfigMap
		err = c.Get(ctx, types.NamespacedName{Name: "unstructured-cm", Namespace: "default"}, &retrieved)
		require.NoError(t, err)
		assert.Equal(t, "hello-world", retrieved.Data["message"])
	})

	t.Run("sets controller owner reference when owner is provided", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-cm",
				Namespace: "default",
				UID:       types.UID("owner-123"),
			},
		}

		child := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "child-secret",
				Namespace: "default",
			},
		}

		err := common.ServerSideApply(ctx, c, child, owner)
		require.NoError(t, err)

		ownerRefs := child.GetOwnerReferences()
		require.Len(t, ownerRefs, 1)
		assert.Equal(t, "owner-cm", ownerRefs[0].Name)
		assert.Equal(t, types.UID("owner-123"), ownerRefs[0].UID)
		require.NotNil(t, ownerRefs[0].Controller)
		assert.True(t, *ownerRefs[0].Controller)

		var retrieved corev1.Secret
		err = c.Get(ctx, types.NamespacedName{Name: "child-secret", Namespace: "default"}, &retrieved)
		require.NoError(t, err)
		require.Len(t, retrieved.GetOwnerReferences(), 1)
		assert.Equal(t, "owner-cm", retrieved.GetOwnerReferences()[0].Name)
	})

	t.Run("fails to set controller owner reference when owner is invalid", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		unregisteredOwner := &dummyUnregisteredObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-owner",
				Namespace: "default",
			},
		}

		child := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "child-cm",
				Namespace: "default",
			},
		}

		err := common.ServerSideApply(ctx, c, child, unregisteredOwner)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set controller reference")
	})

	t.Run("defensively strips managedFields to prevent API rejection", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm-with-managed-fields",
				Namespace: "default",
				ManagedFields: []metav1.ManagedFieldsEntry{
					{
						Manager:    "old-manager",
						Operation:  metav1.ManagedFieldsOperationApply,
						APIVersion: "v1",
					},
				},
			},
			Data: map[string]string{"foo": "bar"},
		}

		err := common.ServerSideApply(ctx, c, cm, nil)
		require.NoError(t, err)

		var retrieved corev1.ConfigMap
		err = c.Get(ctx, types.NamespacedName{Name: "cm-with-managed-fields", Namespace: "default"}, &retrieved)
		require.NoError(t, err)
		assert.Equal(t, "bar", retrieved.Data["foo"])
	})

	t.Run("updates existing object on subsequent apply", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "updated-cm",
				Namespace: "default",
			},
			Data: map[string]string{"k": "initial"},
		}

		err := common.ServerSideApply(ctx, c, cm, nil)
		require.NoError(t, err)

		// Subsequent apply with updated fields
		cm.Data["k"] = "updated"
		err = common.ServerSideApply(ctx, c, cm, nil)
		require.NoError(t, err)

		var retrieved corev1.ConfigMap
		err = c.Get(ctx, types.NamespacedName{Name: "updated-cm", Namespace: "default"}, &retrieved)
		require.NoError(t, err)
		assert.Equal(t, "updated", retrieved.Data["k"])
	})

	t.Run("fails when object type is not registered in scheme", func(t *testing.T) {
		scheme := runtime.NewScheme()
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		unregistered := &dummyUnregisteredObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unregistered-obj",
				Namespace: "default",
			},
		}

		err := common.ServerSideApply(ctx, c, unregistered, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to prepare object for server-side apply")
	})
}

func TestSetSSAManagedFieldsAnnotation(t *testing.T) {
	t.Run("sets sorted newline-separated paths", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					"keep": "me",
				},
			},
		}

		common.SetSSAManagedFieldsAnnotation(obj, []string{".spec.image", ".spec.serviceAccountName", ".metadata.labels['backup']"})

		got := obj.Annotations[addoncfg.SSAManagedFieldsAnnotationKey]
		assert.Equal(t, ".metadata.labels['backup']\n.spec.image\n.spec.serviceAccountName", got)
		assert.Equal(t, "me", obj.Annotations["keep"], "existing annotations must be preserved")
	})

	t.Run("creates annotations map when nil", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}

		common.SetSSAManagedFieldsAnnotation(obj, []string{".spec.image"})

		require.NotNil(t, obj.Annotations)
		assert.Equal(t, ".spec.image", obj.Annotations[addoncfg.SSAManagedFieldsAnnotationKey])
	})

	t.Run("no-op for empty fields", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}

		common.SetSSAManagedFieldsAnnotation(obj, nil)
		assert.Empty(t, obj.Annotations)
	})

	t.Run("typed nil object is a no-op", func(t *testing.T) {
		var u *unstructured.Unstructured
		assert.NotPanics(t, func() {
			common.SetSSAManagedFieldsAnnotation(u, []string{".spec.image"})
		})
	})
}

func TestSSAManagedFieldsAnnotation(t *testing.T) {
	assert.Empty(t, common.SSAManagedFieldsAnnotation(nil))
	var nilObj *corev1.ConfigMap
	assert.Empty(t, common.SSAManagedFieldsAnnotation(nilObj))

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				addoncfg.SSAManagedFieldsAnnotationKey: ".spec.image",
			},
		},
	}
	assert.Equal(t, ".spec.image", common.SSAManagedFieldsAnnotation(obj))
}

func TestDeriveSSAManagedFields(t *testing.T) {
	t.Run("lists spec keys and label keys from the apply payload", func(t *testing.T) {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":      "w",
				"namespace": "ns",
				"labels": map[string]any{
					"cluster.open-cluster-management.io/backup": "",
				},
				"annotations": map[string]any{
					"keep": "me",
				},
			},
			"spec": map[string]any{
				"image":              "foo",
				"serviceAccountName": "sa",
			},
			"status": map[string]any{
				"ready": true,
			},
		}}

		assert.Equal(t, []string{
			".metadata.labels['cluster.open-cluster-management.io/backup']",
			".spec.image",
			".spec.serviceAccountName",
		}, common.DeriveSSAManagedFields(obj))
	})

	t.Run("picks up newly added spec fields automatically", func(t *testing.T) {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{
				"image": "foo",
			},
		}}
		assert.Equal(t, []string{".spec.image"}, common.DeriveSSAManagedFields(obj))

		obj.Object["spec"].(map[string]any)["replicas"] = 1
		assert.Equal(t, []string{".spec.image", ".spec.replicas"}, common.DeriveSSAManagedFields(obj))
	})

	t.Run("nil object yields no paths", func(t *testing.T) {
		assert.Empty(t, common.DeriveSSAManagedFields(nil))
		var u *unstructured.Unstructured
		assert.Empty(t, common.DeriveSSAManagedFields(u))
	})
}

func TestSetSSAManagedFieldsAnnotationFromObject(t *testing.T) {
	t.Run("sets annotation on unstructured object", func(t *testing.T) {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{
				"image": "foo",
			},
		}}
		obj.SetName("w")

		common.SetSSAManagedFieldsAnnotationFromObject(obj)

		assert.Equal(t, ".spec.image", obj.GetAnnotations()[addoncfg.SSAManagedFieldsAnnotationKey])
	})

	t.Run("typed nil object is a no-op", func(t *testing.T) {
		var u *unstructured.Unstructured
		assert.NotPanics(t, func() {
			common.SetSSAManagedFieldsAnnotationFromObject(u)
		})
	})
}

func TestServerSideApply_PopulatesGVKWhenEmpty(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	var appliedData []byte
	var gvkDuringPatch schema.GroupVersionKind

	cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			gvkDuringPatch = obj.GetObjectKind().GroupVersionKind()
			data, err := patch.Data(obj)
			if err != nil {
				return err
			}
			appliedData = data
			return client.Patch(ctx, obj, patch, opts...)
		},
	}).Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}
	require.True(t, cm.GetObjectKind().GroupVersionKind().Empty())

	err := common.ServerSideApply(t.Context(), cl, cm, nil)
	require.NoError(t, err)

	assert.Equal(t, corev1.SchemeGroupVersion.WithKind("ConfigMap"), gvkDuringPatch)
	assert.Contains(t, string(appliedData), `"kind":"ConfigMap"`)
	assert.Contains(t, string(appliedData), `"apiVersion":"v1"`)
}
