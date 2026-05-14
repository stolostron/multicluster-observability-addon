package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	rsgpu "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/gpu"
	rsnamespace "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/namespace"
	rsvirtualization "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/virtualization"
	rsworkload "github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing/workload"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	predictionEnabled := opts.Platform.AnalyticsOptions.RightSizing.PredictionEnabled
	if predictionEnabled {
		if err := o.ensurePredictionConfigMap(ctx, opts); err != nil {
			o.Logger.Error(err, "Failed to ensure prediction ConfigMap exists, continuing")
		}
	}

	namespaceEnabled := opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled
	virtualizationEnabled := opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled
	workloadPodEnabled := opts.Platform.AnalyticsOptions.RightSizing.WorkloadPodEnabled
	gpuEnabled := opts.Platform.AnalyticsOptions.RightSizing.GPUEnabled

	nsMatched := false
	virtMatched := false
	wlMatched := false
	gpuMatched := false

	// Build namespace right-sizing options
	if namespaceEnabled {
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

		if clusterMatchesPlacement(cluster, nsConfigData.PlacementConfiguration) {
			nsOpts, err := o.buildNamespaceOptionsFromConfig(nsConfigData)
			if err != nil {
				return ret, fmt.Errorf("failed to build namespace right-sizing options: %w", err)
			}
			ret.NamespaceRightSizing = nsOpts
			nsMatched = true
		} else {
			o.Logger.V(1).Info("Cluster not selected for namespace right-sizing", "cluster", cluster.Name)
		}
	}

	// Build virtualization right-sizing options
	if virtualizationEnabled {
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

		if clusterMatchesPlacement(cluster, virtConfigData.PlacementConfiguration) {
			virtOpts, err := o.buildVirtualizationOptionsFromConfig(virtConfigData)
			if err != nil {
				return ret, fmt.Errorf("failed to build virtualization right-sizing options: %w", err)
			}
			ret.VirtualizationRightSizing = virtOpts
			virtMatched = true
		} else {
			o.Logger.V(1).Info("Cluster not selected for virtualization right-sizing", "cluster", cluster.Name)
		}
	}

	// Build workload-pod right-sizing options
	if workloadPodEnabled {
		if err := o.ensureWorkloadConfigMap(ctx); err != nil {
			o.Logger.Error(err, "Failed to ensure workload ConfigMap exists, continuing with defaults")
		}

		wlConfigData, err := o.getConfigData(ctx, rightsizing.WorkloadConfigMapName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				wlConfigData = rightsizing.RSConfigMapData{
					PrometheusRuleConfig:   rightsizing.GetDefaultRSPrometheusRuleConfig(),
					PlacementConfiguration: rightsizing.GetDefaultRSPlacement(),
				}
			} else {
				return ret, fmt.Errorf("failed to get workload config: %w", err)
			}
		}

		if clusterMatchesPlacement(cluster, wlConfigData.PlacementConfiguration) {
			wlOpts, err := o.buildWorkloadOptionsFromConfig(wlConfigData)
			if err != nil {
				return ret, fmt.Errorf("failed to build workload right-sizing options: %w", err)
			}
			ret.WorkloadPodRightSizing = wlOpts
			wlMatched = true
		} else {
			o.Logger.V(1).Info("Cluster not selected for workload-pod right-sizing", "cluster", cluster.Name)
		}
	}

	// Build GPU right-sizing options
	if gpuEnabled {
		if err := o.ensureGPUConfigMap(ctx); err != nil {
			o.Logger.Error(err, "Failed to ensure GPU ConfigMap exists, continuing with defaults")
		}

		gpuConfigData, err := o.getConfigData(ctx, rightsizing.GPUConfigMapName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				gpuConfigData = rightsizing.RSConfigMapData{
					PrometheusRuleConfig:   rightsizing.GetDefaultRSPrometheusRuleConfig(),
					PlacementConfiguration: rightsizing.GetDefaultRSPlacement(),
				}
			} else {
				return ret, fmt.Errorf("failed to get GPU config: %w", err)
			}
		}

		if clusterMatchesPlacement(cluster, gpuConfigData.PlacementConfiguration) {
			gpuOpts, err := o.buildGPUOptionsFromConfig(gpuConfigData, wlMatched)
			if err != nil {
				return ret, fmt.Errorf("failed to build GPU right-sizing options: %w", err)
			}
			ret.GPURightSizing = gpuOpts
			gpuMatched = true
		} else {
			o.Logger.V(1).Info("Cluster not selected for GPU right-sizing", "cluster", cluster.Name)
		}
	}

	if opts.Platform.Metrics.CollectionEnabled {
		ret.ScrapeConfig = rightsizing.GenerateScrapeConfig(nsMatched, virtMatched, wlMatched, gpuMatched, predictionEnabled)
	}

	if predictionEnabled {
		ret.PredictionEnabled = true
		ret.PredictionProvider = opts.Platform.AnalyticsOptions.RightSizing.PredictionProvider
		pc := strings.TrimSpace(opts.Platform.AnalyticsOptions.RightSizing.PredictionConfig)
		if pc != "" {
			ret.PredictionConfig = json.RawMessage(pc)
		} else {
			ret.PredictionConfig = json.RawMessage("{}")
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

func (o *OptionsBuilder) buildWorkloadOptionsFromConfig(configData rightsizing.RSConfigMapData) (ComponentOptions, error) {
	opts := ComponentOptions{Enabled: true}
	rule, err := rsworkload.GeneratePrometheusRule(configData)
	if err != nil {
		return opts, fmt.Errorf("failed to generate workload PrometheusRule: %w", err)
	}
	opts.PrometheusRules = []*monitoringv1.PrometheusRule{&rule}
	return opts, nil
}

// buildGPUOptionsFromConfig builds the GPU right-sizing component options.
// When workload-pod RS is also matched, the pod-workload mapping rule is omitted
// from the GPU PrometheusRule to avoid duplicate recording rules.
func (o *OptionsBuilder) buildGPUOptionsFromConfig(configData rightsizing.RSConfigMapData, workloadPodAlsoEnabled bool) (ComponentOptions, error) {
	opts := ComponentOptions{Enabled: true}
	includePodWorkloadMapping := !workloadPodAlsoEnabled
	rule, err := rsgpu.GeneratePrometheusRuleWithMapping(configData, includePodWorkloadMapping)
	if err != nil {
		return opts, fmt.Errorf("failed to generate GPU PrometheusRule: %w", err)
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

// ensureWorkloadConfigMap ensures the workload right-sizing ConfigMap exists on the hub.
func (o *OptionsBuilder) ensureWorkloadConfigMap(ctx context.Context) error {
	_, err := common.GetConfigMap(ctx, o.Client, addoncfg.InstallNamespace, rightsizing.WorkloadConfigMapName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			o.Logger.Info("Creating workload right-sizing ConfigMap with defaults",
				"name", rightsizing.WorkloadConfigMapName,
				"namespace", addoncfg.InstallNamespace)
			return o.createDefaultConfigMap(ctx, rightsizing.WorkloadConfigMapName, rightsizing.GetDefaultWorkloadConfigData())
		}
		return err
	}
	return nil
}

// ensureGPUConfigMap ensures the GPU right-sizing ConfigMap exists on the hub.
func (o *OptionsBuilder) ensureGPUConfigMap(ctx context.Context) error {
	_, err := common.GetConfigMap(ctx, o.Client, addoncfg.InstallNamespace, rightsizing.GPUConfigMapName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			o.Logger.Info("Creating GPU right-sizing ConfigMap with defaults",
				"name", rightsizing.GPUConfigMapName,
				"namespace", addoncfg.InstallNamespace)
			return o.createDefaultConfigMap(ctx, rightsizing.GPUConfigMapName, rightsizing.GetDefaultGPUConfigData())
		}
		return err
	}
	return nil
}

// createDefaultConfigMap creates a ConfigMap with the provided data.
// The ConfigMap is labeled to indicate it's managed for right-sizing.
//
// Both MCO and MCOA use a "create if not exists" pattern for these ConfigMaps,
// so user customizations (namespace filters, recommendation %, placement predicates)
// are preserved across mode switches (MCO <=> MCOA).
func (o *OptionsBuilder) createDefaultConfigMap(ctx context.Context, name string, data map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: addoncfg.InstallNamespace,
			Labels:    rightsizing.RSLabels(),
		},
		Data: data,
	}

	if err := o.Client.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create ConfigMap %s: %w", name, err)
	}

	o.Logger.V(1).Info("Created right-sizing ConfigMap", "name", name, "namespace", addoncfg.InstallNamespace)
	return nil
}

func (o *OptionsBuilder) ensurePredictionConfigMap(ctx context.Context, addonOpts addon.Options) error {
	hOpts := Options{
		PredictionEnabled:  true,
		PredictionProvider: addonOpts.Platform.AnalyticsOptions.RightSizing.PredictionProvider,
	}
	pc := strings.TrimSpace(addonOpts.Platform.AnalyticsOptions.RightSizing.PredictionConfig)
	if pc != "" {
		hOpts.PredictionConfig = json.RawMessage(pc)
	} else {
		hOpts.PredictionConfig = json.RawMessage("{}")
	}

	payload, err := PredictionHubConfigBytes(hOpts)
	if err != nil {
		return fmt.Errorf("marshal prediction hub config: %w", err)
	}

	_, err = common.GetConfigMap(ctx, o.Client, addoncfg.InstallNamespace, rightsizing.RSPredictionConfigMapName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			o.Logger.Info("Creating prediction ConfigMap with defaults",
				"name", rightsizing.RSPredictionConfigMapName,
				"namespace", addoncfg.InstallNamespace)
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rightsizing.RSPredictionConfigMapName,
					Namespace: addoncfg.InstallNamespace,
					Labels:    rightsizing.RSLabels(),
				},
				Data: map[string]string{
					"config.json": string(payload),
				},
			}
			if err := o.Client.Create(ctx, cm); err != nil {
				return fmt.Errorf("failed to create ConfigMap %s: %w", rightsizing.RSPredictionConfigMapName, err)
			}
			return nil
		}
		return err
	}
	return nil
}

// clusterMatchesPlacement evaluates placement predicates in-memory against
// a ManagedCluster, avoiding the need to create Placement resources and rely
// on the OCM scheduler for PlacementDecisions.
// Predicates are ORed (any match selects the cluster). Empty predicates match all.
func clusterMatchesPlacement(cluster *clusterv1.ManagedCluster, placement clusterv1beta1.Placement) bool {
	if len(placement.Spec.Predicates) == 0 {
		return true
	}

	for _, predicate := range placement.Spec.Predicates {
		if clusterMatchesPredicate(cluster, predicate) {
			return true
		}
	}
	return false
}

func clusterMatchesPredicate(cluster *clusterv1.ManagedCluster, pred clusterv1beta1.ClusterPredicate) bool {
	sel := pred.RequiredClusterSelector

	if !clusterMatchesLabelSelector(cluster, sel.LabelSelector) {
		return false
	}
	if !clusterMatchesClaimSelector(cluster, sel.ClaimSelector) {
		return false
	}
	return true
}

func clusterMatchesLabelSelector(cluster *clusterv1.ManagedCluster, ls metav1.LabelSelector) bool {
	selector, err := metav1.LabelSelectorAsSelector(&ls)
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(cluster.Labels))
}

func clusterMatchesClaimSelector(cluster *clusterv1.ManagedCluster, cs clusterv1beta1.ClusterClaimSelector) bool {
	if len(cs.MatchExpressions) == 0 {
		return true
	}

	claimMap := make(map[string]string, len(cluster.Status.ClusterClaims))
	for _, claim := range cluster.Status.ClusterClaims {
		claimMap[claim.Name] = claim.Value
	}

	for _, req := range cs.MatchExpressions {
		val, exists := claimMap[req.Key]
		switch req.Operator {
		case metav1.LabelSelectorOpIn:
			if !exists || !stringInSlice(val, req.Values) {
				return false
			}
		case metav1.LabelSelectorOpNotIn:
			if exists && stringInSlice(val, req.Values) {
				return false
			}
		case metav1.LabelSelectorOpExists:
			if !exists {
				return false
			}
		case metav1.LabelSelectorOpDoesNotExist:
			if exists {
				return false
			}
		}
	}
	return true
}

func stringInSlice(s string, slice []string) bool {
	return slices.Contains(slice, s)
}
