package resource

import (
	"context"
	"fmt"
	"sync"

	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	mu          sync.Mutex
	initialized bool
	ErrNotOwner = fmt.Errorf("controller is not the owner")
)

func DeployDefaultResourcesOnce(ctx context.Context, c client.Client, ns string) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return nil
	}

	// Get clusterManagementAddon resource to use as owner
	owner := &addonapiv1alpha1.ClusterManagementAddOn{}
	if err := c.Get(ctx, types.NamespacedName{Name: addon.Name, Namespace: ns}, owner); err != nil {
		return err
	}

	// Deploy default resources
	resources := DefaultPlaftformAgentResources(ns)
	for _, resource := range resources {
		if err := CreateOrUpdateResource(ctx, c, resource, owner); err != nil {
			return fmt.Errorf("failed to create or update resource %s: %w", resource.GetName(), err)
		}
	}

	return nil
}

func CreateOrUpdateResource(ctx context.Context, c client.Client, newResource, owner client.Object) error {
	if err := controllerutil.SetControllerReference(owner, newResource, c.Scheme()); err != nil {
		return err
	}

	if client.IgnoreAlreadyExists(c.Create(ctx, newResource)) != nil {
		return nil
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingResource := newResource.DeepCopyObject().(client.Object)
		if err := c.Get(ctx, types.NamespacedName{Namespace: newResource.GetNamespace(), Name: newResource.GetName()}, existingResource); err != nil {
			return err
		}

		// Check if this controller is the owner
		isOwner := false
		for _, ownerRef := range existingResource.GetOwnerReferences() {
			if ownerRef.UID == owner.GetUID() {
				isOwner = true
				break
			}
		}

		if !isOwner {
			return ErrNotOwner
		}

		// Overwrite the resource
		newResource.SetResourceVersion(existingResource.GetResourceVersion())

		if err := c.Update(ctx, existingResource); err != nil {
			return err
		}

		return nil
	})

	if retryErr != nil {
		return retryErr
	}

	return nil
}
