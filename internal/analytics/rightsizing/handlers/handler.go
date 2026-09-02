package handlers

import (
	"context"
	"fmt"
	"slices"
	"sync"

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
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// lastValidAggregators caches the last known valid aggregator config per ConfigMap,
// so that an edit containing any invalid entry can be rejected entirely and the
// previous valid config restored. The cache is keyed by "configMapName:cpu" or
// "configMapName:memory". On process restart with an invalid ConfigMap the cache
// is empty and the resolver falls back to hardcoded defaults.
var (
	lastValidMu          sync.RWMutex
	lastValidAggregators = make(map[string][]string)
)

func cacheKey(configMapName, field string) string {
	return configMapName + ":" + field
}

func cacheValidAggregator(configMapName, field string, values []string) {
	lastValidMu.Lock()
	defer lastValidMu.Unlock()
	dst := make([]string, len(values))
	copy(dst, values)
	lastValidAggregators[cacheKey(configMapName, field)] = dst
}

func getCachedAggregator(configMapName, field string) ([]string, bool) {
	lastValidMu.RLock()
	defer lastValidMu.RUnlock()
	v, ok := lastValidAggregators[cacheKey(configMapName, field)]
	if !ok {
		return nil, false
	}
	dst := make([]string, len(v))
	copy(dst, v)
	return dst, true
}

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

	if !opts.Platform.AnalyticsOptions.RightSizing.Delegated {
		return ret, nil
	}

	namespaceEnabled := opts.Platform.AnalyticsOptions.RightSizing.NamespaceEnabled
	virtualizationEnabled := opts.Platform.AnalyticsOptions.RightSizing.VirtualizationEnabled

	nsMatched := false
	virtMatched := false

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
		o.validateAndSanitizeConfig(&nsConfigData, rightsizing.NamespaceConfigMapName)

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
		o.validateAndSanitizeConfig(&virtConfigData, rightsizing.VirtualizationConfigMapName)

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

	if opts.Platform.Metrics.CollectionEnabled {
		ret.ScrapeConfig = rightsizing.GenerateScrapeConfig(nsMatched, virtMatched)
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

// validateAndSanitizeConfig checks cpuAggregator and memoryAggregator for invalid
// profile names. If ANY entry in a list is invalid, the entire edit is rejected
// and the previous valid config (from cache) is restored. This prevents partial
// application of a bad edit. When the cache is empty (e.g. after restart) and the
// config is invalid, the list is set to nil so downstream resolvers fall back to
// hardcoded defaults.
func (o *OptionsBuilder) validateAndSanitizeConfig(configData *rightsizing.RSConfigMapData, configMapName string) {
	if invalid := rightsizing.ValidateAggregatorNames(configData.PrometheusRuleConfig.CpuAggregator); len(invalid) > 0 {
		if cached, ok := getCachedAggregator(configMapName, "cpu"); ok {
			o.Logger.Info("Invalid cpuAggregator values in ConfigMap, rejecting edit and keeping previous valid config",
				"configMap", configMapName, "invalidValues", invalid, "restoredConfig", cached)
			configData.PrometheusRuleConfig.CpuAggregator = cached
		} else {
			o.Logger.Info("Invalid cpuAggregator values in ConfigMap, no previous config cached, falling back to defaults",
				"configMap", configMapName, "invalidValues", invalid)
			configData.PrometheusRuleConfig.CpuAggregator = nil
		}
	} else {
		cacheValidAggregator(configMapName, "cpu", configData.PrometheusRuleConfig.CpuAggregator)
	}

	if invalid := rightsizing.ValidateAggregatorNames(configData.PrometheusRuleConfig.MemoryAggregator); len(invalid) > 0 {
		if cached, ok := getCachedAggregator(configMapName, "memory"); ok {
			o.Logger.Info("Invalid memoryAggregator values in ConfigMap, rejecting edit and keeping previous valid config",
				"configMap", configMapName, "invalidValues", invalid, "restoredConfig", cached)
			configData.PrometheusRuleConfig.MemoryAggregator = cached
		} else {
			o.Logger.Info("Invalid memoryAggregator values in ConfigMap, no previous config cached, falling back to defaults",
				"configMap", configMapName, "invalidValues", invalid)
			configData.PrometheusRuleConfig.MemoryAggregator = nil
		}
	} else {
		cacheValidAggregator(configMapName, "memory", configData.PrometheusRuleConfig.MemoryAggregator)
	}
}

//
// Both MCO and MCOA use a "create if not exists" pattern for these ConfigMaps,
// so user customizations (namespace filters, recommendation %, placement predicates)
// are preserved across mode switches (MCO <=> MCOA). While upgrading from ACM 2.17 to ACM 5.0
//

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
	return nil
}

// createDefaultConfigMap creates a ConfigMap with the provided data.
// The ConfigMap is labeled to indicate it's managed for right-sizing.
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
