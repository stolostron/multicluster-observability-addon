package handlers

import (
	"context"
	"errors"
	"fmt"

	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errObjStorageSecretNotFound = errors.New("object storage secret referenced by LokiStack not found: create it before enabling the default logging stack")

// BuildDefaultStackStorageResources creates the hub storage component: a LokiStack
// template on the hub, a storage mTLS cert in the target cluster namespace, and an
// MCAO config pointing at that LokiStack. The config is attached to the hub MCAO
// today; the same pointer can later be moved to another cluster's MCAO.
func BuildDefaultStackStorageResources(ctx context.Context, k8s client.Client, platform, userWorkloads addon.LogsOptions, hubHostname string) ([]client.Object, []common.ClusterAddonConfig, error) {
	objects := []client.Object{}
	clusterConfig := []common.ClusterAddonConfig{}

	if !platform.DefaultStack {
		return objects, clusterConfig, nil
	}

	managedClusters := &clusterv1.ManagedClusterList{}
	if err := k8s.List(ctx, managedClusters, &client.ListOptions{}); err != nil {
		return nil, nil, err
	}
	tenants := make([]string, 0, len(managedClusters.Items))
	for _, cluster := range managedClusters.Items {
		tenants = append(tenants, cluster.Name)
	}
	targetCluster := common.HubClusterName(managedClusters.Items)

	defaultOpts := manifests.BuildDefaultStackOptions(platform, userWorkloads, hubHostname)

	existingLS := &lokiv1.LokiStack{}
	resourceName := fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName)
	key := client.ObjectKey{Namespace: addoncfg.InstallNamespace, Name: resourceName}
	if err := k8s.Get(ctx, key, existingLS); err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}

	defaultOpts.DefaultStack.Storage.LokiStack = existingLS
	defaultOpts.DefaultStack.Storage.Tenants = tenants

	ls, err := manifests.BuildSSALokiStack(defaultOpts, resourceName, addoncfg.GlobalPlacementNamespace, addoncfg.GlobalPlacementName)
	if err != nil {
		return nil, nil, err
	}

	objStorageSecretKey := client.ObjectKey{Namespace: ls.Namespace, Name: ls.Spec.Storage.Secret.Name}
	if err = k8s.Get(ctx, objStorageSecretKey, &corev1.Secret{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("%w: %s/%s referenced by LokiStack %s/%s", errObjStorageSecretNotFound,
				objStorageSecretKey.Namespace, objStorageSecretKey.Name, ls.Namespace, resourceName)
		}
		return nil, nil, fmt.Errorf("failed to check object storage secret %s/%s: %w", objStorageSecretKey.Namespace, objStorageSecretKey.Name, err)
	}

	objects = append(objects, ls)

	addonConfig, err := common.ObjectToAddonConfig(ls)
	if err != nil {
		return nil, nil, err
	}

	clusterConfig = append(clusterConfig, common.ClusterAddonConfig{
		ClusterNamespace: targetCluster,
		Config:           addonConfig,
	})

	storageCerts, err := manifests.BuildSSAStorageCertificate(targetCluster)
	if err != nil {
		return nil, nil, err
	}
	objects = append(objects, storageCerts...)

	return objects, clusterConfig, nil
}
