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
	if controller := metav1.GetControllerOfNoCopy(d.CMAO); controller != nil {
		mcoUID = controller.UID
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

	mco := &metav1.ObjectMeta{UID: mcoUID}
	mcoManagedScrapeConfigs := []client.Object{}
	userDefinedScrapeConfigs := []client.Object{}
	for _, existingSC := range scrapeConfigsList.Items {
		// Ensures that we only filter for MCO-managed scrape configs or user-defined scrape configs that have at least one of these labels along with the required annotation for user-defined scrape configs
		if !metav1.IsControlledBy(&existingSC, mco) && existingSC.Labels[addoncfg.PartOfK8sLabelKey] != addoncfg.Name {
			continue
		}

		desiredSC := existingSC.DeepCopy()
		desiredSC.ManagedFields = nil // required for patching with ssa
		applyScrapeConfigSSAInvariants(desiredSC, isUWL)
		common.SetSSAManagedFieldsAnnotation(desiredSC, common.DeriveSSAManagedFields(scrapeConfigSSAIntent(isUWL)))

		// SSA the objects rendered
		if !equality.Semantic.DeepDerivative(desiredSC.Spec, existingSC.Spec) ||
			!equality.Semantic.DeepDerivative(desiredSC.Labels, existingSC.Labels) ||
			common.SSAManagedFieldsAnnotation(desiredSC) != common.SSAManagedFieldsAnnotation(&existingSC) {
			if err = common.ServerSideApply(ctx, d.Client, desiredSC, nil); err != nil { // object is controlled by MCO, no owner
				return nil, fmt.Errorf("failed to patch with with server-side apply: %w", err)
			}
			d.Logger.Info("updated scrapeConfig with server-side apply", "namespace", desiredSC.Namespace, "name", desiredSC.Name)
		}
		if metav1.IsControlledBy(&existingSC, mco) {
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
		placementRefs, err := generatePlacementRefs(placementAnnotations)
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

	mco := &metav1.ObjectMeta{UID: mcoUID}
	mcoManagedRules := []client.Object{}
	userDefinedRules := []client.Object{}
	for _, rule := range promRuleList.Items {
		if !metav1.IsControlledBy(&rule, mco) && rule.Labels[addoncfg.PartOfK8sLabelKey] != addoncfg.Name {
			continue
		}

		if metav1.IsControlledBy(&rule, mco) {
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
		placementRefs, err := generatePlacementRefs(placementAnnotations)
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
			if err := d.Client.Update(ctx, agent, client.FieldOwner(addoncfg.Name)); err != nil {
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

	if placementName == "" || placementNS == "" {
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
		LabelSelector: labels.SelectorFromSet(labels.Set(makeComponentLabelSelector(isUWL))),
	}); err != nil {
		return nil, fmt.Errorf("failed to list prometheusAgents: %w", err)
	}

	configs := []common.DefaultConfig{}
	for i := range promAgents.Items {
		agent := &promAgents.Items[i]

		if !isRecognizedAgent(agent, d.CMAO) {
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

		if !equality.Semantic.DeepDerivative(promSSA.Spec, agent.Spec) ||
			common.SSAManagedFieldsAnnotation(promSSA) != common.SSAManagedFieldsAnnotation(agent) {
			var owner client.Object
			if metav1.IsControlledBy(agent, d.CMAO) {
				owner = d.CMAO
			}
			if err := common.ServerSideApply(ctx, d.Client, promSSA, owner); err != nil {
				return nil, fmt.Errorf("failed to server-side apply for %s/%s: %w", promSSA.Namespace, promSSA.Name, err)
			}
		}

		placementAnnotation := agent.Annotations[addoncfg.PlacementAnnotationKey]
		placementRefs, err := generatePlacementRefs(placementAnnotation)
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

func (d DefaultStackResources) CreateDefaultAgent(ctx context.Context, isUWL bool) ([]*cooprometheusv1alpha1.PrometheusAgent, error) {
	allPromAgents := &cooprometheusv1alpha1.PrometheusAgentList{}
	if err := d.Client.List(ctx, allPromAgents, client.InNamespace(config.HubInstallNamespace)); err != nil {
		return nil, fmt.Errorf("failed to list existing prometheusAgents: %w", err)
	}

	idx, err := newAgentIndex(d.CMAO, allPromAgents.Items, isUWL)
	if err != nil {
		return nil, err
	}

	var createdAgents []*cooprometheusv1alpha1.PrometheusAgent
	for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
		cmaoAgents := idx.cmaoOwnedAgentsForPlacement(placement.PlacementRef)

		if idx.isPlacementCoveredByUser(placement.PlacementRef) {
			if err := d.deleteShadowedAgents(ctx, cmaoAgents, placement.Name); err != nil {
				return nil, err
			}
			continue
		}

		if len(cmaoAgents) > 0 {
			continue
		}

		agent, err := d.createDefaultAgentForPlacement(ctx, idx, placement.PlacementRef, isUWL)
		if err != nil {
			return nil, err
		}
		createdAgents = append(createdAgents, agent)
	}

	return createdAgents, nil
}

func (d DefaultStackResources) deleteShadowedAgents(ctx context.Context, cmaoAgents []*cooprometheusv1alpha1.PrometheusAgent, placementName string) error {
	for _, cmaoAgent := range cmaoAgents {
		if err := d.Client.Delete(ctx, cmaoAgent); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete shadowed default agent %s/%s: %w", cmaoAgent.Namespace, cmaoAgent.Name, err)
		}
		d.Logger.Info("deleted default prometheus agent shadowed by user agent", "agentNamespace", cmaoAgent.Namespace, "agentName", cmaoAgent.Name, "placementName", placementName)
	}
	return nil
}

func (d DefaultStackResources) createDefaultAgentForPlacement(ctx context.Context, idx agentIndex, placement addonv1beta1.PlacementRef, isUWL bool) (*cooprometheusv1alpha1.PrometheusAgent, error) {
	defaultAgentName := makeAgentName(idx.appName, placement.Name)
	if stale, ok := idx.cmaoOwnedAgentsByName[defaultAgentName]; ok {
		if err := d.Client.Delete(ctx, stale); client.IgnoreNotFound(err) != nil {
			return nil, fmt.Errorf("failed to delete stale default agent %s/%s: %w", stale.Namespace, stale.Name, err)
		}
	}

	agent := NewDefaultPrometheusAgent(config.HubInstallNamespace, defaultAgentName, isUWL)
	if agent.Annotations == nil {
		agent.Annotations = map[string]string{}
	}
	agent.Annotations[addoncfg.PlacementAnnotationKey] = placement.Namespace + "/" + placement.Name

	if err := controllerutil.SetControllerReference(d.CMAO, agent, d.Client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner reference on default agent for placement %q: %w", placement.Name, err)
	}

	if err := d.Client.Create(ctx, agent, client.FieldOwner(addoncfg.Name)); err != nil {
		return nil, fmt.Errorf("failed to create the default agent for placement %q: %w", placement.Name, err)
	}
	d.Logger.Info("created default prometheus agent for placement", "agentNamespace", agent.Namespace, "agentName", agent.Name, "placementName", placement.Name)

	return agent, nil
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

func generatePlacementRefs(placementAnnotations string) ([]addonv1beta1.PlacementRef, error) {
	if placementAnnotations == "" {
		return nil, nil
	}
	placements := strings.Split(placementAnnotations, ",")
	placementRefs := []addonv1beta1.PlacementRef{}
	for _, placement := range placements {
		placement = strings.TrimSpace(placement)
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

// scrapeConfigSSAIntent returns a sparse object containing only the fields MCOA
// enforces on ScrapeConfigs. DeriveSSAManagedFields walks this object so the
// annotation stays in sync with applyScrapeConfigSSAInvariants.
func scrapeConfigSSAIntent(isUWL bool) *cooprometheusv1alpha1.ScrapeConfig {
	intent := &cooprometheusv1alpha1.ScrapeConfig{}
	applyScrapeConfigSSAInvariants(intent, isUWL)
	return intent
}

func applyScrapeConfigSSAInvariants(sc *cooprometheusv1alpha1.ScrapeConfig, isUWL bool) {
	if sc.Labels == nil {
		sc.Labels = map[string]string{}
	}
	sc.Labels[addoncfg.BackupLabelKey] = addoncfg.BackupLabelValue

	if isUWL {
		return
	}

	// Enforce empty values, they are set when generating the manifests for a given managedCluster
	sc.Spec.ScrapeClassName = ptr.To("not-configurable")
	sc.Spec.Scheme = ptr.To(cooprometheusv1.Scheme("HTTPS"))
	sc.Spec.StaticConfigs = []cooprometheusv1alpha1.StaticConfig{
		{
			Targets: []cooprometheusv1alpha1.Target{
				"not-configurable",
			},
		},
	}
}

// isRecognizedAgent returns true if the agent is either owned by the CMAO (default agents),
// has the part-of label indicating it's a user-defined agent that opts into MCOA management,
// or is explicitly referenced in any placement's Configs on the CMAO.
func isRecognizedAgent(agent *cooprometheusv1alpha1.PrometheusAgent, cmao *addonv1beta1.ClusterManagementAddOn) bool {
	if cmao != nil && metav1.IsControlledBy(agent, cmao) {
		return true
	}
	if agent.Labels[addoncfg.PartOfK8sLabelKey] == addoncfg.Name {
		return true
	}
	return isReferencedInPlacements(agent, cmao)
}

func isReferencedInPlacements(agent *cooprometheusv1alpha1.PrometheusAgent, cmao *addonv1beta1.ClusterManagementAddOn) bool {
	if cmao == nil {
		return false
	}
	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		for _, cfg := range placement.Configs {
			if isPrometheusAgentConfig(cfg) && cfg.Name == agent.Name && cfg.Namespace == agent.Namespace {
				return true
			}
		}
	}
	return false
}
