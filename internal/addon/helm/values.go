package helm

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	rshandlers "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/handlers"
	chandlers "github.com/stolostron/multicluster-observability-addon/internal/coo/handlers"
	cmanifests "github.com/stolostron/multicluster-observability-addon/internal/coo/manifests"
	lhandlers "github.com/stolostron/multicluster-observability-addon/internal/logging/handlers"
	lmanifests "github.com/stolostron/multicluster-observability-addon/internal/logging/manifests"
	mhandlers "github.com/stolostron/multicluster-observability-addon/internal/metrics/handlers"
	mmanifests "github.com/stolostron/multicluster-observability-addon/internal/metrics/manifests"
	omanifests "github.com/stolostron/multicluster-observability-addon/internal/obsapi/manifests"
	thandlers "github.com/stolostron/multicluster-observability-addon/internal/tracing/handlers"
	tmanifests "github.com/stolostron/multicluster-observability-addon/internal/tracing/manifests"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HelmChartValues struct {
	Enabled     bool                          `json:"enabled"`
	Metrics     *mmanifests.MetricsValues     `json:"metrics,omitempty"`
	Logging     *lmanifests.LoggingValues     `json:"logging,omitempty"`
	Tracing     *tmanifests.TracingValues     `json:"tracing,omitempty"`
	COO         *cmanifests.COOValues         `json:"coo,omitempty"`
	RightSizing *rshandlers.RightSizingValues `json:"rightSizing,omitempty"`
	ObsAPI      *omanifests.ObsAPIValues      `json:"obs-api,omitempty"`
}

func GetValuesFunc(ctx context.Context, k8s client.Client, logger logr.Logger) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		mcAddon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		logger = logger.WithValues("cluster", cluster.Name)
		logger.V(2).Info("reconciliation triggered")

		aodc, err := common.GetAddOnDeploymentConfig(ctx, k8s, mcAddon)
		if err != nil {
			return nil, fmt.Errorf("failed to get AddOnDeploymentConfig: %w", err)
		}
		opts, err := addon.BuildOptions(aodc)
		if err != nil {
			return nil, fmt.Errorf("failed to build addon options: %w", err)
		}

		if !opts.Platform.Enabled && !opts.UserWorkloads.Enabled {
			logger.V(2).Info("both platform and userWorkloads are disabled, ignoring cluster")
			return addonfactory.JsonStructToValues(HelmChartValues{})
		}

		userValues := HelmChartValues{
			Enabled: true,
		}

		// Build right-sizing options first (needed for ScrapeConfig merging into metrics)
		var rsOpts *rshandlers.Options
		rsOptsBuilder := rshandlers.OptionsBuilder{
			Client: k8s,
			Logger: logger,
		}
		rsOptsBuilt, err := rsOptsBuilder.Build(ctx, cluster, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to build right-sizing options: %w", err)
		}
		if rsOptsBuilt.NamespaceRightSizing.Enabled || rsOptsBuilt.VirtualizationRightSizing.Enabled {
			rsOpts = &rsOptsBuilt
		}

		userValues.Metrics, err = getMonitoringValues(ctx, k8s, logger, cluster, mcAddon, opts, rsOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to get monitoring values: %w", err)
		}

		userValues.Logging, err = getLoggingValues(ctx, k8s, cluster, mcAddon, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get logging values: %w", err)
		}

		userValues.Tracing, err = getTracingValues(ctx, k8s, cluster, mcAddon, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get tracing values: %w", err)
		}

		userValues.COO, err = getCOOValues(ctx, k8s, logger, cluster, opts)
		if err != nil {
			return nil, err
		}

		// Use already-built right-sizing options for values
		userValues.RightSizing, err = getRightSizingValuesFromOpts(rsOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to get right-sizing values: %w", err)
		}

		// WIP: Temporary solution to enable obs-api and will require to delete the mcoa pod to take effect.
		obsAPIEnabled := aodc.Annotations["mcoa-obs-api"] == "true"
		userValues.ObsAPI = omanifests.BuildValues(common.IsHubCluster(cluster), obsAPIEnabled)

		return addonfactory.JsonStructToValues(userValues)
	}
}

func getMonitoringValues(ctx context.Context, k8s client.Client, logger logr.Logger, cluster *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts addon.Options, rsOpts *rshandlers.Options) (*mmanifests.MetricsValues, error) {
	if !opts.Platform.Metrics.CollectionEnabled && !opts.UserWorkloads.Metrics.CollectionEnabled {
		logger.V(2).Info("both platform and userWorkloads metrics are disabled, ignoring cluster")
		return nil, nil
	}

	optsBuilder := mhandlers.OptionsBuilder{
		Client: k8s,
		Logger: logger,
	}
	metricsOpts, err := optsBuilder.Build(ctx, mcAddon, cluster, opts)
	if err != nil {
		return nil, err
	}

	// Merge right-sizing ScrapeConfigs into platform ScrapeConfigs
	if rsOpts != nil {
		if len(rsOpts.NamespaceRightSizing.ScrapeConfigs) > 0 {
			metricsOpts.Platform.ScrapeConfigs = append(metricsOpts.Platform.ScrapeConfigs, rsOpts.NamespaceRightSizing.ScrapeConfigs...)
			logger.V(2).Info("Merged namespace right-sizing ScrapeConfigs into platform",
				"count", len(rsOpts.NamespaceRightSizing.ScrapeConfigs))
		}
		if len(rsOpts.VirtualizationRightSizing.ScrapeConfigs) > 0 {
			metricsOpts.Platform.ScrapeConfigs = append(metricsOpts.Platform.ScrapeConfigs, rsOpts.VirtualizationRightSizing.ScrapeConfigs...)
			logger.V(2).Info("Merged virtualization right-sizing ScrapeConfigs into platform",
				"count", len(rsOpts.VirtualizationRightSizing.ScrapeConfigs))
		}
	}

	return mmanifests.BuildValues(metricsOpts)
}

func getLoggingValues(ctx context.Context, k8s client.Client, cluster *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts addon.Options) (*lmanifests.LoggingValues, error) {
	if !opts.Platform.Logs.CollectionEnabled && !opts.UserWorkloads.Logs.CollectionEnabled {
		return nil, nil
	}

	if !common.IsOpenShiftVendor(cluster) {
		return nil, nil
	}

	loggingOpts, err := lhandlers.BuildOptions(ctx, k8s, mcAddon, opts.Platform.Logs, opts.UserWorkloads.Logs, common.IsHubCluster(cluster))
	if err != nil {
		return nil, err
	}

	return lmanifests.BuildValues(loggingOpts)
}

func getTracingValues(ctx context.Context, k8s client.Client, cluster *clusterv1.ManagedCluster, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, opts addon.Options) (*tmanifests.TracingValues, error) {
	if common.IsHubCluster(cluster) || !opts.UserWorkloads.Traces.CollectionEnabled {
		return nil, nil
	}

	if !common.IsOpenShiftVendor(cluster) {
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
	if !common.IsOpenShiftVendor(cluster) {
		return nil, nil
	}

	installCOO, err := chandlers.InstallOfCOOOnTheHubIsNeeded(ctx, k8s, logger, common.IsHubCluster(cluster))
	if err != nil {
		return nil, err
	}

	return cmanifests.BuildValues(opts, installCOO, common.IsHubCluster(cluster)), nil
}

// getRightSizingValuesFromOpts converts already-built right-sizing options to helm values.
// This is used to avoid rebuilding the options twice (once for ScrapeConfig merging, once for values).
func getRightSizingValuesFromOpts(rsOpts *rshandlers.Options) (*rshandlers.RightSizingValues, error) {
	if rsOpts == nil {
		return nil, nil
	}

	return rshandlers.BuildValues(*rsOpts)
}
