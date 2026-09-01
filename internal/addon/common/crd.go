package common

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var crdGVK = schema.GroupVersionKind{
	Group:   "apiextensions.k8s.io",
	Version: "v1",
	Kind:    "CustomResourceDefinition",
}

// IsCRDEstablished reports whether the CustomResourceDefinition with the given name exists on
// the cluster and has its "Established" condition set to "True". It is used to gate creation of
// custom resources belonging to operators that may not have finished installing yet (e.g. right
// after MCOA requests an OLM Subscription for an operator it manages).
//
// It deliberately uses an unstructured lookup rather than a typed client call so that callers
// don't need apiextensions/v1 registered in their scheme to use it (mirrors the existing
// MultiClusterHub lookup pattern in this package).
func IsCRDEstablished(ctx context.Context, k8s client.Client, name string) (bool, error) {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGVK)
	if err := k8s.Get(ctx, client.ObjectKey{Name: name}, crd); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get CustomResourceDefinition %s: %w", name, err)
	}

	conditions, found, err := unstructured.NestedSlice(crd.Object, "status", "conditions")
	if err != nil || !found {
		return false, nil
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] == "Established" && condition["status"] == "True" {
			return true, nil
		}
	}

	return false, nil
}
