package resource

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/go-logr/logr"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	addonv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	errTooManyConfigResources = errors.New("too many configuration resources")
	errMissingHubEndpoint     = errors.New("hub endpoint is missing")
)

// DefaultStackResources reconciles the configuration resources needed for metrics collection
type DefaultStackResources struct {
	AddonOptions       addon.Options
	Client             client.Client
	CMAO               *addonv1beta1.ClusterManagementAddOn
	Logger             logr.Logger
	KubeRBACProxyImage string
	PrometheusImage    string
}

// Reconcile ensures the state of the configuration resources for metrics collection.
// For each placement found in the ClusterManagementAddon resource, it generates a default PrometheusAgent
// if not found and then applies configuration invariants using server-side apply.
func (d DefaultStackResources) Reconcile(ctx context.Context) ([]common.DefaultConfig, error) {
	d.Logger.V(1).Info("reconciling DefaultStackResources for metrics", "platformMetricsCollectionEnabled", d.AddonOptions.Platform.Metrics.CollectionEnabled,
		"userWorkloadsMetricsCollectionEnabled", d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled)
	configs := []common.DefaultConfig{}

	var mcoUID types.UID
	for _, owner := range d.CMAO.OwnerReferences {
		if owner.Controller != nil && *owner.Controller {
			mcoUID = owner.UID
			break
		}
	}

	hasHostedClusters := config.HasHostedCLusters(ctx, d.Client, d.Logger)
	if d.AddonOptions.Platform.Metrics.CollectionEnabled {
		// Generate a specific agent config for each placement
		for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
			agentConfig, err := d.reconcileAgentForPlacement(ctx, placement.PlacementRef, false)
			if err != nil {
				return configs, fmt.Errorf("failed to reconcile prometheusAgent %s for placement %s: %w", agentConfig.Config.Name, placement.Name, err)
			}
			configs = append(configs, agentConfig)
		}

		// ScrapeConfigs are common to all placements
		scConfigs, err := d.reconcileScrapeConfigs(ctx, mcoUID, false, hasHostedClusters)
		if err != nil {
			return configs, fmt.Errorf("failed to reconcile scrapeConfigs: %w", err)
		}
		configs = append(configs, scConfigs...)
	}

	if d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
		// Generate a specific agent config for each placement
		for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
			agentConfig, err := d.reconcileAgentForPlacement(ctx, placement.PlacementRef, true)
			if err != nil {
				return configs, fmt.Errorf("failed to reconcile prometheusAgent %s for placement %s: %w", agentConfig.Config.Name, placement.Name, err)
			}
			configs = append(configs, agentConfig)
		}

		// ScrapeConfigs are common to all placements
		scConfigs, err := d.reconcileScrapeConfigs(ctx, mcoUID, true, hasHostedClusters)
		if err != nil {
			return configs, fmt.Errorf("failed to reconcile scrapeConfigs: %w", err)
		}
		configs = append(configs, scConfigs...)
	}

	if d.AddonOptions.Platform.Metrics.CollectionEnabled || d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
		// Platforn and uwl rules are processed the same way. They are common to all placements.
		ruleConfigs, err := d.getPrometheusRules(ctx, mcoUID, hasHostedClusters)
		if err != nil {
			return configs, fmt.Errorf("failed to get prometheusRules: %w", err)
		}
		configs = append(configs, ruleConfigs...)
	}

	return configs, nil
}

func (d DefaultStackResources) reconcileScrapeConfigs(ctx context.Context, mcoUID types.UID, isUWL, hasHostedClusters bool) ([]common.DefaultConfig, error) {
	labelVals := []string{}
	d.Logger.V(2).Info("reconciling ScrapeConfigs", "mcoUID", mcoUID, "isUWL", isUWL, "hasHostedClusters", hasHostedClusters)

	if len(mcoUID) == 0 {
		return []common.DefaultConfig{}, nil
	}

	if isUWL {
		labelVals = append(labelVals, config.UserWorkloadPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])
		// Avoid adding HCP's specific confs when not needed
		if hasHostedClusters {
			labelVals = append(labelVals, config.EtcdHcpUserWorkloadPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey], config.ApiserverHcpUserWorkloadPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])
		}
	} else {
		labelVals = append(labelVals, config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])
	}

	req, err := labels.NewRequirement(addoncfg.ComponentK8sLabelKey, selection.In, labelVals)
	if err != nil {
		return nil, fmt.Errorf("failed to create labels requirement for scrapeConfigs: %w", err)
	}
	labelsSelector := labels.NewSelector().Add(*req)

	scrapeConfigsList := &cooprometheusv1alpha1.ScrapeConfigList{}
	if err = d.Client.List(ctx, scrapeConfigsList, client.InNamespace(addoncfg.InstallNamespace), client.MatchingLabelsSelector{Selector: labelsSelector}); err != nil {
		return nil, fmt.Errorf("failed to list scrapeConfigs: %w", err)
	}

	mcoManagedScrapeConfigs := []client.Object{}
	userDefinedScrapeConfigs := []client.Object{}
	for _, existingSC := range scrapeConfigsList.Items {
		// Ensures that we only filter for MCO-managed scrape configs or user-defined scrape configs that have at least one of these labels along with the required annotation for user-defined scrape configs
		if !hasControllerUID(existingSC.OwnerReferences, mcoUID) && !(existingSC.Labels[addoncfg.PartOfK8sLabelKey] == addoncfg.Name) {
			continue
		}

		desiredSC := existingSC.DeepCopy()
		desiredSC.ManagedFields = nil // required for patching with ssa

		if desiredSC.Labels == nil {
			desiredSC.Labels = map[string]string{}
		}
		desiredSC.Labels[addoncfg.BackupLabelKey] = addoncfg.BackupLabelValue

		if !isUWL {
			// Enforce empty values, they are set when generating the manifests for a given managedCluster
			desiredSC.Spec.ScrapeClassName = ptr.To("not-configurable")
			desiredSC.Spec.Scheme = ptr.To(cooprometheusv1.Scheme("HTTPS"))
			desiredSC.Spec.StaticConfigs = []cooprometheusv1alpha1.StaticConfig{
				{
					Targets: []cooprometheusv1alpha1.Target{
						"not-configurable",
					},
				},
			}
		}

		// SSA the objects rendered
		if !equality.Semantic.DeepDerivative(desiredSC.Spec, existingSC.Spec) ||
			!equality.Semantic.DeepDerivative(desiredSC.Labels, existingSC.Labels) {
			if err = common.ServerSideApply(ctx, d.Client, desiredSC, nil); err != nil { // object is controlled by MCO, no owner
				return nil, fmt.Errorf("failed to patch with with server-side apply: %w", err)
			}
			d.Logger.Info("updated scrapeConfig with server-side apply", "namespace", desiredSC.Namespace, "name", desiredSC.Name)
		}
		if hasControllerUID(existingSC.OwnerReferences, mcoUID) {
			mcoManagedScrapeConfigs = append(mcoManagedScrapeConfigs, desiredSC)
		} else {
			userDefinedScrapeConfigs = append(userDefinedScrapeConfigs, desiredSC)
		}
	}
	configs, err := d.generateConfigsForAllPlacements(mcoManagedScrapeConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate default configs: %w", err)
	}
	for _, userDefinedSC := range userDefinedScrapeConfigs {
		placementAnnotations := userDefinedSC.(*cooprometheusv1alpha1.ScrapeConfig).Annotations[addoncfg.PlacementAnnotationKey]
		placementRefs, err := d.generatePlacementRefs(placementAnnotations)
		if err != nil {
			return nil, fmt.Errorf("failed to generate placement refs: %w", err)
		}
		cfg, err := common.ObjectToAddonConfig(userDefinedSC)
		if err != nil {
			return nil, fmt.Errorf("failed to generate addon config for %s: %w", userDefinedSC.(*cooprometheusv1alpha1.ScrapeConfig).Name, err)
		}
		for _, placementRef := range placementRefs {
			configs = append(configs, common.DefaultConfig{
				PlacementRef: placementRef,
				Config:       cfg,
			})
		}
	}

	return configs, nil
}

func (d DefaultStackResources) getPrometheusRules(ctx context.Context, mcoUID types.UID, hasHostedClusters bool) ([]common.DefaultConfig, error) {
	if !d.AddonOptions.Platform.Metrics.CollectionEnabled && !d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
		return []common.DefaultConfig{}, nil
	}
	d.Logger.V(2).Info("reconciling PrometheusRules", "mcoUID", mcoUID, "hasHostedClusters", hasHostedClusters)

	if len(mcoUID) == 0 {
		return []common.DefaultConfig{}, nil
	}

	labelVals := []string{}
	if d.AddonOptions.Platform.Metrics.CollectionEnabled {
		labelVals = append(labelVals, config.PlatformPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])
	}
	if d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
		labelVals = append(labelVals, config.UserWorkloadPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])

		// Avoid adding HCP's specific confs when not needed
		if hasHostedClusters {
			labelVals = append(labelVals, config.EtcdHcpUserWorkloadPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey], config.ApiserverHcpUserWorkloadPrometheusMatchLabels[addoncfg.ComponentK8sLabelKey])
		}
	}

	req, err := labels.NewRequirement(addoncfg.ComponentK8sLabelKey, selection.In, labelVals)
	if err != nil {
		return nil, fmt.Errorf("failed to create labels requirement: %w", err)
	}
	labelSelector := labels.NewSelector().Add(*req)

	promRuleList := &prometheusv1.PrometheusRuleList{}
	if err = d.Client.List(ctx, promRuleList, client.InNamespace(addoncfg.InstallNamespace), client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, fmt.Errorf("failed to list prometheusRules: %w", err)
	}

	mcoManagedRules := []client.Object{}
	userDefinedRules := []client.Object{}
	for _, rule := range promRuleList.Items {
		if !hasControllerUID(rule.OwnerReferences, mcoUID) && !(rule.Labels[addoncfg.PartOfK8sLabelKey] == addoncfg.Name) {
			continue
		}

		if hasControllerUID(rule.OwnerReferences, mcoUID) {
			mcoManagedRules = append(mcoManagedRules, &rule)
		} else {
			userDefinedRules = append(userDefinedRules, &rule)
		}
	}

	configs, err := d.generateConfigsForAllPlacements(mcoManagedRules)
	if err != nil {
		return nil, fmt.Errorf("failed to generate default configs for prometheusRules: %w", err)
	}

	for _, userDefinedRule := range userDefinedRules {
		placementAnnotations := userDefinedRule.(*prometheusv1.PrometheusRule).Annotations[addoncfg.PlacementAnnotationKey]
		placementRefs, err := d.generatePlacementRefs(placementAnnotations)
		if err != nil {
			return nil, fmt.Errorf("failed to generate placement refs: %w", err)
		}
		cfg, err := common.ObjectToAddonConfig(userDefinedRule)
		if err != nil {
			return nil, fmt.Errorf("failed to generate addon config for %s: %w", userDefinedRule.(*prometheusv1.PrometheusRule).Name, err)
		}
		for _, placementRef := range placementRefs {
			configs = append(configs, common.DefaultConfig{
				PlacementRef: placementRef,
				Config:       cfg,
			})
		}
	}

	return configs, nil
}

func (d DefaultStackResources) reconcileAgentForPlacement(ctx context.Context, placementRef addonv1beta1.PlacementRef, isUWL bool) (common.DefaultConfig, error) {
	d.Logger.V(2).Info("reconciling prometheus agent", "placementName", placementRef.Name, "placementNamespace", placementRef.Namespace, "isUWL", isUWL)
	// Get or create default
	agent, err := d.getOrCreateDefaultAgent(ctx, placementRef, isUWL)
	if err != nil {
		return common.DefaultConfig{}, fmt.Errorf("failed to get or create agent for placement %s: %w", placementRef.Name, err)
	}

	if d.AddonOptions.Platform.Metrics.HubEndpoint.Host == "" {
		return common.DefaultConfig{}, errMissingHubEndpoint
	}

	// SSA mendatory field values
	promBuilder := PrometheusAgentSSA{
		ExistingAgent:       agent,
		IsUwl:               isUWL,
		PrometheusImage:     d.PrometheusImage,
		KubeRBACProxyImage:  d.KubeRBACProxyImage,
		RemoteWriteEndpoint: d.AddonOptions.Platform.Metrics.HubEndpoint.String(),
		Labels: map[string]string{
			addoncfg.PlacementRefNameLabelKey:      placementRef.Name,
			addoncfg.PlacementRefNamespaceLabelKey: placementRef.Namespace,
		},
	}
	promSSA := promBuilder.Build()

	// SSA the objects rendered
	if !equality.Semantic.DeepDerivative(promSSA.Spec, agent.Spec) || !maps.Equal(promSSA.Labels, agent.Labels) {
		if err = common.ServerSideApply(ctx, d.Client, promSSA, d.CMAO); err != nil {
			return common.DefaultConfig{}, fmt.Errorf("failed to server-side apply for %s/%s: %w", promSSA.Namespace, promSSA.Name, err)
		}
		d.Logger.Info("updated prometheus agent with server-side apply", "namespace", promSSA.Namespace, "name", promSSA.Name)
	}

	cfg, err := common.ObjectToAddonConfig(promSSA)
	if err != nil {
		return common.DefaultConfig{}, fmt.Errorf("failed to generate addon config for %s: %w", agent.Name, err)
	}

	return common.DefaultConfig{
		PlacementRef: placementRef,
		Config:       cfg,
	}, nil
}

func (d DefaultStackResources) getOrCreateDefaultAgent(ctx context.Context, placementRef addonv1beta1.PlacementRef, isUWL bool) (*cooprometheusv1alpha1.PrometheusAgent, error) {
	promAgents := &cooprometheusv1alpha1.PrometheusAgentList{}
	if err := d.Client.List(ctx, promAgents, &client.ListOptions{
		Namespace:     config.HubInstallNamespace,
		LabelSelector: labels.SelectorFromSet(labels.Set(makeConfigResourceLabels(isUWL, placementRef))),
	}); err != nil {
		return nil, fmt.Errorf("failed to list existing prometheusAgents: %w", err)
	}

	if len(promAgents.Items) > 1 {
		names := []string{}
		for _, item := range promAgents.Items {
			names = append(names, item.Name)
		}
		return nil, fmt.Errorf("%w: found %d prometheusAgents in namespace %q with names %+v", errTooManyConfigResources, len(promAgents.Items), config.HubInstallNamespace, names)
	}

	if len(promAgents.Items) == 1 {
		return &promAgents.Items[0], nil
	}
	// Create default resource
	appName := config.PlatformMetricsCollectorApp
	if isUWL {
		appName = config.UserWorkloadMetricsCollectorApp
	}
	agent := NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(appName, placementRef.Name), isUWL, placementRef)

	if err := controllerutil.SetControllerReference(d.CMAO, agent, d.Client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner reference on default agent for placement %q: %w", placementRef.Name, err)
	}

	if err := d.Client.Create(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create the default agent for placement %q: %w", placementRef.Name, err)
	}
	d.Logger.Info("created default prometheus agent for placement", "agentNamespace", agent.Namespace, "agentName", agent.Name, "placementName", placementRef.Name)

	// Re-fetch the agent to populate server-side fields and, critically, TypeMeta.
	// The 'agent' object was mutated by Create() and its TypeMeta is now empty.
	key := client.ObjectKeyFromObject(agent)
	createdAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	if err := d.Client.Get(ctx, key, createdAgent); err != nil {
		return nil, fmt.Errorf("failed to re-fetch created default agent %q: %w", key, err)
	}

	return createdAgent, nil
}

func (d DefaultStackResources) generateConfigsForAllPlacements(object []client.Object) ([]common.DefaultConfig, error) {
	// Compute configs to add to each placement
	addonConfigs := []addonv1beta1.AddOnConfig{}
	for _, obj := range object {
		cfg, err := common.ObjectToAddonConfig(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to generate addon config from object %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
		addonConfigs = append(addonConfigs, cfg)
	}

	defaultConfigs := []common.DefaultConfig{}
	for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
		for _, cfg := range addonConfigs {
			defaultConfigs = append(defaultConfigs, common.DefaultConfig{
				PlacementRef: placement.PlacementRef,
				Config:       cfg,
			})
		}
	}

	return defaultConfigs, nil
}

func (d DefaultStackResources) generatePlacementRefs(placementAnnotations string) ([]addonv1beta1.PlacementRef, error) {
	if placementAnnotations == "" {
		return nil, nil
	}
	// Compute configs to add to each placement
	placements := strings.Split(placementAnnotations, ",")
	placementRefs := []addonv1beta1.PlacementRef{}
	for _, placement := range placements {
		nameNamespacePair := strings.Split(placement, "/")
		placementNamespace := nameNamespacePair[0]
		placementName := nameNamespacePair[1]
		placementRef := addonv1beta1.PlacementRef{
			Namespace: placementNamespace,
			Name:      placementName,
		}
		placementRefs = append(placementRefs, placementRef)
	}
	return placementRefs, nil
}

func makeAgentName(app, placement string) string {
	return fmt.Sprintf("%s-%s-%s", addoncfg.DefaultStackPrefix, app, placement)
}

func hasControllerUID(ownerRefs []metav1.OwnerReference, uid types.UID) bool {
	for _, owner := range ownerRefs {
		if owner.Controller != nil && *owner.Controller && owner.UID == uid {
			return true
		}
	}
	return false
}
