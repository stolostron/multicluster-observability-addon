package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ErrNilObject is returned when attempting to server-side apply a nil object.
var ErrNilObject = errors.New("cannot server-side apply a nil object")

func isNil(obj client.Object) bool {
	if obj == nil {
		return true
	}
	val := reflect.ValueOf(obj)
	return val.Kind() == reflect.Pointer && val.IsNil()
}

// ServerSideApply applies the provided object using Kubernetes Server-Side Apply (SSA).
// If owner is non-nil, a controller reference is established before applying.
func ServerSideApply(ctx context.Context, c client.Client, obj client.Object, owner client.Object) error {
	if isNil(obj) {
		return ErrNilObject
	}

	// Only set controller reference if an owner is provided
	if !isNil(owner) {
		if err := controllerutil.SetControllerReference(owner, obj, c.Scheme()); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	u, err := toUnstructuredApply(c, obj)
	if err != nil {
		return fmt.Errorf("failed to prepare object for server-side apply: %w", err)
	}

	applyConfig := client.ApplyConfigurationFromUnstructured(u)
	if err := c.Apply(ctx, applyConfig, client.FieldOwner(addoncfg.Name), client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply with SSA: %w", err)
	}

	if err := syncBackToObject(u, obj); err != nil {
		return fmt.Errorf("failed to sync server response back to object: %w", err)
	}
	return nil
}

func toUnstructuredApply(c client.Client, obj client.Object) (*unstructured.Unstructured, error) {
	var u *unstructured.Unstructured
	if unstr, ok := obj.(*unstructured.Unstructured); ok {
		u = unstr.DeepCopy()
	} else {
		unstrMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %T to unstructured map: %w", obj, err)
		}
		u = &unstructured.Unstructured{Object: unstrMap}
	}

	if u.GetAPIVersion() == "" || u.GetKind() == "" {
		gvk, err := c.GroupVersionKindFor(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to determine GVK for %T: %w", obj, err)
		}
		u.SetGroupVersionKind(gvk)
	}

	// Defensive stripping of managedFields to avoid API server rejection
	u.SetManagedFields(nil)
	return u, nil
}

func syncBackToObject(u *unstructured.Unstructured, obj client.Object) error {
	if unstr, ok := obj.(*unstructured.Unstructured); ok {
		*unstr = *u
		return nil
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return fmt.Errorf("failed to sync server response to %T: %w", obj, err)
	}

	if gvk := u.GroupVersionKind(); !gvk.Empty() {
		obj.GetObjectKind().SetGroupVersionKind(gvk)
	}
	return nil
}

// DeriveSSAManagedFields returns JSON paths for fields obj sets relative to a
// zero-value object of the same type. This picks up newly added spec fields
// automatically without a hand-maintained path list, and ignores zero-value
// noise from typed structs (empty slices, nested structs without omitempty).
func DeriveSSAManagedFields(obj client.Object) []string {
	if isNil(obj) {
		return nil
	}

	objType := reflect.TypeOf(obj)
	if objType.Kind() != reflect.Pointer {
		return nil
	}

	// Build a zero-value object of the same type as obj.
	zero, ok := reflect.New(objType.Elem()).Interface().(client.Object)
	if !ok {
		return nil
	}

	objPayload := marshalToMap(obj)
	zeroPayload := marshalToMap(zero)

	// Get the JSON paths of the spec fields as well as the labels that are different.
	paths := specKeyDiff(zeroPayload["spec"], objPayload["spec"])
	paths = append(paths, labelKeyDiff(zeroPayload, objPayload)...)
	slices.Sort(paths)
	return paths
}

// marshalToMap marshals to JSON and unmarshals to a map[string]any.
// This is to get proper JSON paths and drop omitempty.
func marshalToMap(obj client.Object) map[string]any {
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

// specKeyDiff compares the spec of two objects and returns the JSON paths of the top-level fields that are different.
func specKeyDiff(zeroSpec, objSpec any) []string {
	objMap, _ := objSpec.(map[string]any)
	if len(objMap) == 0 {
		return nil
	}
	zeroMap, _ := zeroSpec.(map[string]any)
	if zeroMap == nil {
		zeroMap = map[string]any{}
	}

	paths := make([]string, 0, len(objMap))
	for key, val := range objMap {
		zeroVal, exists := zeroMap[key]
		if !exists || !reflect.DeepEqual(zeroVal, val) {
			paths = append(paths, ".spec."+key)
		}
	}
	return paths
}

// labelKeyDiff compares the labels of two objects and returns the labels that are different.
func labelKeyDiff(zeroPayload, objPayload map[string]any) []string {
	objLabels := labelsFromPayload(objPayload)
	if len(objLabels) == 0 {
		return nil
	}
	zeroLabels := labelsFromPayload(zeroPayload)

	paths := make([]string, 0, len(objLabels))
	for key, val := range objLabels {
		zeroVal, exists := zeroLabels[key]
		if !exists || !reflect.DeepEqual(zeroVal, val) {
			paths = append(paths, fmt.Sprintf(".metadata.labels['%s']", key))
		}
	}
	return paths
}

func labelsFromPayload(payload map[string]any) map[string]any {
	metadata, _ := payload["metadata"].(map[string]any)
	labels, _ := metadata["labels"].(map[string]any)
	return labels
}

// SetSSAManagedFieldsAnnotation records the JSON paths MCOA enforces with server-side apply.
// The annotation is sorted for a stable value so reconciles are not retriggered by field-order churn.
func SetSSAManagedFieldsAnnotation(obj client.Object, fields []string) {
	if isNil(obj) || len(fields) == 0 {
		return
	}

	sorted := slices.Clone(fields)
	slices.Sort(sorted)

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[addoncfg.SSAManagedFieldsAnnotationKey] = strings.Join(sorted, "\n")
	obj.SetAnnotations(annotations)
}

// SetSSAManagedFieldsAnnotationFromObject derives managed-field paths from obj and
// writes them onto obj. obj should contain only the fields MCOA is applying.
func SetSSAManagedFieldsAnnotationFromObject(obj client.Object) {
	if isNil(obj) {
		return
	}
	SetSSAManagedFieldsAnnotation(obj, DeriveSSAManagedFields(obj))
}

// SSAManagedFieldsAnnotation returns the SSA managed-fields annotation value, or "" if unset.
func SSAManagedFieldsAnnotation(obj client.Object) string {
	if isNil(obj) {
		return ""
	}
	return obj.GetAnnotations()[addoncfg.SSAManagedFieldsAnnotationKey]
}
