package common

import (
	"context"
	"errors"
	"fmt"

	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	// errMissingResource is returned when a resource referenced in the ManagedClusterAddOn config is not found
	errMissingResource = errors.New("resource referenced in ManagedClusterAddOn config not found")

	// errMissingResourceRefs is returned when no references to the resource found in ManagedClusterAddOn config
	errMissingResourceRefs = errors.New("no references to the resource found in ManagedClusterAddOn config")

	// errMissingOwnerRef is returned when the reference exists but no resource owned MCOA was found
	errMissingOwnerRef = errors.New("no resource owned by MCOA found in references")
)

// GetResourceWithOwnerRef finds a resource from ManagedClusterAddOn config references that is owned by ClusterManagementAddOn
func GetResourceWithOwnerRef[T client.Object](
	ctx context.Context,
	k8s client.Client,
	mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	group, resource string,
	obj T,
) (T, error) {
	// Get resource references from ManagedClusterAddOn
	keys := GetObjectKeys(mcAddon.Status.ConfigReferences, group, resource)
	if len(keys) == 0 {
		return obj, fmt.Errorf("%w: %s/%s", errMissingResourceRefs, group, resource)
	}

	// ClusterManagementAddOn object to check owner ref against
	cmao := &addonapiv1alpha1.ClusterManagementAddOn{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterManagementAddOn",
			APIVersion: addonapiv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: addoncfg.Name,
		},
	}

	// Check each resource referenced in the addon to find the one owned by our controller
	for _, key := range keys {
		// Clone the object for each iteration to avoid mutating the input object
		tempObj := obj.DeepCopyObject().(T)
		if err := k8s.Get(ctx, key, tempObj, &client.GetOptions{}); err != nil {
			if k8serrors.IsNotFound(err) {
				return obj, fmt.Errorf("%w: %s/%s %s/%s", errMissingResource, group, resource, key.Namespace, key.Name)
			}
			return obj, err
		}

		hasOwnerRef, err := controllerutil.HasOwnerReference(tempObj.GetOwnerReferences(), cmao, k8s.Scheme())
		if err != nil {
			continue
		}

		if hasOwnerRef {
			obj = tempObj
			break
		}
	}

	// Verify we found an owned resource
	if obj.GetName() == "" {
		return obj, fmt.Errorf("%w: group=%s, resource=%s", errMissingOwnerRef, group, resource)
	}

	return obj, nil
}
