package handlers

import (
	"context"
	"fmt"
	"slices"

	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterIDLabel                  = "clusterID"
	haProxyImage                    = "registry.connect.redhat.com/haproxytech/haproxy@sha256:07ee4e701e6ce23d6c35b37d159244fb14ef9c90190710542ce60492cbe4d68a"
	platformMetricsCollectorApp     = "acm-platform-metrics-collector"
	userWorkloadMetricsCollectorApp = "acm-user-workload-metrics-collector"
)

type OptionsBuilder struct {
	Client          client.Client
	HubNamespace    string
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
		if err := o.buildPrometheusAgent(ctx, &ret, configResources, platformMetricsCollectorApp); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.Platform.ScrapeConfigs = getResourceByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, map[string]string{"app": platformMetricsCollectorApp})
		ret.Platform.Rules = getResourceByLabelSelector[*prometheusv1.PrometheusRule](configResources, map[string]string{"app": platformMetricsCollectorApp})
	}

	if userWorkloads.CollectionEnabled {
		if err := o.buildPrometheusAgent(ctx, &ret, configResources, userWorkloadMetricsCollectorApp); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.UserWorkloads.ScrapeConfigs = getResourceByLabelSelector[*prometheusalpha1.ScrapeConfig](configResources, map[string]string{"app": userWorkloadMetricsCollectorApp})
		ret.UserWorkloads.Rules = getResourceByLabelSelector[*prometheusv1.PrometheusRule](configResources, map[string]string{"app": userWorkloadMetricsCollectorApp})
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
	platformAgents := getResourceByLabelSelector[*prometheusalpha1.PrometheusAgent](configResources, map[string]string{"app": appName})
	if len(platformAgents) != 1 {
		return fmt.Errorf("invalid number of PrometheusAgent resources for app %s: %d", appName, len(platformAgents))
	}

	// Build the agent using a builder pattern
	promBuilder := PrometheusAgentBuilder{
		Agent:                platformAgents[0].DeepCopy(),
		Name:                 appName,
		ClusterName:          opts.ClusterName,
		ClusterID:            opts.ClusterID,
		HAProxyConfigMapName: fmt.Sprintf("%s-haproxy-config", appName),
		HAProxyImage:         opts.Images.HAProxy,
		MatchLabels:          map[string]string{"app": appName},
		RemoteWriteEndpoint:  o.RemoteWriteURL,
	}

	agent := promBuilder.Build()

	// Set the built agent in the appropriate workload option
	switch appName {
	case platformMetricsCollectorApp:
		opts.Platform.PrometheusAgent = agent
	case userWorkloadMetricsCollectorApp:
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
func (o *OptionsBuilder) addSecret(ctx context.Context, secrets *[]corev1.Secret, secretName, secretNamespace string) error {
	if slices.IndexFunc(*secrets, func(s corev1.Secret) bool { return s.Name == secretName && s.Namespace == secretNamespace }) != -1 {
		return nil
	}

	secret := corev1.Secret{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &secret); err != nil {
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

	ret.HAProxy = haProxyImage

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
		case addon.PrometheusAgentResource:
			obj = &prometheusalpha1.PrometheusAgent{}
		case addon.PrometheusScrapeConfigResource:
			obj = &prometheusalpha1.ScrapeConfig{}
		case addon.PrometheusRuleResource:
			obj = &prometheusv1.PrometheusRule{}
		case "configmaps":
			obj = &corev1.ConfigMap{}
		default:
			return ret, fmt.Errorf("unsupported configuration reference resource %s in managedClusterAddon.Status.ConfigReferences of %s/%s", cfg.ConfigGroupResource.Resource, mcAddon.Namespace, mcAddon.Name)
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
