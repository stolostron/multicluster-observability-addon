package helm

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	clusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	clusterlifecycleconstants "github.com/stolostron/cluster-lifecycle-api/constants"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	chandlers "github.com/stolostron/multicluster-observability-addon/internal/coo/handlers"
	cmanifests "github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	lhandlers "github.com/stolostron/multicluster-observability-addon/internal/logging/handlers"
	lmanifests "github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	mhandlers "github.com/stolostron/multicluster-observability-addon/internal/metrics/handlers"
	mmanifests "github.com/stolostron/multicluster-observability-addon/internal/metrics/manifests"
	thandlers "github.com/stolostron/multicluster-observability-addon/internal/tracing/handlers"
	tmanifests "github.com/stolostron/multicluster-observability-addon/internal/tracing/manifests"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	addonutils "open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errMissingAODCRef     = errors.New("missing required AddOnDeploymentConfig reference in addon configuration")
	errMultipleAODCRef    = errors.New("addonmultiple AddOnDeploymentConfig references found - only one is supported")
	errMissingHubEndpoint = errors.New("metricsHubHostname key is missing but it's required when either platformMetricsCollection or userWorkloadMetricsCollection are present")
)

type HelmChartValues struct {
	Enabled bool                      `json:"enabled"`
	Metrics *mmanifests.MetricsValues `json:"metrics,omitempty"`
	Logging *lmanifests.LoggingValues `json:"logging,omitempty"`
	Tracing *tmanifests.TracingValues `json:"tracing,omitempty"`
	COO     *cmanifests.COOValues     `json:"coo,omitempty"`
}

func GetValuesFunc(ctx context.Context, k8s client.Client, logger logr.Logger) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		logger.V(2).Info("reconciliation triggered", "cluster", cluster.Name)
		// if hub cluster, then don't install anything.
		// some kube flavors are also currently not supported
		if !supportedKubeVendors(cluster) {
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

		userValues.Metrics, err = getMonitoringValues(ctx, k8s, logger, cluster, mcAddon, opts)
		if err != nil {
			return nil, err
		}

		userValues.Logging, err = getLoggingValues(ctx, k8s, cluster, mcAddon, opts)
		if err != nil {
			return nil, err
		}

		userValues.Tracing, err = getTracingValues(ctx, k8s, cluster, mcAddon, opts)
		if err != nil {
			return nil, err
		}

		userValues.COO, err = getCOOValues(ctx, k8s, logger, cluster, opts)
		if err != nil {
			return nil, err
		}

		return addonfactory.JsonStructToValues(userValues)
	}
}

func getMonitoringValues(ctx context.Context, k8s client.Client, logger logr.Logger, cluster *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts addon.Options) (*mmanifests.MetricsValues, error) {
	if !opts.Platform.Metrics.CollectionEnabled && !opts.UserWorkloads.Metrics.CollectionEnabled {
		return nil, nil
	}

	if opts.Platform.Metrics.HubEndpoint == nil || opts.Platform.Metrics.HubEndpoint.Host == "" {
		return nil, errMissingHubEndpoint
	}

	optsBuilder := mhandlers.OptionsBuilder{
		Client:         k8s,
		RemoteWriteURL: opts.Platform.Metrics.HubEndpoint.String(),
		Logger:         logger,
	}
	metricsOpts, err := optsBuilder.Build(ctx, mcAddon, cluster, opts.Platform.Metrics, opts.UserWorkloads.Metrics)
	if err != nil {
		return nil, err
	}

	return mmanifests.BuildValues(metricsOpts)
}

func getLoggingValues(ctx context.Context, k8s client.Client, cluster *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts addon.Options) (*lmanifests.LoggingValues, error) {
	if !opts.Platform.Logs.CollectionEnabled && !opts.UserWorkloads.Logs.CollectionEnabled {
		return nil, nil
	}

	loggingOpts, err := lhandlers.BuildOptions(ctx, k8s, mcAddon, opts.Platform.Logs, opts.UserWorkloads.Logs, isHubCluster(cluster))
	if err != nil {
		return nil, err
	}

	return lmanifests.BuildValues(loggingOpts)
}

func getTracingValues(ctx context.Context, k8s client.Client, cluster *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts addon.Options) (*tmanifests.TracingValues, error) {
	if isHubCluster(cluster) || !opts.UserWorkloads.Traces.CollectionEnabled {
		return nil, nil
	}

	tracingOpts, err := thandlers.BuildOptions(ctx, k8s, mcAddon, opts.UserWorkloads.Traces)
	if err != nil {
		return nil, err
	}

	tracing, err := tmanifests.BuildValues(tracingOpts)
	if err != nil {
		return nil, err
	}

	return &tracing, nil
}

func getCOOValues(ctx context.Context, k8s client.Client, logger logr.Logger, cluster *clusterv1.ManagedCluster, opts addon.Options) (*cmanifests.COOValues, error) {
	installCOO, err := chandlers.InstallCOO(ctx, k8s, logger, isHubCluster(cluster))
	if err != nil {
		return nil, err
	}

	return cmanifests.BuildValues(opts, installCOO, isHubCluster(cluster)), nil
}

func getAddOnDeploymentConfig(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) (*addonapiv1alpha1.AddOnDeploymentConfig, error) {
	aodc := &addonapiv1alpha1.AddOnDeploymentConfig{}
	keys := common.GetObjectKeys(mcAddon.Status.ConfigReferences, addonutils.AddOnDeploymentConfigGVR.Group, addon.AddonDeploymentConfigResource)
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
	return cluster.Labels[clusterlifecycleconstants.SelfManagedClusterLabelKey] == "true"
}

func supportedKubeVendors(cluster *clusterv1.ManagedCluster) bool {
	return cluster.Labels[clusterinfov1beta1.LabelKubeVendor] == string(clusterinfov1beta1.KubeVendorOpenShift)
}
