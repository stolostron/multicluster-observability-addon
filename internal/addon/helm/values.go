package helm

import (
	"context"
	"errors"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	lhandlers "github.com/rhobs/multicluster-observability-addon/internal/logging/handlers"
	lmanifests "github.com/rhobs/multicluster-observability-addon/internal/logging/manifests"
	mconfig "github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	mhandlers "github.com/rhobs/multicluster-observability-addon/internal/metrics/handlers"
	mmanifests "github.com/rhobs/multicluster-observability-addon/internal/metrics/manifests"
	mresource "github.com/rhobs/multicluster-observability-addon/internal/metrics/resource"
	thandlers "github.com/rhobs/multicluster-observability-addon/internal/tracing/handlers"
	tmanifests "github.com/rhobs/multicluster-observability-addon/internal/tracing/manifests"
	clusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	clusterlifecycleconstants "github.com/stolostron/cluster-lifecycle-api/constants"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errMissingAODCRef     = errors.New("missing AddOnDeploymentConfig reference on addon installation")
	errMultipleAODCRef    = errors.New("multiple AddOnDeploymentConfig references on addon installation")
	errMissingHubEndpoint = errors.New("platform hub endpoint is required for metrics collection")
)

type HelmChartValues struct {
	Enabled bool                     `json:"enabled"`
	Metrics mmanifests.MetricsValues `json:"metrics"`
	Logging lmanifests.LoggingValues `json:"logging"`
	Tracing tmanifests.TracingValues `json:"tracing"`
}

func GetValuesFunc(ctx context.Context, k8s client.Client, logger logr.Logger) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		// if hub cluster, then don't install anything.
		// some kube flavors are also currently not supported
		if isHubCluster(cluster) || !supportedKubeVendors(cluster) {
			return addonfactory.JsonStructToValues(HelmChartValues{})
		}

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

		userValues := HelmChartValues{
			Enabled: true,
		}

		if opts.Platform.Metrics.CollectionEnabled || opts.UserWorkloads.Metrics.CollectionEnabled {
			if opts.Platform.Metrics.HubEndpoint == "" {
				return nil, errMissingHubEndpoint
			}

			if err := mresource.DeployDefaultResourcesOnce(ctx, k8s, logger, mconfig.HubInstallNamespace); err != nil {
				return nil, err
			}

			remoteWriteURL, err := url.JoinPath(opts.Platform.Metrics.HubEndpoint, "/api/metrics/v1/default/api/v1/receive")
			if err != nil {
				return nil, err
			}
			optsBuilder := mhandlers.OptionsBuilder{
				Client:          k8s,
				ImagesConfigMap: mconfig.ImagesConfigMap,
				RemoteWriteURL:  remoteWriteURL,
				Logger:          logger,
			}
			metricsOpts, err := optsBuilder.Build(ctx, mcAddon, cluster, opts.Platform.Metrics, opts.UserWorkloads.Metrics)
			if err != nil {
				return nil, err
			}

			metrics, err := mmanifests.BuildValues(metricsOpts)
			if err != nil {
				return nil, err
			}
			userValues.Metrics = metrics
		}

		if opts.Platform.Logs.CollectionEnabled || opts.UserWorkloads.Logs.CollectionEnabled {
			loggingOpts, err := lhandlers.BuildOptions(ctx, k8s, mcAddon, opts.Platform.Logs, opts.UserWorkloads.Logs)
			if err != nil {
				return nil, err
			}

			logging, err := lmanifests.BuildValues(loggingOpts)
			if err != nil {
				return nil, err
			}
			userValues.Logging = *logging
		}

		if opts.UserWorkloads.Traces.CollectionEnabled {
			tracingOpts, err := thandlers.BuildOptions(ctx, k8s, mcAddon, opts.UserWorkloads.Traces)
			if err != nil {
				return nil, err
			}

			tracing, err := tmanifests.BuildValues(tracingOpts)
			if err != nil {
				return nil, err
			}
			userValues.Tracing = tracing
		}

		return addonfactory.JsonStructToValues(userValues)
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
	val, ok := cluster.Labels[clusterlifecycleconstants.SelfManagedClusterLabelKey]
	if !ok {
		return false
	}
	return val == "true"
}

func supportedKubeVendors(cluster *clusterv1.ManagedCluster) bool {
	val, ok := cluster.Labels[clusterinfov1beta1.LabelKubeVendor]
	if !ok {
		return false
	}
	return val == string(clusterinfov1beta1.KubeVendorOpenShift)
}
