package handlers

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errInvalidConfigResourcesCount = errors.New("invalid number of configuration resources")
	errUnsupportedAppName          = errors.New("unsupported app name")
	errMissingDesiredConfig        = errors.New("missing desiredConfig in managedClusterAddon.Status.ConfigReferences")
	errMissingRemoteWriteConfig    = errors.New("missing expected remote write spec in the prometheusAgent")
	errMissingCMAOOwnership        = errors.New("object is not owned by the ClusterManagementAddOn")
)

type OptionsBuilder struct {
	Client         client.Client
	RemoteWriteURL string
	Logger         logr.Logger
}

func (o *OptionsBuilder) Build(ctx context.Context, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, managedCluster *clusterv1.ManagedCluster, platform, userWorkloads addon.MetricsOptions) (Options, error) {
	ret := Options{}

	if !platform.CollectionEnabled && !userWorkloads.CollectionEnabled {
		return ret, nil
	}

	ret.ClusterName = managedCluster.Name
	ret.ClusterID = managedCluster.Labels[config.ManagedClusterLabelClusterID]
	if ret.ClusterID == "" {
		ret.ClusterID = managedCluster.Name
	}

	// Fetch image overrides
	var err error
	ret.Images, err = config.GetImageOverrides(ctx, o.Client)
	if err != nil {
		return ret, fmt.Errorf("failed to get image overrides: %w", err)
	}

	// Fetch configuration references
	configResources, err := o.getAvailableConfigResources(ctx, mcAddon)
	if err != nil {
		return ret, fmt.Errorf("failed to get configuration resources: %w", err)
	}

	// Build Prometheus agents for platform and user workloads
	if platform.CollectionEnabled {
		if err := o.buildPrometheusAgent(ctx, &ret, configResources, config.PlatformMetricsCollectorApp, false); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.Platform.ScrapeConfigs = common.FilterResourcesByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, config.PlatformPrometheusMatchLabels)
		if len(ret.Platform.ScrapeConfigs) == 0 {
			o.Logger.V(1).Info("No scrape configs found for platform metrics")
		}
		ret.Platform.Rules = common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.PlatformPrometheusMatchLabels)
		if len(ret.Platform.Rules) == 0 {
			o.Logger.V(1).Info("No rules found for platform metrics")
		}
	}

	isHypershiftCluster := IsHypershiftEnabled(managedCluster)

	if userWorkloads.CollectionEnabled {
		if err := o.buildPrometheusAgent(ctx, &ret, configResources, config.UserWorkloadMetricsCollectorApp, isHypershiftCluster); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.UserWorkloads.ScrapeConfigs = common.FilterResourcesByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, config.UserWorkloadPrometheusMatchLabels)
		if len(ret.UserWorkloads.ScrapeConfigs) == 0 {
			o.Logger.V(1).Info("No scrape configs found for user workloads")
		}
		ret.UserWorkloads.Rules = common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.UserWorkloadPrometheusMatchLabels)
		if len(ret.UserWorkloads.Rules) == 0 {
			o.Logger.V(1).Info("No rules found for user workloads")
		}
	}

	if isHypershiftCluster {
		if userWorkloads.CollectionEnabled {
			if err := o.buildHypershiftResources(ctx, &ret, managedCluster, configResources); err != nil {
				return ret, fmt.Errorf("failed to generate hypershift resources: %w", err)
			}
		} else {
			o.Logger.Info("User workload monitoring is needed to monitor Hosted Control Planes managed by the hypershift addon. Ignoring related resources creation.")
		}
	}

	return ret, nil
}

// buildPrometheusAgent abstracts the logic of building a Prometheus agent for platform or user workloads
func (o *OptionsBuilder) buildPrometheusAgent(ctx context.Context, opts *Options, configResources []client.Object, appName string, isHypershift bool) error {
	// Fetch Prometheus agent resource
	labelsMatcher := config.PlatformPrometheusMatchLabels
	if appName == config.UserWorkloadMetricsCollectorApp {
		labelsMatcher = config.UserWorkloadPrometheusMatchLabels
	}
	platformAgents := common.FilterResourcesByLabelSelector[*prometheusalpha1.PrometheusAgent](configResources, labelsMatcher)
	if len(platformAgents) != 1 {
		return fmt.Errorf("%w: for application %s, found %d agents with labels %+v", errInvalidConfigResourcesCount, appName, len(platformAgents), labelsMatcher)
	}
	agent := platformAgents[0]

	isOwned, err := common.HasCMAOOwnerReference(ctx, o.Client, agent)
	if err != nil {
		return fmt.Errorf("failed to check owner reference of the Prometheus Agent %s/%s: %w", agent.Namespace, agent.Name, err)
	}
	if !isOwned {
		return fmt.Errorf("%w: kind %s %s/%s", errMissingCMAOOwnership, agent.Kind, agent.Namespace, agent.Name)
	}

	// add the relabel cfg
	remoteWriteSpecIdx := slices.IndexFunc(agent.Spec.RemoteWrite, func(e prometheusv1.RemoteWriteSpec) bool {
		return e.Name != nil && *e.Name == config.RemoteWriteCfgName
	})
	if remoteWriteSpecIdx == -1 {
		return fmt.Errorf("%w: failed to get the %q remote write spec in agent %s/%s", errMissingRemoteWriteConfig, config.RemoteWriteCfgName, agent.Namespace, agent.Name)
	}
	agent.Spec.RemoteWrite[remoteWriteSpecIdx].WriteRelabelConfigs = createWriteRelabelConfigs(opts.ClusterName, opts.ClusterID, isHypershift)

	// Set the built agent in the appropriate workload option
	switch appName {
	case config.PlatformMetricsCollectorApp:
		opts.Platform.PrometheusAgent = agent
	case config.UserWorkloadMetricsCollectorApp:
		opts.UserWorkloads.PrometheusAgent = agent
	default:
		return fmt.Errorf("%w: %s", errUnsupportedAppName, appName)
	}

	// Fetch related secrets
	for _, secretName := range agent.Spec.Secrets {
		if err := o.addSecret(ctx, &opts.Secrets, secretName, agent.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func (o *OptionsBuilder) buildHypershiftResources(ctx context.Context, opts *Options, managedCluster *clusterv1.ManagedCluster, configResources []client.Object) error {
	etcdScrapeConfigs := common.FilterResourcesByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, config.EtcdHcpUserWorkloadPrometheusMatchLabels)
	etcdRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.EtcdHcpUserWorkloadPrometheusMatchLabels)
	apiserverScrapeConfigs := common.FilterResourcesByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, config.ApiserverHcpUserWorkloadPrometheusMatchLabels)
	apiserverRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.ApiserverHcpUserWorkloadPrometheusMatchLabels)

	if len(etcdScrapeConfigs) == 0 {
		o.Logger.V(1).Info("no scrapeConfigs found in configuration resources for etcd HPCs", "expectedLabel", fmt.Sprintf("%+v", config.EtcdHcpUserWorkloadPrometheusMatchLabels))
	}

	if len(apiserverScrapeConfigs) == 0 {
		o.Logger.V(1).Info("no scrapeConfigs found in configuration resources for apiserver HPCs", "expectedLabel", fmt.Sprintf("%+v", config.ApiserverHcpUserWorkloadPrometheusMatchLabels))
	}

	hyper := Hypershift{
		Client:         o.Client,
		ManagedCluster: managedCluster,
		Logger:         o.Logger,
	}

	hyperResources, err := hyper.GenerateResources(ctx,
		CollectionConfig{ScrapeConfigs: etcdScrapeConfigs, Rules: etcdRules},
		CollectionConfig{ScrapeConfigs: apiserverScrapeConfigs, Rules: apiserverRules},
	)
	if err != nil {
		return fmt.Errorf("failed to generate hypershift resources: %w", err)
	}

	opts.UserWorkloads.ScrapeConfigs = append(opts.UserWorkloads.ScrapeConfigs, hyperResources.ScrapeConfigs...)
	opts.UserWorkloads.Rules = append(opts.UserWorkloads.Rules, hyperResources.Rules...)
	opts.UserWorkloads.ServiceMonitors = append(opts.UserWorkloads.ServiceMonitors, hyperResources.ServiceMonitors...)
	return nil
}

// Simplified addSecret function (unchanged)
func (o *OptionsBuilder) addSecret(ctx context.Context, secrets *[]*corev1.Secret, secretName, secretNamespace string) error {
	if slices.IndexFunc(*secrets, func(s *corev1.Secret) bool { return s.Name == secretName && s.Namespace == secretNamespace }) != -1 {
		return nil
	}

	secret := &corev1.Secret{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, secret); err != nil {
		return fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, secretNamespace, err)
	}

	*secrets = append(*secrets, secret)
	return nil
}

func (o *OptionsBuilder) getAvailableConfigResources(ctx context.Context, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) ([]client.Object, error) {
	ret := []client.Object{}

	for _, cfg := range mcAddon.Status.ConfigReferences {
		var obj client.Object
		switch cfg.Resource {
		case prometheusalpha1.PrometheusAgentName:
			obj = &prometheusalpha1.PrometheusAgent{}
		case prometheusalpha1.ScrapeConfigName:
			obj = &prometheusalpha1.ScrapeConfig{}
		case prometheusv1.PrometheusRuleName:
			obj = &prometheusv1.PrometheusRule{}
		case "configmaps":
			obj = &corev1.ConfigMap{}
		default:
			continue
		}

		if cfg.DesiredConfig == nil {
			return ret, fmt.Errorf("%w: %s from %s/%s", errMissingDesiredConfig, cfg.Resource, mcAddon.Namespace, mcAddon.Name)
		}

		if err := o.Client.Get(ctx, types.NamespacedName{Name: cfg.DesiredConfig.Name, Namespace: cfg.DesiredConfig.Namespace}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return ret, err
		}

		ret = append(ret, obj)
	}

	return ret, nil
}

func createWriteRelabelConfigs(clusterName, clusterID string, isHypershiftLocalCluster bool) []prometheusv1.RelabelConfig {
	ret := []prometheusv1.RelabelConfig{}
	if isHypershiftLocalCluster {
		// Don't overwrite the clusterID label as some are set to the hosted cluster ID (for hosted etcd and apiserver)
		// These rules ensure that the correct management cluster labels are set if the clusterID label differs from the current cluster one.
		// If the clusterID it the current cluster one, nothing is done.
		var isNotHcpTmpLabel prometheusv1.LabelName = "__tmp_is_not_hcp"
		ret = append(ret,
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        "^$", // Is empty
				TargetLabel:  config.ClusterNameMetricLabel,
				Action:       "replace",
				Replacement:  &clusterName,
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        "^$", // Is empty
				TargetLabel:  config.ClusterIDMetricLabel,
				Action:       "replace",
				Replacement:  &clusterID,
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        clusterID,
				TargetLabel:  string(isNotHcpTmpLabel),
				Action:       "replace",
				Replacement:  ptr.To("true"),
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{isNotHcpTmpLabel},
				Regex:        "^$", // Is not the current clusterID and is not empty
				TargetLabel:  config.ManagementClusterIDMetricLabel,
				Action:       "replace",
				Replacement:  &clusterID,
			},
			prometheusv1.RelabelConfig{
				SourceLabels: []prometheusv1.LabelName{isNotHcpTmpLabel},
				Regex:        "^$", // Is not the current clusterID and is not empty
				TargetLabel:  config.ManagementClusterNameMetricLabel,
				Action:       "replace",
				Replacement:  &clusterName,
			},
		)
	} else {
		// If not hypershift hub, enforce the clusterID and Name on all metrics
		ret = append(ret,
			prometheusv1.RelabelConfig{
				Replacement: &clusterName,
				TargetLabel: config.ClusterNameMetricLabel,
				Action:      "replace",
			},
			prometheusv1.RelabelConfig{
				Replacement: &clusterID,
				TargetLabel: config.ClusterIDMetricLabel,
				Action:      "replace",
			})
	}

	return append(ret,
		prometheusv1.RelabelConfig{
			SourceLabels: []prometheusv1.LabelName{"exported_job"},
			TargetLabel:  "job",
			Action:       "replace",
		},
		prometheusv1.RelabelConfig{
			SourceLabels: []prometheusv1.LabelName{"exported_instance"},
			TargetLabel:  "instance",
			Action:       "replace",
		},
		prometheusv1.RelabelConfig{
			Regex:  "exported_job|exported_instance",
			Action: "labeldrop",
		})
}
