package handlers

import (
	"context"
	"fmt"
	"slices"

	prometheusalpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/metrics/manifests"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterIDLabel                  = "clusterID"
	imagesCMName                    = "images-list"
	hubNamespace                    = "open-cluster-management-observability"
	imageManifestLabelKey           = "ocm-configmap-type"
	imageManifestLabelValue         = "image-manifest"
	versionLabelKey                 = "ocm-release-version"
	mceNamespace                    = "open-cluster-management"
	platformMetricsCollectorApp     = "acm-platform-metrics-collector"
	userWorkloadMetricsCollectorApp = "acm-user-workload-metrics-collector"
)

func BuildOptions(ctx context.Context, k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, platform, userWorkloads addon.MetricsOptions) (manifests.Options, error) {
	ret := manifests.Options{}

	// Fetch the managed cluster and set identifiers
	managedCluster, err := getManagedCluster(ctx, k8s, mcAddon.GetNamespace())
	if err != nil {
		return ret, err
	}
	ret.ClusterName = managedCluster.Name
	ret.ClusterID = managedCluster.ObjectMeta.Labels[clusterIDLabel]

	// Fetch image overrides
	ret.Images, err = getImageOverrides(ctx, k8s, hubNamespace)
	if err != nil {
		return ret, fmt.Errorf("failed to get image overrides: %w", err)
	}

	// Fetch configuration references
	configResources, err := getConfigResources(ctx, k8s, mcAddon)
	if err != nil {
		return ret, fmt.Errorf("failed to get configuration resources: %w", err)
	}

	// Build Prometheus agents for platform and user workloads
	if platform.CollectionEnabled {
		if err := buildPrometheusAgent(ctx, k8s, &ret, configResources, platformMetricsCollectorApp); err != nil {
			return ret, err
		}

		// Fetch scrape configs
		scrapeConfigs, err := getScrapeConfigs(ctx, k8s, managedCluster.Name, platformMetricsCollectorApp)
		if err != nil {
			return ret, fmt.Errorf("failed to get scrape configs: %w", err)
		}
		ret.Platform.ScrapeConfigs = scrapeConfigs
	}

	if userWorkloads.CollectionEnabled {
		if err := buildPrometheusAgent(ctx, k8s, &ret, configResources, userWorkloadMetricsCollectorApp); err != nil {
			return ret, err
		}

		// Fetch scrape configs
		scrapeConfigs, err := getScrapeConfigs(ctx, k8s, managedCluster.Name, userWorkloadMetricsCollectorApp)
		if err != nil {
			return ret, fmt.Errorf("failed to get scrape configs: %w", err)
		}
		ret.UserWorkloads.ScrapeConfigs = scrapeConfigs
	}

	return ret, nil
}

// Helper function to get ManagedCluster
func getManagedCluster(ctx context.Context, k8s client.Client, namespace string) (*clusterv1.ManagedCluster, error) {
	managedCluster := &clusterv1.ManagedCluster{}
	if err := k8s.Get(ctx, types.NamespacedName{Name: namespace}, managedCluster); err != nil {
		return nil, fmt.Errorf("failed to get managed cluster: %w", err)
	}
	return managedCluster, nil
}

// buildPrometheusAgent abstracts the logic of building a Prometheus agent for platform or user workloads
func buildPrometheusAgent(ctx context.Context, k8s client.Client, opts *manifests.Options, configResources []client.Object, appName string) error {
	// Fetch Prometheus agent resource
	platformAgents := GetResourceByLabelSelector[*prometheusalpha1.PrometheusAgent](configResources, map[string]string{"app": appName})
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
	}

	agent := promBuilder.Build()
	agent.Spec.CommonPrometheusFields.ScrapeConfigNamespaceSelector = &metav1.LabelSelector{} // Restrict scraping to the same namespace

	// Set the built agent in the appropriate workload option
	if appName == platformMetricsCollectorApp {
		opts.Platform.PrometheusAgent = agent
	} else {
		opts.UserWorkloads.PrometheusAgent = agent
	}

	// Fetch related secrets
	for _, secretName := range agent.Spec.CommonPrometheusFields.Secrets {
		if err := addSecret(ctx, k8s, &opts.Secrets, secretName, agent.Namespace); err != nil {
			return err
		}
	}

	return nil
}

// Simplified addSecret function (unchanged)
func addSecret(ctx context.Context, k8s client.Client, secrets *[]corev1.Secret, secretName, secretNamespace string) error {
	if slices.IndexFunc(*secrets, func(s corev1.Secret) bool { return s.Name == secretName && s.Namespace == secretNamespace }) != -1 {
		return nil
	}

	secret := corev1.Secret{}
	if err := k8s.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &secret); err != nil {
		return fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, secretNamespace, err)
	}

	*secrets = append(*secrets, secret)
	return nil
}

func getScrapeConfigs(ctx context.Context, c client.Client, namespace string, appName string) ([]*prometheusalpha1.ScrapeConfig, error) {
	selector := labels.Set{"app": appName}

	scrapeConfigs := prometheusalpha1.ScrapeConfigList{}
	if err := c.List(ctx, &scrapeConfigs, &client.ListOptions{Namespace: namespace, LabelSelector: labels.SelectorFromSet(selector)}); err != nil {
		return nil, err
	}

	return scrapeConfigs.Items, nil
}

func getImageOverrides(ctx context.Context, c client.Client, cmNamespace string) (manifests.ImagesOptions, error) {
	ret := manifests.ImagesOptions{}
	// Get the ACM images overrides
	imagesList := corev1.ConfigMap{}
	if err := c.Get(ctx, types.NamespacedName{Name: imagesCMName, Namespace: cmNamespace}, &imagesList); err != nil {
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

	ret.HAProxy = "registry.connect.redhat.com/haproxytech/haproxy@sha256:07ee4e701e6ce23d6c35b37d159244fb14ef9c90190710542ce60492cbe4d68a"

	return ret, nil
}

func getConfigResources(ctx context.Context, c client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) ([]client.Object, error) {
	ret := []client.Object{}
	for _, cfg := range mcAddon.Status.ConfigReferences {
		var obj client.Object
		switch cfg.ConfigGroupResource.Resource {
		case addon.PrometheusAgentResource:
			obj = &prometheusalpha1.PrometheusAgent{}
		case "configmaps":
			obj = &corev1.ConfigMap{}
		default:
			return ret, fmt.Errorf("unsupported configuration reference resource %s in managedClusterAddon.Status.ConfigReferences of %s/%s", cfg.ConfigGroupResource.Resource, mcAddon.Namespace, mcAddon.Name)
		}

		if err := c.Get(ctx, types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace}, obj); err != nil {
			return ret, err
		}

		ret = append(ret, obj)
	}

	return ret, nil
}

// GetResourceByLabelSelector returns the first resource that matches the label selector.
// It works generically for any Kubernetes resource that implements client.Object.
func GetResourceByLabelSelector[T client.Object](resources []client.Object, selector map[string]string) []T {
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
