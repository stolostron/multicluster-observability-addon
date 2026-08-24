package common_test

import (
	"testing"

	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
}

func TestSSAManagedFieldsAnnotation(t *testing.T) {
	assert.Empty(t, common.SSAManagedFieldsAnnotation(nil))

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
	})
}

func TestSetSSAManagedFieldsAnnotationFromObject(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"image": "foo",
		},
	}}
	obj.SetName("w")

	common.SetSSAManagedFieldsAnnotationFromObject(obj)

	assert.Equal(t, ".spec.image", obj.GetAnnotations()[addoncfg.SSAManagedFieldsAnnotationKey])
}
