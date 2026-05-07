// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package handlers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	rsnamespace "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/namespace"
	rsvirtualization "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/virtualization"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// MCOAClusterManagementAddOnName is the name of the MCOA ClusterManagementAddOn
	MCOAClusterManagementAddOnName = "multicluster-observability-addon"
)

// OptionsBuilder builds right-sizing options for the helm chart
type OptionsBuilder struct {
	Client client.Client
	Logger logr.Logger
}

// Build builds the right-sizing options based on the addon options and cluster
func (o *OptionsBuilder) Build(ctx context.Context, cluster *clusterv1.ManagedCluster, opts addon.Options) (Options, error) {
	ret := Options{}

	// Skip if platform is not enabled or analytics options are not set
	if !opts.Platform.Enabled {
		return ret, nil
	}

	// Check if this is an OpenShift cluster - right-sizing only works on OpenShift
	if !common.IsOpenShiftVendor(cluster) {
		o.Logger.V(2).Info("Skipping right-sizing for non-OpenShift cluster", "cluster", cluster.Name)
		return ret, nil
	}

	namespaceEnabled := opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled
	virtualizationEnabled := opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled

	// Build namespace right-sizing options
	if namespaceEnabled {
		// Ensure ConfigMap exists on hub (MCOA owns all RS resources)
		if err := o.ensureNamespaceConfigMap(ctx); err != nil {
			o.Logger.Error(err, "Failed to ensure namespace ConfigMap exists, continuing with defaults")
		}

		nsConfigData, err := o.getConfigData(ctx, rightsizing.NamespaceConfigMapName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				nsConfigData = rightsizing.RSConfigMapData{
					PrometheusRuleConfig:   rightsizing.GetDefaultRSPrometheusRuleConfig(),
					PlacementConfiguration: rightsizing.GetDefaultRSPlacement(),
				}
			} else {
				return ret, fmt.Errorf("failed to get namespace config: %w", err)
			}
		}

		// Check if this cluster is selected by the namespace Placement
		// (Placement resource is created/updated by ResourceCreator)
		nsSelected, err := o.isClusterSelectedByRSPlacement(ctx, rightsizing.NamespacePlacementName, cluster.Name)
		if err != nil {
			o.Logger.Error(err, "Failed to check namespace placement selection, defaulting to selected")
			nsSelected = true
		}

		if nsSelected {
			nsOpts, err := o.buildNamespaceOptionsFromConfig(nsConfigData)
			if err != nil {
				return ret, fmt.Errorf("failed to build namespace right-sizing options: %w", err)
			}
			ret.NamespaceRightSizing = nsOpts
		} else {
			o.Logger.V(1).Info("Cluster not selected for namespace right-sizing", "cluster", cluster.Name)
		}
	}

	// Build virtualization right-sizing options
	if virtualizationEnabled {
		// Ensure ConfigMap exists on hub (MCOA owns all RS resources)
		if err := o.ensureVirtualizationConfigMap(ctx); err != nil {
			o.Logger.Error(err, "Failed to ensure virtualization ConfigMap exists, continuing with defaults")
		}

		virtConfigData, err := o.getConfigData(ctx, rightsizing.VirtualizationConfigMapName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				virtConfigData = rightsizing.RSConfigMapData{
					PrometheusRuleConfig:   rightsizing.GetDefaultRSPrometheusRuleConfig(),
					PlacementConfiguration: rightsizing.GetDefaultRSPlacement(),
				}
			} else {
				return ret, fmt.Errorf("failed to get virtualization config: %w", err)
			}
		}

		// Check if this cluster is selected by the virtualization Placement
		virtSelected, err := o.isClusterSelectedByRSPlacement(ctx, rightsizing.VirtualizationPlacementName, cluster.Name)
		if err != nil {
			o.Logger.Error(err, "Failed to check virtualization placement selection, defaulting to selected")
			virtSelected = true
		}

		if virtSelected {
			virtOpts, err := o.buildVirtualizationOptionsFromConfig(virtConfigData)
			if err != nil {
				return ret, fmt.Errorf("failed to build virtualization right-sizing options: %w", err)
			}
			ret.VirtualizationRightSizing = virtOpts
		} else {
			o.Logger.V(1).Info("Cluster not selected for virtualization right-sizing", "cluster", cluster.Name)
		}
	}

	// Generate ScrapeConfig for metrics federation if any right-sizing is enabled
	if namespaceEnabled || virtualizationEnabled {
		scrapeConfig := rightsizing.GenerateScrapeConfig(namespaceEnabled, virtualizationEnabled)
		if scrapeConfig != nil {
			// Add ScrapeConfig to namespace options (it will be merged with platform ScrapeConfigs)
			if namespaceEnabled {
				ret.NamespaceRightSizing.ScrapeConfigs = append(ret.NamespaceRightSizing.ScrapeConfigs, scrapeConfig)
			} else {
				ret.VirtualizationRightSizing.ScrapeConfigs = append(ret.VirtualizationRightSizing.ScrapeConfigs, scrapeConfig)
			}
			o.Logger.V(2).Info("Generated right-sizing ScrapeConfig for metrics federation",
				"namespaceEnabled", namespaceEnabled,
				"virtualizationEnabled", virtualizationEnabled)
		}
	}

	return ret, nil
}

func (o *OptionsBuilder) buildNamespaceOptionsFromConfig(configData rightsizing.RSConfigMapData) (ComponentOptions, error) {
	opts := ComponentOptions{Enabled: true}
	rule, err := rsnamespace.GeneratePrometheusRule(configData)
	if err != nil {
		return opts, fmt.Errorf("failed to generate namespace PrometheusRule: %w", err)
	}
	opts.PrometheusRules = []*monitoringv1.PrometheusRule{&rule}
	return opts, nil
}

func (o *OptionsBuilder) buildVirtualizationOptionsFromConfig(configData rightsizing.RSConfigMapData) (ComponentOptions, error) {
	opts := ComponentOptions{Enabled: true}
	rule, err := rsvirtualization.GeneratePrometheusRule(configData)
	if err != nil {
		return opts, fmt.Errorf("failed to generate virtualization PrometheusRule: %w", err)
	}
	opts.PrometheusRules = []*monitoringv1.PrometheusRule{&rule}
	return opts, nil
}

func (o *OptionsBuilder) getConfigData(ctx context.Context, configMapName string) (rightsizing.RSConfigMapData, error) {
	cm, err := common.GetConfigMap(ctx, o.Client, addoncfg.InstallNamespace, configMapName)
	if err != nil {
		return rightsizing.RSConfigMapData{}, err
	}

	return rightsizing.ParseConfigMapData(cm.Data)
}

// ensureNamespaceConfigMap ensures the namespace right-sizing ConfigMap exists on the hub.
// MCOA owns all right-sizing resources including ConfigMaps for cleaner architecture.
func (o *OptionsBuilder) ensureNamespaceConfigMap(ctx context.Context) error {
	_, err := common.GetConfigMap(ctx, o.Client, addoncfg.InstallNamespace, rightsizing.NamespaceConfigMapName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			o.Logger.Info("Creating namespace right-sizing ConfigMap with defaults",
				"name", rightsizing.NamespaceConfigMapName,
				"namespace", addoncfg.InstallNamespace)
			return o.createDefaultConfigMap(ctx, rightsizing.NamespaceConfigMapName, rightsizing.GetDefaultNamespaceConfigData())
		}
		return err
	}
	// ConfigMap already exists
	return nil
}

// ensureVirtualizationConfigMap ensures the virtualization right-sizing ConfigMap exists on the hub.
// MCOA owns all right-sizing resources including ConfigMaps for cleaner architecture.
func (o *OptionsBuilder) ensureVirtualizationConfigMap(ctx context.Context) error {
	_, err := common.GetConfigMap(ctx, o.Client, addoncfg.InstallNamespace, rightsizing.VirtualizationConfigMapName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			o.Logger.Info("Creating virtualization right-sizing ConfigMap with defaults",
				"name", rightsizing.VirtualizationConfigMapName,
				"namespace", addoncfg.InstallNamespace)
			return o.createDefaultConfigMap(ctx, rightsizing.VirtualizationConfigMapName, rightsizing.GetDefaultVirtualizationConfigData())
		}
		return err
	}
	// ConfigMap already exists
	return nil
}

// createDefaultConfigMap creates a ConfigMap with the provided data.
// The ConfigMap is labeled to indicate it's managed by MCOA for right-sizing.
func (o *OptionsBuilder) createDefaultConfigMap(ctx context.Context, name string, data map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: addoncfg.InstallNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":  "right-sizing",
				"app.kubernetes.io/managed-by": "multicluster-observability-addon",
			},
		},
		Data: data,
	}

	if err := o.Client.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create ConfigMap %s: %w", name, err)
	}

	o.Logger.V(1).Info("Created right-sizing ConfigMap", "name", name, "namespace", addoncfg.InstallNamespace)
	return nil
}

