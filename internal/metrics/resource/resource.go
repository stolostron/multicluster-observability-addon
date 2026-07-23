package resource

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	errMissingHubEndpoint        = errors.New("hub endpoint is missing")
	errInvalidPlacementReference = errors.New("invalid placement reference")
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
	// Migrate existing agents from placement labels to annotations
	if err := d.migrateAgentPlacementLabels(ctx); err != nil {
		return configs, fmt.Errorf("failed to migrate agent placement labels: %w", err)
	}

	hasHostedClusters := config.HasHostedCLusters(ctx, d.Client, d.Logger)
	if d.AddonOptions.Platform.Metrics.CollectionEnabled {
		agentConfig, err := d.reconcileAgents(ctx, false)
		if err != nil {
			return configs, fmt.Errorf("failed to reconcile prometheusAgents %w", err)
		}
		configs = append(configs, agentConfig...)

		// ScrapeConfigs are common to all placements
		scConfigs, err := d.reconcileScrapeConfigs(ctx, mcoUID, false, hasHostedClusters)
		if err != nil {
			return configs, fmt.Errorf("failed to reconcile scrapeConfigs: %w", err)
		}
		configs = append(configs, scConfigs...)
	}

	if d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
		agentConfig, err := d.reconcileAgents(ctx, true)
		if err != nil {
			return configs, fmt.Errorf("failed to reconcile prometheusAgents %w", err)
		}
		configs = append(configs, agentConfig...)

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
		if !hasControllerUID(existingSC.OwnerReferences, mcoUID) && existingSC.Labels[addoncfg.PartOfK8sLabelKey] != addoncfg.Name {
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
		if !hasControllerUID(rule.OwnerReferences, mcoUID) && rule.Labels[addoncfg.PartOfK8sLabelKey] != addoncfg.Name {
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

// migrateAgentPlacementLabels migrates the placement labels to an annotation for all PrometheusAgents in the Hub install namespace.
func (d DefaultStackResources) migrateAgentPlacementLabels(ctx context.Context) error {
	promAgents := &cooprometheusv1alpha1.PrometheusAgentList{}
	if err := d.Client.List(ctx, promAgents, &client.ListOptions{
		Namespace: config.HubInstallNamespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			addoncfg.ManagedByK8sLabelKey: addoncfg.Name,
		}),
	}); err != nil {
		return fmt.Errorf("failed to list prometheusAgents for migration: %w", err)
	}

	for i := range promAgents.Items {
		agent := &promAgents.Items[i]
		if migrateAgentPlacementLabelsToAnnotation(agent) {
			if err := d.Client.Update(ctx, agent); err != nil {
				return fmt.Errorf("failed to update migrated agent %s/%s: %w", agent.Namespace, agent.Name, err)
			}
			d.Logger.Info("migrated agent placement labels to annotation", "name", agent.Name)
		}
	}

	return nil
}

// migrateAgentPlacementLabelsToAnnotation migrates the placement labels to an annotation for a given PrometheusAgent.
// If the placement labels are not present, it returns false.
// If the placement labels are present, it migrates them to an annotation and returns true.
func migrateAgentPlacementLabelsToAnnotation(agent *cooprometheusv1alpha1.PrometheusAgent) bool {
	placementName := agent.Labels[addoncfg.PlacementRefNameLabelKey]
	placementNS := agent.Labels[addoncfg.PlacementRefNamespaceLabelKey]

	if placementName == "" && placementNS == "" {
		return false
	}

	if agent.Annotations == nil {
		agent.Annotations = map[string]string{}
	}

	newRef := placementNS + "/" + placementName
	existing := agent.Annotations[addoncfg.PlacementAnnotationKey]
	if existing == "" {
		agent.Annotations[addoncfg.PlacementAnnotationKey] = newRef
	} else if !strings.Contains(existing, newRef) {
		agent.Annotations[addoncfg.PlacementAnnotationKey] = existing + "," + newRef
	}

	delete(agent.Labels, addoncfg.PlacementRefNameLabelKey)
	delete(agent.Labels, addoncfg.PlacementRefNamespaceLabelKey)

	return true
}

func (d DefaultStackResources) reconcileAgents(ctx context.Context, isUWL bool) ([]common.DefaultConfig, error) {
	_, err := d.CreateDefaultAgent(ctx, isUWL)
	if err != nil {
		return nil, fmt.Errorf("failed to create default agent: %w", err)
	}

	if d.AddonOptions.Platform.Metrics.HubEndpoint.Host == "" {
		return nil, errMissingHubEndpoint
	}

	promAgents := &cooprometheusv1alpha1.PrometheusAgentList{}
	if err := d.Client.List(ctx, promAgents, &client.ListOptions{
		Namespace:     config.HubInstallNamespace,
		LabelSelector: labels.SelectorFromSet(labels.Set(makeUWLOrPlatformLabels(isUWL))),
	}); err != nil {
		return nil, fmt.Errorf("failed to list prometheusAgents: %w", err)
	}

	configs := []common.DefaultConfig{}
	for i := range promAgents.Items {
		agent := &promAgents.Items[i]

		if !isRecognizedAgent(agent, d.CMAO.UID) {
			continue
		}

		promBuilder := PrometheusAgentSSA{
			ExistingAgent:       agent,
			IsUwl:               isUWL,
			PrometheusImage:     d.PrometheusImage,
			KubeRBACProxyImage:  d.KubeRBACProxyImage,
			RemoteWriteEndpoint: d.AddonOptions.Platform.Metrics.HubEndpoint.String(),
		}
		promSSA := promBuilder.Build()

		if !equality.Semantic.DeepDerivative(promSSA.Spec, agent.Spec) {
			if err := common.ServerSideApply(ctx, d.Client, promSSA, d.CMAO); err != nil {
				return nil, fmt.Errorf("failed to server-side apply for %s/%s: %w", promSSA.Namespace, promSSA.Name, err)
			}
		}

		placementAnnotation := agent.Annotations[addoncfg.PlacementAnnotationKey]
		placementRefs, err := d.generatePlacementRefs(placementAnnotation)
		if err != nil {
			return nil, fmt.Errorf("failed to generate placement refs for agent %s: %w", agent.Name, err)
		}

		cfg, err := common.ObjectToAddonConfig(promSSA)
		if err != nil {
			return nil, fmt.Errorf("failed to generate addon config for %s: %w", agent.Name, err)
		}

		for _, ref := range placementRefs {
			configs = append(configs, common.DefaultConfig{
				PlacementRef: ref,
				Config:       cfg,
			})
		}
	}
	return configs, nil
}

func (d DefaultStackResources) CreateDefaultAgent(ctx context.Context, isUWL bool) (*cooprometheusv1alpha1.PrometheusAgent, error) {
	promAgents := &cooprometheusv1alpha1.PrometheusAgentList{}
	if err := d.Client.List(ctx, promAgents, &client.ListOptions{
		Namespace:     config.HubInstallNamespace,
		LabelSelector: labels.SelectorFromSet(labels.Set(makeUWLOrPlatformLabels(isUWL))),
	}); err != nil {
		return nil, fmt.Errorf("failed to list existing prometheusAgents: %w", err)
	}
	globalExists := hasPlacement(d.CMAO, "global", config.HubInstallNamespace)

	appName := config.PlatformMetricsCollectorApp
	if isUWL {
		appName = config.UserWorkloadMetricsCollectorApp
	}
	globalReferenced := false

	for _, agent := range promAgents.Items {
		if !isRecognizedAgent(&agent, d.CMAO.UID) {
			continue
		}
		if agentTargetsPlacement(&agent, config.HubInstallNamespace, "dummy") {
			return nil, nil
		}
		if agentTargetsPlacement(&agent, config.HubInstallNamespace, "global") && agent.Name == makeAgentName(appName, "global")+"-default" {
			return nil, nil
		}
		if agentTargetsPlacement(&agent, config.HubInstallNamespace, "global") {
			globalReferenced = true
		}
	}
	agent := &cooprometheusv1alpha1.PrometheusAgent{}
	// If there is no global placement reference, create a dummy Prometheus Agent that points at dummy
	if !globalExists {
		agent = NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(appName, "dummy"), isUWL)
	} else if globalReferenced {
		// Global placement exists and an agent already covers it, create a dummy agent
		agent = NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(appName, "dummy"), isUWL)
	} else {
		// Global placement exists but no agent covers it, create a default agent for global
		agent = NewDefaultPrometheusAgent(config.HubInstallNamespace, makeAgentName(appName, "global")+"-default", isUWL)
	}
	placementRefName := ""
	if agent.Name == makeAgentName(appName, "global")+"-default" {
		placementRefName = "global"
	} else {
		placementRefName = "dummy"
	}

	if agent.Annotations == nil {
		agent.Annotations = map[string]string{}
	}
	agent.Annotations[addoncfg.PlacementAnnotationKey] = config.HubInstallNamespace + "/" + placementRefName

	if err := controllerutil.SetControllerReference(d.CMAO, agent, d.Client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner reference on default agent for placement %q: %w", placementRefName, err)
	}

	if err := d.Client.Create(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create the default agent for placement %q: %w", placementRefName, err)
	}
	d.Logger.Info("created default prometheus agent for placement", "agentNamespace", agent.Namespace, "agentName", agent.Name, "placementName", placementRefName)

	// Re-fetch the agent to populate server-side fields and, critically, TypeMeta.
	// The 'agent' object was mutated by Create() and its TypeMeta is now empty.
	key := client.ObjectKeyFromObject(agent)
	createdAgent := &cooprometheusv1alpha1.PrometheusAgent{}
	if err := d.Client.Get(ctx, key, createdAgent); err != nil {
		return nil, fmt.Errorf("failed to re-fetch created default agent %q: %w", key, err)
	}

	return createdAgent, nil
}

func hasPlacement(cmao *addonv1beta1.ClusterManagementAddOn, name, namespace string) bool {
	for _, p := range cmao.Spec.InstallStrategy.Placements {
		if p.Name == name && p.Namespace == namespace {
			return true
		}
	}
	return false
}

func agentTargetsPlacement(agent *cooprometheusv1alpha1.PrometheusAgent, namespace, name string) bool {
	annotation := agent.Annotations[addoncfg.PlacementAnnotationKey]
	if annotation == "" {
		return false
	}
	target := namespace + "/" + name
	for ref := range strings.SplitSeq(annotation, ",") {
		if strings.TrimSpace(ref) == target {
			return true
		}
	}
	return false
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
	placements := strings.Split(placementAnnotations, ",")
	placementRefs := []addonv1beta1.PlacementRef{}
	for _, placement := range placements {
		if placement == "" {
			continue
		}
		nameNamespacePair := strings.SplitN(placement, "/", 2)
		if len(nameNamespacePair) != 2 || nameNamespacePair[0] == "" || nameNamespacePair[1] == "" {
			return nil, fmt.Errorf("%w %q: expected format namespace/name", errInvalidPlacementReference, placement)
		}
		ref := addonv1beta1.PlacementRef{
			Namespace: nameNamespacePair[0],
			Name:      nameNamespacePair[1],
		}
		if slices.Contains(placementRefs, ref) {
			continue
		}
		placementRefs = append(placementRefs, ref)
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

// isRecognizedAgent returns true if the agent is either owned by the CMAO (default agents)
// or has the part-of label indicating it's a user-defined agent that opts into MCOA management.
func isRecognizedAgent(agent *cooprometheusv1alpha1.PrometheusAgent, cmaoUID types.UID) bool {
	if hasControllerUID(agent.OwnerReferences, cmaoUID) {
		return true
	}
	return agent.Labels[addoncfg.PartOfK8sLabelKey] == addoncfg.Name
}
