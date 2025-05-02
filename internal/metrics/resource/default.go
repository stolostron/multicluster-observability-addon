package resource

import (
	"context"
	"fmt"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DefaultStackResources struct {
	Client       client.Client
	CMAO         *addonv1alpha1.ClusterManagementAddOn
	AddonOptions addon.Options
	Logger       klog.Logger
}

// Reconcile creates resources for the default logging stack
// based on the provided options and placement information.
func (d DefaultStackResources) Reconcile(ctx context.Context) error {
	// Reconcile existing placements
	// TODO:  add cleaning of removed placements
	for _, placement := range d.CMAO.Spec.InstallStrategy.Placements {
		if d.AddonOptions.Platform.Metrics.CollectionEnabled {
			if err := d.reconcilePlatformAgent(ctx, placement.PlacementRef); err != nil {
				return fmt.Errorf("failed to reconcile platform prometheusAgent for placement %s: %w", placement.Name, err)
			}
		}
		if d.AddonOptions.UserWorkloads.Metrics.CollectionEnabled {
			if err := d.reconcileUWLAgent(ctx, placement.PlacementRef); err != nil {
				return fmt.Errorf("failed to reconcile user workloads prometheusAgent for placement %s: %w", placement.Name, err)
			}
		}
	}

	return nil
}

func (d DefaultStackResources) reconcilePlatformAgent(ctx context.Context, placementRef addonv1alpha1.PlacementRef) error {
	// Get or create default
	platformAgent, err := d.getOrCreateDefaultPlatformAgent(ctx, placementRef.Name)
	if err != nil {
		return fmt.Errorf("failed to get or create platform agent for placement %s: %w", placementRef.Name, err)
	}

	// SSA mendatory field values
	promBuilder := PrometheusAgentBuilder{
		Agent: &prometheusalpha1.PrometheusAgent{ObjectMeta: metav1.ObjectMeta{
			Name:      platformAgent.Name,
			Namespace: platformAgent.Namespace,
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
	if err := common.ServerSideApply(ctx, d.Client, promSSA, d.CMAO); err != nil {
		return fmt.Errorf("failed to server-side apply for %s/%s: %w", promSSA.Namespace, promSSA.Name, err)
	}

	return nil
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
	// TODO: add default stack label and placement name or id label to be able to find it from MCO and add it to the config list
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

func makePlacementRef(placementRef addonv1alpha1.PlacementRef) string {
	return fmt.Sprintf("%s.%s", placementRef.Namespace, placementRef.Name)
}
