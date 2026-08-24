package common

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func ServerSideApply(ctx context.Context, c client.Client, obj client.Object, owner client.Object) error {
	// Only set controller reference if an owner is provided
	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, obj, c.Scheme()); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	//nolint:staticcheck // client.Apply is deprecated, but alternative requires ApplyConfigurations which we don't have
	if err := c.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(addoncfg.Name)); err != nil {
		return fmt.Errorf("failed to patch with SSA: %w", err)
	}

	return nil
}

// DeriveSSAManagedFields returns JSON paths for fields obj sets relative to a
// zero-value object of the same type. This picks up newly added spec fields
// automatically without a hand-maintained path list, and ignores zero-value
// noise from typed structs (empty slices, nested structs without omitempty).
func DeriveSSAManagedFields(obj client.Object) []string {
	if obj == nil {
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

	paths := []string{}
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

	paths := []string{}
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
	if obj == nil || len(fields) == 0 {
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
	SetSSAManagedFieldsAnnotation(obj, DeriveSSAManagedFields(obj))
}

// SSAManagedFieldsAnnotation returns the SSA managed-fields annotation value, or "" if unset.
func SSAManagedFieldsAnnotation(obj client.Object) string {
	if obj == nil {
		return ""
	}
	return obj.GetAnnotations()[addoncfg.SSAManagedFieldsAnnotationKey]
}
