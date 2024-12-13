package helm

import (
	"context"
	"errors"

	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	lhandlers "github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	lmanifests "github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	thandlers "github.com/rhobs/multicluster-observability-addon/internal/tracing/handlers"
	tmanifests "github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const annotationLocalCluster = "local-cluster"

var (
	errMissingAODCRef  = errors.New("missing AddOnDeploymentConfig reference on addon installation")
	errMultipleAODCRef = errors.New("multiple AddOnDeploymentConfig references on addon installation")
)

type HelmChartValues struct {
	Enabled bool                     `json:"enabled"`
	Logging lmanifests.LoggingValues `json:"logging"`
	Tracing tmanifests.TracingValues `json:"tracing"`
}

func GetValuesFunc(ctx context.Context, k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		isHubCluster := isHubCluster(cluster)
		aodc, err := getAddOnDeploymentConfig(ctx, k8s, mcAddon)
		if err != nil {
			return nil, err
		}
		opts, err := addon.BuildOptions(aodc)
		if err != nil {
			return nil, err
		}

		if !opts.Platform.Enabled && !opts.UserWorkloads.Enabled {
			return addonfactory.JsonStructToValues(HelmChartValues{})
		}

		values := HelmChartValues{
			Enabled: true,
		}

		if opts.Platform.Logs.CollectionEnabled || opts.UserWorkloads.Logs.CollectionEnabled || opts.Platform.Logs.ManagedStack {
			loggingOpts, err := lhandlers.BuildOptions(ctx, k8s, mcAddon, opts.Platform.Logs, opts.UserWorkloads.Logs, isHubCluster, opts.HubHostname)
			if err != nil {
				return nil, err
			}

			logging, err := lmanifests.BuildValues(loggingOpts)
			if err != nil {
				return nil, err
			}
			values.Logging = *logging
		}

		if !isHubCluster && opts.UserWorkloads.Traces.CollectionEnabled {
			tracingOpts, err := thandlers.BuildOptions(ctx, k8s, mcAddon, opts.UserWorkloads.Traces)
			if err != nil {
				return nil, err
			}

			tracing, err := tmanifests.BuildValues(tracingOpts)
			if err != nil {
				return nil, err
			}
			values.Tracing = tracing
		}

		return addonfactory.JsonStructToValues(values)
	}
}

func getAddOnDeploymentConfig(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) (*addonapiv1alpha1.AddOnDeploymentConfig, error) {
	aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
	keys := addon.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addon.AddonDeploymentConfigResource)
	switch {
	case len(keys) == 0:
		return aodc, errMissingAODCRef
	case len(keys) > 1:
		return aodc, errMultipleAODCRef
	}
	if err := k8s.Get(ctx, keys[0], aodc, &client.GetOptions{}); err != nil {
		// TODO(JoaoBraveCoding) Add proper error handling
		return aodc, err
	}
	return aodc, nil
}

func isHubCluster(cluster *clusterv1.ManagedCluster) bool {
	val, ok := cluster.Labels[annotationLocalCluster]
	if !ok {
		return false
	}
	return val == "true"
}
