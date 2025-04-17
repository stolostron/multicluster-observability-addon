package handlers

import (
	"context"
	"fmt"

	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	lmanifests "github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildDefaultStackResources creates resources for the default logging stack
// based on the provided options and placement information.
func BuildDefaultStackResources(
	ctx context.Context,
	c client.Client,
	cmao *addonv1alpha1.ClusterManagementAddOn,
	platform, userWorkloads addon.LogsOptions,
	hubHostname string,
) ([]client.Object, error) {
	objects := []client.Object{}

	if !platform.DefaultStack {
		return objects, nil
	}

	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		resourceName := fmt.Sprintf("%s-%s", addon.DefaultStackPrefix, placement.Name)

		loggingOpts, err := DefaultStackOptions(ctx, c, platform, userWorkloads, hubHostname, resourceName)
		if err != nil {
			return nil, err
		}

		clf, err := lmanifests.BuildSSAClusterLogForwarder(loggingOpts, resourceName)
		if err != nil {
			return nil, err
		}

		objects = append(objects, clf)

		ls, err := lmanifests.BuildSSALokiStack(loggingOpts, resourceName)
		if err != nil {
			return nil, err
		}
		objects = append(objects, ls)
	}

	return objects, nil
}
