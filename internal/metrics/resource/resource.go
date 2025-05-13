package resource

import (
	"context"
	"errors"
	"fmt"
	"slices"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var errTooManyConfigResources = errors.New("too many configuration resources")

// DefaultStackResources reconciles the configuration resources needed for metrics collection
type DefaultStackResources struct {
	AddonOptions    addon.Options
	Client          client.Client
	CMAO            *addonv1alpha1.ClusterManagementAddOn
	Logger          klog.Logger
	PrometheusImage string
}

type defaultConfig struct {
	placementRef addonv1alpha1.PlacementRef
	config       addonv1alpha1.AddOnConfig
}

// Reconcile ensures the state of the configuration resources for metrics collection.
// For each placement found in the ClusterManagementAddon resource, it generates a default PrometheusAgent
// if not found and then applies configuration invariants using server-side apply.
func (d DefaultStackResources) Reconcile(ctx context.Context) error {
	configs := []defaultConfig{}

	// Reconcile existing placements.
	for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
		if d.AddonOptions.Platform.Metrics.CollectionEnabled {
			agent, err := d.reconcileAgent(ctx, placement.PlacementRef, false)
			if err != nil {
				return fmt.Errorf("failed to reconcile platform prometheusAgent for placement %s: %w", placement.Name, err)
			}
			configs = append(configs, defaultConfig{
				placementRef: placement.PlacementRef,
				config:       promAgentToAddonConfig(agent),
			})
		}

		if d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
			agent, err := d.reconcileAgent(ctx, placement.PlacementRef, true)
			if err != nil {
				return fmt.Errorf("failed to reconcile user workloads prometheusAgent for placement %s: %w", placement.Name, err)
			}
			configs = append(configs, defaultConfig{
				placementRef: placement.PlacementRef,
				config:       promAgentToAddonConfig(agent),
			})
		}
	}

	// Ensure that configs are referenced in the ClusterManagementAddon.
	if err := d.ensureAddonConfig(ctx, configs); err != nil {
		return fmt.Errorf("failed to ensure addon configs: %w", err)
	}

	// Clean default configs from removed placements.
	if err := common.CleanOrphanResources(ctx, d.Client, d.CMAO, &prometheusalpha1.PrometheusAgentList{}); err != nil {
		return fmt.Errorf("failed to clean orphan resources: %w", err)
	}
	return nil
}

// ensureAddonConfig ensures that the provided configuration are present in the CMAO
// for each placement.
func (d DefaultStackResources) ensureAddonConfig(ctx context.Context, configs []defaultConfig) error {
	// CMAO is a shared object, using retry
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cmao := &addonv1alpha1.ClusterManagementAddOn{}
		if err := d.Client.Get(ctx, types.NamespacedName{Name: d.CMAO.Name}, cmao); err != nil {
			return fmt.Errorf("failed to get ClusterManagementAddOn: %w", err)
		}

		desiredCmao := cmao.DeepCopy()
		ensureConfigsInAddon(desiredCmao, configs)
		if equality.Semantic.DeepEqual(cmao, desiredCmao) {
			return nil
		}

		err := d.Client.Update(ctx, desiredCmao)
		if err == nil {
			d.Logger.Info("addon config updated with default configs")
		}
		return err
	})

	if retryErr != nil {
		return fmt.Errorf("failed to update CMAO with default configs: %w", retryErr)
	}

	return nil
}

func (d DefaultStackResources) reconcileAgent(ctx context.Context, placementRef addonv1alpha1.PlacementRef, isUWL bool) (*prometheusalpha1.PrometheusAgent, error) {
	// Get or create default
	agent, err := d.getOrCreateDefaultAgent(ctx, placementRef, isUWL)
	if err != nil {
		return agent, fmt.Errorf("failed to get or create agent for placement %s: %w", placementRef.Name, err)
	}

	// SSA mendatory field values
	promBuilder := PrometheusAgentSSA{
		ExistingAgent:       agent,
		IsUwl:               isUWL,
		RemoteWriteEndpoint: d.AddonOptions.Platform.Metrics.HubEndpoint.String(),
		// Commented while the stolostron build of prometheus is not based on v3 as it requires support for the --agent flag.
		// PrometheusImage:     d.PrometheusImage,
		Labels: map[string]string{
			addon.PlacementRefNameLabelKey:      placementRef.Name,
			addon.PlacementRefNamespaceLabelKey: placementRef.Namespace,
		},
	}
	promSSA := promBuilder.Build()

	// SSA the objects rendered
	if !equality.Semantic.DeepDerivative(promSSA, agent) {
		if err := common.ServerSideApply(ctx, d.Client, promSSA, d.CMAO); err != nil {
			return agent, fmt.Errorf("failed to server-side apply for %s/%s: %w", promSSA.Namespace, promSSA.Name, err)
		}
		d.Logger.Info("updated prometheus agent with server-side apply", "namespace", promSSA.Namespace, "name", promSSA.Name)
	}

	return agent, nil
}

func (d DefaultStackResources) getOrCreateDefaultAgent(ctx context.Context, placementRef addonv1alpha1.PlacementRef, isUWL bool) (*prometheusalpha1.PrometheusAgent, error) {
	promAgents := &prometheusalpha1.PrometheusAgentList{}
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

	return agent, nil
}

func makeAgentName(app, placement string) string {
	return fmt.Sprintf("%s-%s-%s", addon.DefaultStackPrefix, app, placement)
}

func promAgentToAddonConfig(agent *prometheusalpha1.PrometheusAgent) addonv1alpha1.AddOnConfig {
	return addonv1alpha1.AddOnConfig{
		ConfigReferent: addonv1alpha1.ConfigReferent{
			Namespace: agent.Namespace,
			Name:      agent.Name,
		},
		ConfigGroupResource: addonv1alpha1.ConfigGroupResource{
			Group:    prometheusalpha1.SchemeGroupVersion.Group,
			Resource: prometheusalpha1.PrometheusAgentName,
		},
	}
}

func ensureConfigsInAddon(cmao *addonv1alpha1.ClusterManagementAddOn, configs []defaultConfig) {
	// Group configs by placement.
	placementConfigs := map[addonv1alpha1.PlacementRef][]addonv1alpha1.AddOnConfig{}
	for _, cfg := range configs {
		placementConfigs[cfg.placementRef] = append(placementConfigs[cfg.placementRef], cfg.config)
	}

	// For each placement in CMAO, ensure configs are present.
	for i, placement := range cmao.Spec.InstallStrategy.Placements {
		desiredConfigs := placementConfigs[placement.PlacementRef]
		for _, cfg := range desiredConfigs {
			isPresent := slices.ContainsFunc(placement.Configs, func(e addonv1alpha1.AddOnConfig) bool {
				return e == cfg
			})

			if isPresent {
				continue
			}

			cmao.Spec.InstallStrategy.Placements[i].Configs = append(cmao.Spec.InstallStrategy.Placements[i].Configs, cfg)
		}
	}
}
