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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errObjStorageSecretNotFound = errors.New("object storage secret referenced by LokiStack not found: create it before enabling the default logging stack")

func defaultStackLokiStackName() string {
	return fmt.Sprintf("%s-%s", addoncfg.DefaultStackPrefix, addoncfg.GlobalPlacementName)
}

func listManagedClusters(ctx context.Context, k8s client.Client) ([]clusterv1.ManagedCluster, error) {
	managedClusters := &clusterv1.ManagedClusterList{}
	if err := k8s.List(ctx, managedClusters, &client.ListOptions{}); err != nil {
		return nil, err
	}
	return managedClusters.Items, nil
}

// BuildDefaultStackStorageResources creates the hub storage component: a LokiStack
// template on the hub and a storage mTLS cert in the target cluster namespace.
func BuildDefaultStackStorageResources(ctx context.Context, k8s client.Client, platform, userWorkloads addon.LogsOptions, hubHostname string) ([]client.Object, error) {
	objects := []client.Object{}

	if !platform.DefaultStack {
		return objects, nil
	}

	clusters, err := listManagedClusters(ctx, k8s)
	if err != nil {
		return nil, err
	}
	tenants := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		tenants = append(tenants, cluster.Name)
	}
	targetCluster := common.HubClusterName(clusters)

	defaultOpts := manifests.BuildDefaultStackOptions(platform, userWorkloads, hubHostname)

	existingLS := &lokiv1.LokiStack{}
	resourceName := defaultStackLokiStackName()
	key := client.ObjectKey{Namespace: addoncfg.InstallNamespace, Name: resourceName}
	if err = k8s.Get(ctx, key, existingLS); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	defaultOpts.DefaultStack.Storage.LokiStack = existingLS
	defaultOpts.DefaultStack.Storage.Tenants = tenants

	ls, err := manifests.BuildSSALokiStack(defaultOpts, resourceName, addoncfg.GlobalPlacementNamespace, addoncfg.GlobalPlacementName)
	if err != nil {
		return nil, err
	}

	objStorageSecretKey := client.ObjectKey{Namespace: ls.Namespace, Name: ls.Spec.Storage.Secret.Name}
	if err = k8s.Get(ctx, objStorageSecretKey, &corev1.Secret{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("%w: %s/%s referenced by LokiStack %s/%s", errObjStorageSecretNotFound,
				objStorageSecretKey.Namespace, objStorageSecretKey.Name, ls.Namespace, resourceName)
		}
		return nil, fmt.Errorf("failed to check object storage secret %s/%s: %w", objStorageSecretKey.Namespace, objStorageSecretKey.Name, err)
	}

	objects = append(objects, ls)

	storageCerts, err := manifests.BuildSSAStorageCertificate(targetCluster)
	if err != nil {
		return nil, err
	}
	objects = append(objects, storageCerts...)

	return objects, nil
}

// BuildDefaultStackStorageClusterConfig returns the LokiStack addon config to attach
// to a single ManagedClusterAddOn (the hub today; another cluster later).
func BuildDefaultStackStorageClusterConfig(ctx context.Context, k8s client.Client, platform addon.LogsOptions) ([]common.ClusterAddonConfig, error) {
	if !platform.DefaultStack {
		return nil, nil
	}

	clusters, err := listManagedClusters(ctx, k8s)
	if err != nil {
		return nil, err
	}

	ls := &lokiv1.LokiStack{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LokiStack",
			APIVersion: lokiv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultStackLokiStackName(),
			Namespace: addoncfg.InstallNamespace,
		},
	}
	addonConfig, err := common.ObjectToAddonConfig(ls)
	if err != nil {
		return nil, err
	}

	return []common.ClusterAddonConfig{{
		ClusterNamespace: common.HubClusterName(clusters),
		Config:           addonConfig,
	}}, nil
}
