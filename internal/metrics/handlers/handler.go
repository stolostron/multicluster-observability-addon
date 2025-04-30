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
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errInvalidConfigResourcesCount = errors.New("invalid number of configuration resources")
	errUnsupportedAppName          = errors.New("unsupported app name")
	errMissingImageOverride        = errors.New("missing image override")
	errMissingDesiredConfig        = errors.New("missing desiredConfig in managedClusterAddon.Status.ConfigReferences")
)

type OptionsBuilder struct {
	Client          client.Client
	ImagesConfigMap types.NamespacedName
	RemoteWriteURL  string
	Logger          logr.Logger
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
	ret.Images, err = o.getImageOverrides(ctx)
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

	// Build the agent
	promBuilder := PrometheusAgentBuilder{
		Agent:                    platformAgents[0].DeepCopy(),
		Name:                     appName,
		ClusterName:              opts.ClusterName,
		ClusterID:                opts.ClusterID,
		MatchLabels:              map[string]string{"app": appName},
		RemoteWriteEndpoint:      o.RemoteWriteURL,
		IsHypershiftLocalCluster: isHypershift,
	}

	var agent *prometheusalpha1.PrometheusAgent

	// Set the built agent in the appropriate workload option
	switch appName {
	case config.PlatformMetricsCollectorApp:
		promBuilder.MatchLabels = config.PlatformPrometheusMatchLabels
		agent = promBuilder.Build()
		opts.Platform.PrometheusAgent = agent
	case config.UserWorkloadMetricsCollectorApp:
		promBuilder.MatchLabels = config.UserWorkloadPrometheusMatchLabels
		agent = promBuilder.Build()
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

func (o *OptionsBuilder) getImageOverrides(ctx context.Context) (ImagesOptions, error) {
	ret := ImagesOptions{}
	// Get the ACM images overrides
	imagesList := corev1.ConfigMap{}
	if err := o.Client.Get(ctx, o.ImagesConfigMap, &imagesList); err != nil {
		return ret, err
	}

	for key, value := range imagesList.Data {
		switch key {
		case "prometheus_operator":
			ret.PrometheusOperator = value
		case "prometheus_config_reloader":
			ret.PrometheusConfigReloader = value
		case "kube_rbac_proxy":
			ret.KubeRBACProxy = value
		default:
		}
	}

	if ret.PrometheusOperator == "" || ret.PrometheusConfigReloader == "" || ret.KubeRBACProxy == "" {
		return ret, fmt.Errorf("%w: %+v", errMissingImageOverride, ret)
	}

	return ret, nil
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
