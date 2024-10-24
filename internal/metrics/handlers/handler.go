package handlers

import (
	"context"
	"fmt"
	"slices"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterIDLabel = "clusterID"
)

type OptionsBuilder struct {
	Client          client.Client
	ImagesConfigMap types.NamespacedName
	RemoteWriteURL  string
}

func (o *OptionsBuilder) Build(ctx context.Context, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, platform, userWorkloads addon.MetricsOptions) (Options, error) {
	ret := Options{}

	if !platform.CollectionEnabled && !userWorkloads.CollectionEnabled {
		return ret, nil
	}

	// Fetch the managed cluster and set cluster identifiers
	managedCluster, err := o.getManagedCluster(ctx, mcAddon.GetNamespace())
	if err != nil {
		return ret, err
	}
	ret.ClusterName = managedCluster.Name
	ret.ClusterID = managedCluster.ObjectMeta.Labels[clusterIDLabel]
	if ret.ClusterID == "" {
		ret.ClusterID = managedCluster.Name
	}

	// Fetch image overrides
	ret.Images, err = o.getImageOverrides(ctx)
	if err != nil {
		return ret, fmt.Errorf("failed to get image overrides: %w", err)
	}

	// Fetch configuration references
	configResources, err := o.getConfigResources(ctx, mcAddon)
	if err != nil {
		return ret, fmt.Errorf("failed to get configuration resources: %w", err)
	}

	// Build Prometheus agents for platform and user workloads
	if platform.CollectionEnabled {
		if err := o.buildPrometheusAgent(ctx, &ret, configResources, config.PlatformMetricsCollectorApp); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.Platform.ScrapeConfigs = getResourceByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, config.PlatformPrometheusMatchLabels)
		ret.Platform.Rules = getResourceByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.PlatformPrometheusMatchLabels)
	}

	if userWorkloads.CollectionEnabled {
		if err := o.buildPrometheusAgent(ctx, &ret, configResources, config.UserWorkloadMetricsCollectorApp); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.UserWorkloads.ScrapeConfigs = getResourceByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, config.UserWorkloadPrometheusMatchLabels)
		ret.UserWorkloads.Rules = getResourceByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.UserWorkloadPrometheusMatchLabels)
	}

	return ret, nil
}

// Helper function to get ManagedCluster
func (o *OptionsBuilder) getManagedCluster(ctx context.Context, namespace string) (*clusterv1.ManagedCluster, error) {
	managedCluster := &clusterv1.ManagedCluster{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: namespace}, managedCluster); err != nil {
		return nil, fmt.Errorf("failed to get managed cluster: %w", err)
	}
	return managedCluster, nil
}

// buildPrometheusAgent abstracts the logic of building a Prometheus agent for platform or user workloads
func (o *OptionsBuilder) buildPrometheusAgent(ctx context.Context, opts *Options, configResources []client.Object, appName string) error {
	// Fetch Prometheus agent resource
	labelsMatcher := config.PlatformPrometheusMatchLabels
	if appName == config.UserWorkloadMetricsCollectorApp {
		labelsMatcher = config.UserWorkloadPrometheusMatchLabels
	}
	platformAgents := getResourceByLabelSelector[*prometheusalpha1.PrometheusAgent](configResources, labelsMatcher)
	if len(platformAgents) != 1 {
		return fmt.Errorf("invalid number of PrometheusAgent resources with labels %+v for application %s, found %d agents", labelsMatcher, appName, len(platformAgents))
	}

	// Fetch the haproxy config
	envoyProxyConfigMap := getResourceByLabelSelector[*corev1.ConfigMap](configResources, labelsMatcher)
	if len(envoyProxyConfigMap) != 1 {
		return fmt.Errorf("invalid number of ConfigMap for Envoy resource with labels %+v for application %s, found %d configmaps", labelsMatcher, appName, len(envoyProxyConfigMap))
	}
	envoyProxyConfigMapName := fmt.Sprintf("%s-envoy-config", appName)
	envoyProxyConfigMap[0].Name = envoyProxyConfigMapName
	envoyProxyConfigMap[0].Labels = labelsMatcher // For convienence and easier retrieval, especially in tests
	opts.ConfigMaps = append(opts.ConfigMaps, envoyProxyConfigMap[0])

	// Build the agent using a builder pattern
	promBuilder := PrometheusAgentBuilder{
		Agent:               platformAgents[0].DeepCopy(),
		Name:                appName,
		ClusterName:         opts.ClusterName,
		ClusterID:           opts.ClusterID,
		EnvoyConfigMapName:  envoyProxyConfigMapName,
		EnvoyProxyImage:     opts.Images.Envoy,
		MatchLabels:         map[string]string{"app": appName},
		RemoteWriteEndpoint: o.RemoteWriteURL,
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
		return fmt.Errorf("unsupported app name %s", appName)
	}

	// Fetch related secrets
	for _, secretName := range agent.Spec.CommonPrometheusFields.Secrets {
		if err := o.addSecret(ctx, &opts.Secrets, secretName, agent.Namespace); err != nil {
			return err
		}
	}

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

	ret.Envoy = config.EnvoyImage

	if ret.PrometheusOperator == "" || ret.PrometheusConfigReloader == "" || ret.KubeRBACProxy == "" {
		return ret, fmt.Errorf("missing image overrides in ConfigMap %s, got %+v", o.ImagesConfigMap.String(), ret)
	}

	return ret, nil
}

func (o *OptionsBuilder) getConfigResources(ctx context.Context, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) ([]client.Object, error) {
	ret := []client.Object{}
	for _, cfg := range mcAddon.Status.ConfigReferences {
		var obj client.Object
		switch cfg.ConfigGroupResource.Resource {
		case prometheusalpha1.PrometheusAgentName:
			obj = &prometheusalpha1.PrometheusAgent{}
		case prometheusalpha1.ScrapeConfigName:
			obj = &prometheusalpha1.ScrapeConfig{}
		case prometheusv1.PrometheusRuleName:
			obj = &prometheusv1.PrometheusRule{}
		case "configmaps":
			obj = &corev1.ConfigMap{}
		case addon.AddonDeploymentConfigResource:
			continue
		default:
			return ret, fmt.Errorf("unsupported configuration reference resource %q in managedClusterAddon.Status.ConfigReferences of %s/%s", cfg.ConfigGroupResource.Resource, mcAddon.Namespace, mcAddon.Name)
		}

		if cfg.DesiredConfig == nil {
			return ret, fmt.Errorf("missing desiredConfig in managedClusterAddon.Status.ConfigReferences of %s/%s", mcAddon.Namespace, mcAddon.Name)
		}

		if err := o.Client.Get(ctx, types.NamespacedName{Name: cfg.DesiredConfig.Name, Namespace: cfg.DesiredConfig.Namespace}, obj); err != nil {
			return ret, err
		}

		ret = append(ret, obj)
	}

	return ret, nil
}

// getResourceByLabelSelector returns the first resource that matches the label selector.
// It works generically for any Kubernetes resource that implements client.Object.
func getResourceByLabelSelector[T client.Object](resources []client.Object, selector map[string]string) []T {
	labelSelector := labels.SelectorFromSet(selector)
	ret := []T{}

	for _, obj := range resources {
		if resource, ok := obj.(T); ok {
			if labelSelector.Matches(labels.Set(resource.GetLabels())) {
				ret = append(ret, resource)
			}
		}
	}

	return ret
}
