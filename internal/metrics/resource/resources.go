package resource

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var initialized bool

func DeployDefaultResourcesOnce(ctx context.Context, c client.Client, logger logr.Logger, ns string) error {
	if initialized {
		return nil
	}

	logger.Info("deploying default monitoring resources")

	// Deploy default resources
	resources := DefaultPlaftformAgentResources(ns)
	resources = append(resources, DefaultUserWorkloadAgentResources(ns)...)
	if err := common.CreateOrUpdateWithAddOnOwner(ctx, logger, c, resources); err != nil {
		return fmt.Errorf("failed to deploy default monitoring resources: %w", err)
	}

	initialized = true

	return nil
}
