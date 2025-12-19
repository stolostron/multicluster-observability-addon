package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	ocinfrav1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	crdResourceName = "customresourcedefinitions"
)

var (
	errInvalidConfigResourcesCount = errors.New("invalid number of configuration resources")
	errUnsupportedAppName          = errors.New("unsupported app name")
	errMissingDesiredConfig        = errors.New("missing desiredConfig in managedClusterAddon.Status.ConfigReferences")
	errMissingRemoteWriteConfig    = errors.New("missing expected remote write spec in the prometheusAgent")
	errMissingCMAOOwnership        = errors.New("object is not owned by the ClusterManagementAddOn")
)

type OptionsBuilder struct {
	Client client.Client
	Logger logr.Logger
}

func (o *OptionsBuilder) Build(ctx context.Context, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, managedCluster *clusterv1.ManagedCluster, opts addon.Options) (Options, error) {
	ret := Options{
		IsHub:            common.IsHubCluster(managedCluster),
		InstallNamespace: opts.InstallNamespace,
		NodeSelector:     opts.NodeSelector,
		Tolerations:      opts.Tolerations,
		ProxyConfig:      opts.ProxyConfig,
	}

	if !opts.Platform.Metrics.CollectionEnabled && !opts.UserWorkloads.Metrics.CollectionEnabled {
		return ret, nil
	}

	ret.ClusterName = managedCluster.Name
	ret.ClusterID = common.GetManagedClusterID(managedCluster)
	ret.AlertManagerEndpoint = opts.Platform.Metrics.AlertManagerEndpoint.Host // Just the host othrewise, pronetheus raises error if the scheme is included
	ret.ClusterVendor = managedCluster.Labels[config.ManagedClusterLabelVendorKey]
	// For e2e testing non OCP cases more easily, we use a special annotation to override the cluster vendor
	vendorOverride := mcAddon.Annotations[addoncfg.VendorOverrideAnnotationKey]
	if vendorOverride != "" {
		o.Logger.V(1).Info("Vendor for the managed cluster is overridden.", "managedcluster", managedCluster.Name, "vendor", vendorOverride)
		ret.ClusterVendor = vendorOverride
	}
	if ret.AlertManagerEndpoint == "" && !ret.IsOCPCluster() {
		o.Logger.Info("Alert forwarding is not configured for non OCP cluster as the AlertManager domain is not set in the addOnDeploymentConfig", "managedcluster", managedCluster.Name)
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

	trimmedClusterID, err := getTrimmedClusterID(o.Client)
	if err != nil {
		return ret, fmt.Errorf("failed to get clusterID: %w", err)
	}

	if err = o.addAlertmanagerSecrets(ctx, &ret.Secrets, trimmedClusterID, opts, common.IsOpenShiftVendor(managedCluster)); err != nil {
		return ret, fmt.Errorf("failed to add alertmanager secrets: %w", err)
	}

	// Build Prometheus agents for platform and user workloads
	if opts.Platform.Metrics.CollectionEnabled {
		if err = o.buildPrometheusAgent(ctx, &ret, configResources, config.PlatformMetricsCollectorApp, false); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.Platform.ScrapeConfigs = common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](configResources, config.PlatformPrometheusMatchLabels)
		if len(ret.Platform.ScrapeConfigs) == 0 {
			o.Logger.V(2).Info("No scrape configs found for platform metrics")
		}
		ret.Platform.Rules = common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.PlatformPrometheusMatchLabels)
		if len(ret.Platform.Rules) == 0 {
			o.Logger.V(2).Info("No rules found for platform metrics")
		}
	}

	// Check both if hypershift is enabled and has hosted clusters to limit noisy logs when uwl monitoring is disabled while there is no hostedCluster
	isHypershiftCluster := IsHypershiftEnabled(managedCluster) && HasHostedCLusters(ctx, o.Client, o.Logger)

	if common.IsOpenShiftVendor(managedCluster) && opts.UserWorkloads.Metrics.CollectionEnabled {
		if err = o.buildPrometheusAgent(ctx, &ret, configResources, config.UserWorkloadMetricsCollectorApp, isHypershiftCluster); err != nil {
			return ret, err
		}

		// Fetch rules and scrape configs
		ret.UserWorkloads.ScrapeConfigs = common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](configResources, config.UserWorkloadPrometheusMatchLabels)
		if len(ret.UserWorkloads.ScrapeConfigs) == 0 {
			o.Logger.V(2).Info("No scrape configs found for user workloads")
		}
		ret.UserWorkloads.Rules = common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.UserWorkloadPrometheusMatchLabels)
		if len(ret.UserWorkloads.Rules) == 0 {
			o.Logger.V(2).Info("No rules found for user workloads")
		}
	}

	if isHypershiftCluster {
		if opts.UserWorkloads.Metrics.CollectionEnabled {
			if err = o.buildHypershiftResources(ctx, &ret, managedCluster, configResources); err != nil {
				return ret, fmt.Errorf("failed to generate hypershift resources: %w", err)
			}
		} else {
			o.Logger.Info("User workload monitoring is needed to monitor Hosted Control Planes managed by the hypershift addon. Ignoring related resources creation.")
		}
	}

	if common.IsOpenShiftVendor(managedCluster) {
		if ret.COOIsSubscribed, err = o.cooIsSubscribed(ctx, managedCluster); err != nil {
			return ret, fmt.Errorf("failed to check if coo is subscribed on the managed cluster: %w", err)
		}
		if !ret.COOIsSubscribed {
			// If we deploy our own operator, create an annotation to restart it once the CRDs are established.
			promAgentCRD := workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: crdResourceName,
				Name:     fmt.Sprintf("%s.%s", cooprometheusv1alpha1.PrometheusAgentName, cooprometheusv1alpha1.SchemeGroupVersion.Group),
			}
			scrapeConfigCRD := workv1.ResourceIdentifier{
				Group:    apiextensionsv1.GroupName,
				Resource: crdResourceName,
				Name:     fmt.Sprintf("%s.%s", cooprometheusv1alpha1.ScrapeConfigName, cooprometheusv1alpha1.SchemeGroupVersion.Group),
			}

			feedback, err := common.GetFeedbackValuesForResources(ctx, o.Client, managedCluster.Name, addoncfg.Name, promAgentCRD, scrapeConfigCRD)
			if err != nil {
				return ret, fmt.Errorf("failed to get feedback for CRDs: %w", err)
			}

			type CrdTimestamps struct {
				PrometheusAgent string `json:"prometheusAgent,omitempty"`
				ScrapeConfig    string `json:"scrapeConfig,omitempty"`
			}
			timestamps := CrdTimestamps{}

			// Process PrometheusAgent CRD
			promAgentValues := feedback[promAgentCRD]
			isEstablishedValues := common.FilterFeedbackValuesByName(promAgentValues, addoncfg.IsEstablishedFeedbackName)
			if len(isEstablishedValues) > 0 && isEstablishedValues[0].Value.String != nil && strings.ToLower(*isEstablishedValues[0].Value.String) == "true" {
				lastTransitionTimeValues := common.FilterFeedbackValuesByName(promAgentValues, addoncfg.LastTransitionTimeFeedbackName)
				if len(lastTransitionTimeValues) > 0 && lastTransitionTimeValues[0].Value.String != nil {
					timestamps.PrometheusAgent = *lastTransitionTimeValues[0].Value.String
				}
			}

			// Process ScrapeConfig CRD
			scrapeConfigValues := feedback[scrapeConfigCRD]
			isEstablishedValues = common.FilterFeedbackValuesByName(scrapeConfigValues, addoncfg.IsEstablishedFeedbackName)
			if len(isEstablishedValues) > 0 && isEstablishedValues[0].Value.String != nil && strings.ToLower(*isEstablishedValues[0].Value.String) == "true" {
				lastTransitionTimeValues := common.FilterFeedbackValuesByName(scrapeConfigValues, addoncfg.LastTransitionTimeFeedbackName)
				if len(lastTransitionTimeValues) > 0 && lastTransitionTimeValues[0].Value.String != nil {
					timestamps.ScrapeConfig = *lastTransitionTimeValues[0].Value.String
				}
			}

			if timestamps.PrometheusAgent != "" && timestamps.ScrapeConfig != "" {
				jsonBytes, err := json.Marshal(timestamps)
				if err != nil {
					return ret, fmt.Errorf("failed to marshal CRD timestamps: %w", err)
				}
				ret.CRDEstablishedAnnotation = string(jsonBytes)
			}
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
	platformAgents := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.PrometheusAgent](configResources, labelsMatcher)
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

	// ensure the expected remote write config exists
	remoteWriteSpecIdx := slices.IndexFunc(agent.Spec.RemoteWrite, func(e cooprometheusv1.RemoteWriteSpec) bool {
		return e.Name != nil && *e.Name == config.RemoteWriteCfgName
	})
	if remoteWriteSpecIdx == -1 {
		return fmt.Errorf("%w: failed to get the %q remote write spec in agent %s/%s", errMissingRemoteWriteConfig, config.RemoteWriteCfgName, agent.Namespace, agent.Name)
	}

	// add the relabel cfg to all remote write configs
	for i := range agent.Spec.RemoteWrite {
		agent.Spec.RemoteWrite[i].WriteRelabelConfigs = append(agent.Spec.RemoteWrite[i].WriteRelabelConfigs,
			createWriteRelabelConfigs(opts.ClusterName, opts.ClusterID, isHypershift)...)
	}

	// Add proxy configuration to all remoteWrite configurations
	if opts.ProxyConfig.ProxyURL != nil {
		proxyURL := opts.ProxyConfig.ProxyURL.String()
		for i := range agent.Spec.RemoteWrite {
			agent.Spec.RemoteWrite[i].ProxyURL = &proxyURL
			agent.Spec.RemoteWrite[i].NoProxy = &opts.ProxyConfig.NoProxy
		}
	}

	// Apply addonDeploymentConfig settings
	agent.Spec.Tolerations = opts.Tolerations
	agent.Spec.NodeSelector = opts.NodeSelector

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
		// empty target namespace result in using default $.Release.Namespace in yaml
		if err := o.addSecret(ctx, &opts.Secrets, secretName, agent.Namespace, secretName, ""); err != nil {
			return err
		}
	}

	// Fetch related configmaps
	for _, configMapName := range agent.Spec.ConfigMaps {
		// empty target namespace result in using default $.Release.Namespace in yaml
		if err := o.addConfigMap(ctx, &opts.ConfigMaps, configMapName, agent.Namespace, configMapName, ""); err != nil {
			return err
		}
	}

	return nil
}

func (o *OptionsBuilder) buildHypershiftResources(ctx context.Context, opts *Options, managedCluster *clusterv1.ManagedCluster, configResources []client.Object) error {
	etcdScrapeConfigs := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](configResources, config.EtcdHcpUserWorkloadPrometheusMatchLabels)
	etcdRules := common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.EtcdHcpUserWorkloadPrometheusMatchLabels)
	apiserverScrapeConfigs := common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](configResources, config.ApiserverHcpUserWorkloadPrometheusMatchLabels)
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
// empty target namespace result in using default $.Release.Namespace in yaml
func (o *OptionsBuilder) addSecret(ctx context.Context, secrets *[]*corev1.Secret, secretName, secretNamespace string, targetName string, targetNamespace string) error {
	if slices.IndexFunc(*secrets, func(s *corev1.Secret) bool { return s.Name == secretName && s.Namespace == secretNamespace }) != -1 {
		return nil
	}

	secret := &corev1.Secret{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, secret); err != nil {
		return fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, secretNamespace, err)
	}

	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[addoncfg.AnnotationOriginalResource] = fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	secret.Namespace = targetNamespace
	secret.Name = targetName

	*secrets = append(*secrets, secret)
	return nil
}

func (o *OptionsBuilder) addConfigMap(ctx context.Context, configMaps *[]*corev1.ConfigMap, configMapName, configMapNamespace string, targetName string, targetNamespace string) error {
	if slices.IndexFunc(*configMaps, func(cm *corev1.ConfigMap) bool { return cm.Name == configMapName && cm.Namespace == configMapNamespace }) != -1 {
		return nil
	}

	configMap := &corev1.ConfigMap{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: configMapNamespace}, configMap); err != nil {
		return fmt.Errorf("failed to get configmap %s in namespace %s: %w", configMapName, configMapNamespace, err)
	}

	if configMap.Annotations == nil {
		configMap.Annotations = map[string]string{}
	}
	configMap.Annotations[addoncfg.AnnotationOriginalResource] = fmt.Sprintf("%s/%s", configMap.Namespace, configMap.Name)

	configMap.Namespace = targetNamespace
	configMap.Name = targetName

	*configMaps = append(*configMaps, configMap)
	return nil
}

func (o *OptionsBuilder) getAvailableConfigResources(ctx context.Context, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) ([]client.Object, error) {
	ret := []client.Object{}

	for _, cfg := range mcAddon.Status.ConfigReferences {
		var obj client.Object
		switch cfg.Resource {
		case cooprometheusv1alpha1.PrometheusAgentName:
			obj = &cooprometheusv1alpha1.PrometheusAgent{}
		case cooprometheusv1alpha1.ScrapeConfigName:
			obj = &cooprometheusv1alpha1.ScrapeConfig{}
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

// cooIsSubscribed returns true if coo is considered installed, preventing conflicting resources creation.
// It checks the feedback rules for the scrapeconfigs.monitoring.rhobs CRD.
func (o *OptionsBuilder) cooIsSubscribed(ctx context.Context, managedCluster *clusterv1.ManagedCluster) (bool, error) {
	crdID := workv1.ResourceIdentifier{
		Group:    apiextensionsv1.GroupName,
		Resource: crdResourceName,
		Name:     fmt.Sprintf("%s.%s", cooprometheusv1alpha1.ScrapeConfigName, cooprometheusv1alpha1.SchemeGroupVersion.Group),
	}

	feedback, err := common.GetFeedbackValuesForResources(ctx, o.Client, managedCluster.Name, addoncfg.Name, crdID)
	if err != nil {
		return false, fmt.Errorf("failed to get feedback values for %s: %w", crdID.Name, err)
	}

	crdFeedback, ok := feedback[crdID]
	if !ok || len(crdFeedback) == 0 {
		o.Logger.V(2).Info("scrapeconfigs.monitoring.rhobs CRD not found in manifestwork status, considering COO as not subscribed")
		return false, nil
	}

	olmValues := common.FilterFeedbackValuesByName(crdFeedback, addoncfg.IsOLMManagedFeedbackName)
	for _, v := range olmValues {
		if v.Value.String != nil && strings.ToLower(*v.Value.String) == "true" {
			o.Logger.V(2).Info("found scrapeconfigs.monitoring.rhobs CRD with OLM label, considering COO as subscribed")
			return true, nil
		}
	}

	o.Logger.V(2).Info("scrapeconfigs.monitoring.rhobs CRD missing the OLM label, considering COO as not subscribed")
	return false, nil
}

func createWriteRelabelConfigs(clusterName, clusterID string, isHypershiftLocalCluster bool) []cooprometheusv1.RelabelConfig {
	ret := []cooprometheusv1.RelabelConfig{}
	if isHypershiftLocalCluster {
		// Don't overwrite the clusterID label as some are set to the hosted cluster ID (for hosted etcd and apiserver)
		// These rules ensure that the correct management cluster labels are set if the clusterID label differs from the current cluster one.
		// If the clusterID it the current cluster one, nothing is done.
		var isNotHcpTmpLabel cooprometheusv1.LabelName = "__tmp_is_not_hcp"
		ret = append(ret,
			cooprometheusv1.RelabelConfig{
				SourceLabels: []cooprometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        "^$", // Is empty
				TargetLabel:  config.ClusterNameMetricLabel,
				Action:       "replace",
				Replacement:  &clusterName,
			},
			cooprometheusv1.RelabelConfig{
				SourceLabels: []cooprometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        "^$", // Is empty
				TargetLabel:  config.ClusterIDMetricLabel,
				Action:       "replace",
				Replacement:  &clusterID,
			},
			cooprometheusv1.RelabelConfig{
				SourceLabels: []cooprometheusv1.LabelName{config.ClusterIDMetricLabel},
				Regex:        clusterID,
				TargetLabel:  string(isNotHcpTmpLabel),
				Action:       "replace",
				Replacement:  ptr.To("true"),
			},
			cooprometheusv1.RelabelConfig{
				SourceLabels: []cooprometheusv1.LabelName{isNotHcpTmpLabel},
				Regex:        "^$", // Is not the current clusterID and is not empty
				TargetLabel:  config.ManagementClusterIDMetricLabel,
				Action:       "replace",
				Replacement:  &clusterID,
			},
			cooprometheusv1.RelabelConfig{
				SourceLabels: []cooprometheusv1.LabelName{isNotHcpTmpLabel},
				Regex:        "^$", // Is not the current clusterID and is not empty
				TargetLabel:  config.ManagementClusterNameMetricLabel,
				Action:       "replace",
				Replacement:  &clusterName,
			},
		)
	} else {
		// If not hypershift hub, enforce the clusterID and Name on all metrics
		ret = append(ret,
			cooprometheusv1.RelabelConfig{
				Replacement: &clusterName,
				TargetLabel: config.ClusterNameMetricLabel,
				Action:      "replace",
			},
			cooprometheusv1.RelabelConfig{
				Replacement: &clusterID,
				TargetLabel: config.ClusterIDMetricLabel,
				Action:      "replace",
			})
	}

	return append(ret,
		cooprometheusv1.RelabelConfig{
			SourceLabels: []cooprometheusv1.LabelName{"exported_job"},
			TargetLabel:  "job",
			Action:       "replace",
		},
		cooprometheusv1.RelabelConfig{
			SourceLabels: []cooprometheusv1.LabelName{"exported_instance"},
			TargetLabel:  "instance",
			Action:       "replace",
		},
		cooprometheusv1.RelabelConfig{
			Regex:  "exported_job|exported_instance",
			Action: "labeldrop",
		})
}

func (o *OptionsBuilder) getRouterCACert(ctx context.Context) ([]byte, string, string, error) {
	// Check if BYO certs exist
	amRouteBYOCaSrt := &corev1.Secret{}
	err1 := o.Client.Get(ctx, types.NamespacedName{Name: config.AlertmanagerRouteBYOCAName, Namespace: config.HubInstallNamespace}, amRouteBYOCaSrt)
	amRouteBYOCertSrt := &corev1.Secret{}
	err2 := o.Client.Get(ctx, types.NamespacedName{Name: config.AlertmanagerRouteBYOCERTName, Namespace: config.HubInstallNamespace}, amRouteBYOCertSrt)

	if err1 == nil && err2 == nil {
		return amRouteBYOCaSrt.Data["tls.crt"], amRouteBYOCaSrt.Namespace, amRouteBYOCaSrt.Name, nil
	}

	// No BYO certs, use default ingress certs
	ingressOperator := &operatorv1.IngressController{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: "default", Namespace: "openshift-ingress-operator"}, ingressOperator); err != nil {
		return nil, "", "", fmt.Errorf("failed to get default ingress controller: %w", err)
	}

	routerCASrtName := "router-certs-default"
	// check if custom default certificate is provided or not
	if ingressOperator.Spec.DefaultCertificate != nil {
		routerCASrtName = ingressOperator.Spec.DefaultCertificate.Name
	}

	routerCASecret := &corev1.Secret{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: routerCASrtName, Namespace: "openshift-ingress"}, routerCASecret); err != nil {
		return nil, "", "", fmt.Errorf("failed to get router CA secret: %w", err)
	}
	return routerCASecret.Data["tls.crt"], routerCASecret.Namespace, routerCASecret.Name, nil
}

func (o *OptionsBuilder) addAlertmanagerSecrets(ctx context.Context, secrets *[]*corev1.Secret, trimmedClusterID string, opts addon.Options, isOCP bool) error {
	// Get the router CA secret
	routerCACert, routerCANamespace, routerCAName, err := o.getRouterCACert(ctx)
	if err != nil {
		return fmt.Errorf("failed to get router CA cert: %w", err)
	}

	// Add secrets based on enabled metric collection
	if opts.Platform.Metrics.CollectionEnabled {
		targetNamespace := config.AlertmanagerPlatformNamespace
		if !isOCP {
			targetNamespace = "" // This is replaced by the default value in the help template that is the installation namespace
		}
		if err := o.addSecret(ctx, secrets, config.AlertmanagerAccessorSecretName, config.HubInstallNamespace, config.AlertmanagerAccessorSecretName+"-"+trimmedClusterID, targetNamespace); err != nil {
			return fmt.Errorf("failed to add accessor secret for platform metrics: %w", err)
		}
		ca := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.AlertmanagerRouterCASecretName + "-" + trimmedClusterID,
				Namespace: targetNamespace,
				Annotations: map[string]string{
					addoncfg.AnnotationOriginalResource: fmt.Sprintf("%s/%s", routerCANamespace, routerCAName),
				},
			},
			Data: map[string][]byte{"service-ca.crt": routerCACert},
		}
		*secrets = append(*secrets, ca)
	}

	if isOCP && opts.UserWorkloads.Metrics.CollectionEnabled {
		targetNamespace := config.AlertmanagerUWLNamespace
		if err := o.addSecret(ctx, secrets, config.AlertmanagerAccessorSecretName, config.HubInstallNamespace, config.AlertmanagerAccessorSecretName+"-"+trimmedClusterID, targetNamespace); err != nil {
			return fmt.Errorf("failed to add accessor secret for user workload metrics: %w", err)
		}
		caUWL := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.AlertmanagerRouterCASecretName + "-" + trimmedClusterID,
				Namespace: targetNamespace,
				Annotations: map[string]string{
					addoncfg.AnnotationOriginalResource: fmt.Sprintf("%s/%s", routerCANamespace, routerCAName),
				},
			},
			Data: map[string][]byte{"service-ca.crt": routerCACert},
		}
		*secrets = append(*secrets, caUWL)
	}

	return nil
}

// getClusterID is used to get the cluster uid.
func getClusterID(ctx context.Context, c client.Client) (string, error) {
	clusterVersion := &ocinfrav1.ClusterVersion{}
	if err := c.Get(ctx, types.NamespacedName{Name: "version"}, clusterVersion); err != nil {
		return "", fmt.Errorf("failed to get clusterVersion: %w", err)
	}

	return string(clusterVersion.Spec.ClusterID), nil
}

func getTrimmedClusterID(c client.Client) (string, error) {
	id, err := getClusterID(context.TODO(), c)
	if err != nil {
		return "", err
	}
	// We use this ID later to postfix the follow secrets:
	// hub-alertmanager-router-ca
	// observability-alertmanager-accessor
	//
	// when prom-opreator mounts these secrets to the prometheus-k8s pod
	// it will take the name of the secret, and prepend `secret-` to the
	// volume mount name. However since this is volume mount name is a label
	// that must be at most 63 chars. Therefore we trim it here to 19 chars.
	idTrim := strings.ReplaceAll(id, "-", "")
	return fmt.Sprintf("%.19s", idTrim), nil
}
