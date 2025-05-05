package resource

import (
	"context"
	"fmt"
	"maps"
	"slices"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DefaultStackResources struct {
	Client       client.Client
	CMAO         *addonv1alpha1.ClusterManagementAddOn
	AddonOptions addon.Options
	Logger       klog.Logger
}

type defaultConfig struct {
	placementRef addonv1alpha1.PlacementRef
	config       addonv1alpha1.AddOnConfig
}

// Reconcile creates resources for the default logging stack
// based on the provided options and placement information.
func (d DefaultStackResources) Reconcile(ctx context.Context) error {
	configs := []defaultConfig{}

	// Reconcile existing placements
	for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
		if d.AddonOptions.Platform.Metrics.CollectionEnabled {
			agent, err := d.reconcilePlatformAgent(ctx, placement.PlacementRef)
			if err != nil {
				return fmt.Errorf("failed to reconcile platform prometheusAgent for placement %s: %w", placement.Name, err)
			}
			configs = append(configs, defaultConfig{
				placementRef: placement.PlacementRef,
				config:       promAgentToAddonConfig(agent),
			})
		}

		if d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
			if err := d.reconcileUWLAgent(ctx, placement.PlacementRef); err != nil {
				return fmt.Errorf("failed to reconcile user workloads prometheusAgent for placement %s: %w", placement.Name, err)
			}
		}
	}

	// Ensure that configs are referenced in the ClusterManagementAddon
	if err := d.ensureAddonConfig(ctx, configs); err != nil {
		return fmt.Errorf("failed to ensure addon configs: %w", err)
	}

	// Clean default configs from removed placements
	if err := d.cleanOrphanResources(ctx); err != nil {
		return fmt.Errorf("failed to clean orphan resources: %w", err)
	}
	return nil
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

func (d DefaultStackResources) cleanOrphanResources(ctx context.Context) error {
	// list prometheus agents owned by cmao
	// remove the ones having a non existing placement
	items := prometheusalpha1.PrometheusAgentList{}
	if err := d.Client.List(ctx, &items); err != nil {
		return fmt.Errorf("")
	}

	makePlacementKey := func(namespace, name string) string {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	placementsDict := map[string]struct{}{}
	for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
		placementsDict[makePlacementKey(placement.Namespace, placement.Name)] = struct{}{}
	}

	for _, agent := range items.Items {
		hasOwnerRef, err := controllerutil.HasOwnerReference(agent.GetOwnerReferences(), d.CMAO, d.Client.Scheme())
		if err != nil {
			return fmt.Errorf("failed to check owner references: %w", err)
		}

		if !hasOwnerRef {
			continue
		}

		placementNs := agent.Labels[config.PlacementRefNamespaceLabelKey]
		placementName := agent.Labels[config.PlacementRefNameLabelKey]
		if _, ok := placementsDict[makePlacementKey(placementNs, placementName)]; ok {
			continue
		}

		if err := d.Client.Delete(ctx, &agent); err != nil {
			return fmt.Errorf("failed to delete owned agent: %w", err)
		}
	}

	return nil
}

func (d DefaultStackResources) ensureAddonConfig(ctx context.Context, configs []defaultConfig) error {
	// with retry
	// get and check/add config
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cmao := &addonv1alpha1.ClusterManagementAddOn{}
		if err := d.Client.Get(ctx, types.NamespacedName{Name: d.CMAO.Name}, cmao); err != nil {
			return fmt.Errorf("failed to get ClusterManagementAddOn: %w", err)
		}

		if updated := ensureConfigsInAddon(cmao, configs); !updated {
			return nil
		}

		return d.Client.Update(ctx, cmao)
	})

	if retryErr != nil {
		return fmt.Errorf("failed to update CMAO with default configs: %w", retryErr)
	}

	return nil
}

func ensureConfigsInAddon(cmao *addonv1alpha1.ClusterManagementAddOn, configs []defaultConfig) bool {
	var updated bool
	// group configs by placement
	placementConfigs := map[addonv1alpha1.PlacementRef][]addonv1alpha1.AddOnConfig{}
	for _, cfg := range configs {
		placementConfigs[cfg.placementRef] = append(placementConfigs[cfg.placementRef], cfg.config)
	}

	// for each placement in cmao, ensure configs are present
	for _, placement := range cmao.Spec.InstallStrategy.Placements {
		desiredConfigs := placementConfigs[placement.PlacementRef]
		for _, cfg := range desiredConfigs {
			idx := slices.IndexFunc(placement.Configs, func(e addonv1alpha1.AddOnConfig) bool {
				return e == cfg
			})

			if idx >= 0 {
				continue
			}

			updated = true
			placement.Configs = append(placement.Configs, cfg)
		}
	}

	return updated
}

// type defaultAgentReconciler struct {
// 	Client client.Client

// }

func (d DefaultStackResources) reconcilePlatformAgent(ctx context.Context, placementRef addonv1alpha1.PlacementRef) (*prometheusalpha1.PrometheusAgent, error) {
	// Get or create default
	platformAgent, err := d.getOrCreateDefaultPlatformAgent(ctx, placementRef.Name)
	if err != nil {
		return platformAgent, fmt.Errorf("failed to get or create platform agent for placement %s: %w", placementRef.Name, err)
	}

	// SSA mendatory field values
	promBuilder := PrometheusAgentBuilder{
		Agent: &prometheusalpha1.PrometheusAgent{ObjectMeta: metav1.ObjectMeta{
			Name:      platformAgent.Name,
			Namespace: platformAgent.Namespace,
			Labels:    maps.Clone(platformAgent.Labels),
		}},
		SAName:              config.PlatformMetricsCollectorApp,
		MatchLabels:         map[string]string{"app": config.PlatformMetricsCollectorApp},
		RemoteWriteEndpoint: d.AddonOptions.Platform.Metrics.HubEndpoint.String(),
	}
	promSSA := promBuilder.Build()
	if promSSA.Labels == nil {
		promSSA.Labels = map[string]string{}
	}
	promSSA.Labels[config.PlacementRefNameLabelKey] = placementRef.Name
	promSSA.Labels[config.PlacementRefNamespaceLabelKey] = placementRef.Namespace

	//SSA the objects rendered
	if !equality.Semantic.DeepDerivative(promSSA, platformAgent) {
		if err := common.ServerSideApply(ctx, d.Client, promSSA, d.CMAO); err != nil {
			return platformAgent, fmt.Errorf("failed to server-side apply for %s/%s: %w", promSSA.Namespace, promSSA.Name, err)
		}
		d.Logger.Info("updated prometheus agent %s/%s with server-side apply", promSSA.Namespace, promSSA.Name)
	}

	return platformAgent, nil
}

func (d DefaultStackResources) getOrCreateDefaultPlatformAgent(ctx context.Context, placementName string) (*prometheusalpha1.PrometheusAgent, error) {
	platformAgent := &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", addon.DefaultStackPrefix, config.PlatformMetricsCollectorApp, placementName),
			Namespace: config.HubInstallNamespace,
		},
	}
	if err := d.Client.Get(ctx, client.ObjectKeyFromObject(platformAgent), platformAgent); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get default platform agent for placement %q: %w", placementName, err)
		}

		// Create default resource
		platformAgent = NewDefaultPlatformPrometheusAgent(platformAgent.Namespace, platformAgent.Name)
		if err := d.Client.Create(ctx, platformAgent); err != nil {
			return nil, fmt.Errorf("failed to create the default platform agent for placement %q: %w", placementName, err)
		}
		d.Logger.Info("created default prometheus agent %s/%s for placement %s", platformAgent.Namespace, platformAgent.Name, placementName)
	}

	return platformAgent, nil
}

func (d DefaultStackResources) reconcileUWLAgent(ctx context.Context, placementRef addonv1alpha1.PlacementRef) error {
	// Get or create default
	platformAgent, err := d.getOrCreateDefaultUWLAgent(ctx, placementRef.Name)
	if err != nil {
		return fmt.Errorf("failed to get or create user workloads agent for placement %s: %w", placementRef.Name, err)
	}

	// SSA mendatory field values
	promBuilder := PrometheusAgentBuilder{
		Agent: &prometheusalpha1.PrometheusAgent{ObjectMeta: metav1.ObjectMeta{
			Name:      platformAgent.Name,
			Namespace: platformAgent.Namespace,
		}},
		SAName:              config.UserWorkloadMetricsCollectorApp,
		IsUwl:               true,
		MatchLabels:         map[string]string{"app": config.UserWorkloadMetricsCollectorApp},
		RemoteWriteEndpoint: d.AddonOptions.Platform.Metrics.HubEndpoint.String(),
	}
	promSSA := promBuilder.Build()
	if promSSA.Labels == nil {
		promSSA.Labels = map[string]string{}
	}
	promSSA.Labels[config.PlacementRefNameLabelKey] = placementRef.Name
	promSSA.Labels[config.PlacementRefNamespaceLabelKey] = placementRef.Namespace

	//SSA the objects rendered
	if err := common.ServerSideApply(ctx, d.Client, promSSA, d.CMAO); err != nil {
		return fmt.Errorf("failed to server-side apply for %s/%s: %w", promSSA.Namespace, promSSA.Name, err)
	}

	return nil
}

func (d DefaultStackResources) getOrCreateDefaultUWLAgent(ctx context.Context, placementName string) (*prometheusalpha1.PrometheusAgent, error) {
	platformAgent := &prometheusalpha1.PrometheusAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", addon.DefaultStackPrefix, config.UserWorkloadMetricsCollectorApp, placementName),
			Namespace: config.HubInstallNamespace,
		},
	}
	if err := d.Client.Get(ctx, client.ObjectKeyFromObject(platformAgent), platformAgent); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get default user workloads agent for placement %q: %w", placementName, err)
		}

		// Create default resource
		platformAgent = NewDefaultUserWorkloadsPrometheusAgent(platformAgent.Namespace, platformAgent.Name)
		if err := d.Client.Create(ctx, platformAgent); err != nil {
			return nil, fmt.Errorf("failed to create the default user workloads agent for placement %q: %w", placementName, err)
		}
	}

	return platformAgent, nil
}
