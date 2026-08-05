package common

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mchGroup   = "operator.open-cluster-management.io"
	mchVersion = "v1"
	mchKind    = "MultiClusterHub"
)

var MchGVK = schema.GroupVersionKind{
	Group:   mchGroup,
	Version: mchVersion,
	Kind:    mchKind,
}

func NewMultiClusterHub() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(MchGVK)
	return u
}

// IsNetworkPoliciesEnabled reports whether MCH has spec.networkPolicies.enabled set.
// Future change this to use multiclusterhub apiv1
func IsNetworkPoliciesEnabled(u *unstructured.Unstructured) bool {
	if u == nil {
		return false
	}
	enabled, found, err := unstructured.NestedBool(u.Object, "spec", "networkPolicies", "enabled")
	if err != nil || !found {
		return false
	}
	return enabled
}

// lists MultiClusterHub and returns whether network policies are enabled.
func GetNetworkPoliciesEnabled(ctx context.Context, c client.Client) (bool, error) {
	mchList := &unstructured.UnstructuredList{}
	mchList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   mchGroup,
		Version: mchVersion,
		Kind:    mchKind,
	})
	if err := c.List(ctx, mchList); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to list MultiClusterHub: %w", err)
	}
	if len(mchList.Items) == 0 {
		return false, nil
	}
	return IsNetworkPoliciesEnabled(&mchList.Items[0]), nil
}
