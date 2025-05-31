package handlers

import (
	"context"
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildDefaultStackResources creates resources for the default logging stack
// based on the provided options and placement information.
func BuildDefaultStackResources(
	ctx context.Context,
	k8s client.Client,
	cmao *addonv1alpha1.ClusterManagementAddOn,
	platform, userWorkloads addon.LogsOptions,
	hubHostname string,
) ([]client.Object, []common.DefaultConfig, error) {
	objects := []client.Object{}
	defaultConfig := []common.DefaultConfig{}

	if !platform.DefaultStack {
		return objects, defaultConfig, nil
	}

	defaultOpts := manifests.BuildDefaultStackOptions(platform, userWorkloads, hubHostname)

	// Build ClusterLogForwarder for each placement
	for _, placement := range cmao.Spec.InstallStrategy.Placements {

		existingCLF := &loggingv1.ClusterLogForwarder{}
		resourceName := fmt.Sprintf("%s-%s", addon.DefaultStackPrefix, placement.Name)
		key := client.ObjectKey{Namespace: addon.InstallNamespace, Name: resourceName}
		if err := k8s.Get(ctx, key, existingCLF); err != nil && !apierrors.IsNotFound(err) {
			return nil, nil, err
		}

		defaultOpts.DefaultStack.Collection.ClusterLogForwarder = existingCLF
		clf, err := manifests.BuildSSAClusterLogForwarder(defaultOpts, resourceName, placement.Namespace, placement.Name)
		if err != nil {
			return nil, nil, err
		}
		objects = append(objects, clf)

		addonConfig, err := common.ObjectToAddonConfig(clf)
		if err != nil {
			return nil, nil, err
		}

		defaultConfig = append(defaultConfig, common.DefaultConfig{
			PlacementRef: placement.PlacementRef,
			Config:       addonConfig,
		})
	}

	// Build tenants for LokiStack
	// TODO(JoaoBraveCoding): In the future we might want to do this based on
	// placements and have separate LokiStacks for each placement
	// since this will require the hub to reconcile multiple LokiStacks we will
	// first focus on a single one
	managedClusters := &clusterv1.ManagedClusterList{}
	if err := k8s.List(ctx, managedClusters, &client.ListOptions{}); err != nil {
		return nil, nil, err
	}
	tenants := make([]string, 0, len(managedClusters.Items))
	for _, cluster := range managedClusters.Items {
		tenants = append(tenants, cluster.Name)
	}

	existingLS := &lokiv1.LokiStack{}
	resourceName := fmt.Sprintf("%s-%s", addon.DefaultStackPrefix, addon.GlobalPlacementName)
	key := client.ObjectKey{Namespace: addon.InstallNamespace, Name: resourceName}
	if err := k8s.Get(ctx, key, existingLS); err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}

	defaultOpts.DefaultStack.Storage.LokiStack = existingLS
	defaultOpts.DefaultStack.Storage.Tenants = tenants

	ls, err := manifests.BuildSSALokiStack(defaultOpts, resourceName, addon.GlobalPlacementNamespace, addon.GlobalPlacementName)
	if err != nil {
		return nil, nil, err
	}
	objects = append(objects, ls)

	addonConfig, err := common.ObjectToAddonConfig(ls)
	if err != nil {
		return nil, nil, err
	}

	defaultConfig = append(defaultConfig, common.DefaultConfig{
		PlacementRef: addon.GlobalPlacementRef,
		Config:       addonConfig,
	})

	// Build certiticate objects for each tenant + hub
	for _, tenant := range tenants {
		certObjs, err := manifests.BuildSSAClusterCertificates(tenant)
		if err != nil {
			return nil, nil, err
		}
		objects = append(objects, certObjs...)
	}

	return objects, defaultConfig, nil
}
