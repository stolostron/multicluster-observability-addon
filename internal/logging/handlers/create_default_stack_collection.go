package handlers

import (
	"context"
	"fmt"

	loggingv1 "github.com/openshift/cluster-logging-operator/api/observability/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildDefaultStackCollectionResources creates spoke collection templates:
// ClusterLogForwarders attached to CMAO placements, plus per-cluster mTLS certs.
func BuildDefaultStackCollectionResources(ctx context.Context, k8s client.Client, cmao *addonv1beta1.ClusterManagementAddOn, platform, userWorkloads addon.LogsOptions, hubHostname string) ([]client.Object, []common.DefaultConfig, error) {
	objects := []client.Object{}
	defaultConfig := []common.DefaultConfig{}

	if !platform.DefaultStack {
		return objects, defaultConfig, nil
	}

	defaultOpts := manifests.BuildDefaultStackOptions(platform, userWorkloads, hubHostname)

	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		existingCLF := &loggingv1.ClusterLogForwarder{}
		resourceName := fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, placement.Name)
		key := client.ObjectKey{Namespace: addoncfg.InstallNamespace, Name: resourceName}
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

	managedClusters := &clusterv1.ManagedClusterList{}
	if err := k8s.List(ctx, managedClusters, &client.ListOptions{}); err != nil {
		return nil, nil, err
	}
	for _, cluster := range managedClusters.Items {
		certObjs, err := manifests.BuildSSACollectionCertificates(cluster.Name)
		if err != nil {
			return nil, nil, err
		}
		objects = append(objects, certObjs...)
	}

	return objects, defaultConfig, nil
}
