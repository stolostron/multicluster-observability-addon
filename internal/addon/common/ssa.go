package common

import (
	"context"
	"fmt"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func ServerSideApply(ctx context.Context, c client.Client, obj client.Object, owner client.Object) error {
	if err := controllerutil.SetControllerReference(owner, obj, c.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := c.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(addon.Name)); err != nil {
		return fmt.Errorf("failed to patch with SSA: %w", err)
	}

	return nil
}
