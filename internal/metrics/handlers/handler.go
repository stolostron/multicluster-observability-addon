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
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	"github.com/stolostron/multicluster-observability-addon/internal/addon/common"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/remotewrite"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
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

func (o *OptionsBuilder) Build(ctx context.Context, mcAddon *addonapiv1beta1.ManagedClusterAddOn, managedCluster *clusterv1.ManagedCluster, opts addon.Options) (Options, error) {
	ret := Options{
		IsHub:                     common.IsHubCluster(managedCluster),
		InstallNamespace:          opts.InstallNamespace,
		NodeSelector:              opts.NodeSelector,
		Tolerations:               opts.Tolerations,
		ResourceReqs:              opts.ResourceReqs,
		ProxyConfig:               opts.ProxyConfig,
		NodeExporter:              opts.Platform.Metrics.NodeExporter,
		PlatformAlertsEnabled:     opts.Platform.Metrics.AlertsEnabled,
		UserWorkloadAlertsEnabled: opts.UserWorkloads.Metrics.AlertsEnabled,
	}

	if !opts.Platform.Metrics.CollectionEnabled && !opts.UserWorkloads.Metrics.CollectionEnabled {
		return ret, nil
	}

	ret.ClusterName = managedCluster.Name
	ret.ClusterID = common.GetManagedClusterID(managedCluster)
	ret.HubEndpoint = opts.Platform.Metrics.HubEndpoint.Host // Use the same host as the metrics for alerts forwarding
	isOpenShiftVendor := common.IsOpenShiftVendor(managedCluster)
	ret.IsOpenShiftVendor = isOpenShiftVendor

	if fakeVendor := common.VendorIsOverridden(managedCluster); fakeVendor != "" {
		o.Logger.V(2).Info("Vendor for the managed cluster is overridden. This annotation is for testing prupose only", "managedcluster", managedCluster.Name, "newVendor", fakeVendor)
	}

	// Fetch image overrides
	var err error
	ret.Images, err = config.GetImageOverrides(ctx, o.Client, opts.Registries, o.Logger)
	if err != nil {
		return ret, fmt.Errorf("failed to get image overrides: %w", err)
	}

	// Fetch configuration references
	configResources, err := o.getAvailableConfigResources(ctx, mcAddon)
	if err != nil {
		return ret, fmt.Errorf("failed to get configuration resources: %w", err)
	}

	hubId, err := getClusterID(ctx, o.Client)
	if err != nil {
		return ret, fmt.Errorf("failed to get the hub cluster id: %w", err)
	}
	ret.HubClusterID = hubId

	if err = o.addAlertmanagerMtlsSecrets(ctx, &ret.Secrets, config.GetTrimmedClusterID(hubId), opts, isOpenShiftVendor); err != nil {
		return ret, fmt.Errorf("failed to add alertmanager secrets: %w", err)
	}

	// Build Prometheus agents for platform and user workloads
	if opts.Platform.Metrics.CollectionEnabled {
		if err = o.buildPrometheusAgent(ctx, &ret, configResources, config.PlatformMetricsCollectorApp, false); err != nil {
			return ret, fmt.Errorf("failed to build platform metrics collector: %w", err)
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

	if isOpenShiftVendor && opts.UserWorkloads.Metrics.CollectionEnabled {
		if err = o.buildPrometheusAgent(ctx, &ret, configResources, config.UserWorkloadMetricsCollectorApp, isHypershiftCluster); err != nil {
			return ret, fmt.Errorf("failed to build user workloads metrics collector: %w", err)
		}

		// Fetch rules and scrape configs
		ret.UserWorkloads.ScrapeConfigs = common.FilterResourcesByLabelSelector[*cooprometheusv1alpha1.ScrapeConfig](configResources, config.UserWorkloadPrometheusMatchLabels)
		if len(ret.UserWorkloads.ScrapeConfigs) == 0 {
			o.Logger.V(2).Info("No scrape configs found for user workloads")
		}
		ret.UserWorkloads.Rules = common.FilterResourcesByLabelSelector[*prometheusv1.PrometheusRule](configResources, config.UserWorkloadPrometheusMatchLabels)
		ret.UserWorkloads.COORules = append(ret.UserWorkloads.COORules, common.FilterResourcesByLabelSelector[*cooprometheusv1.PrometheusRule](configResources, config.UserWorkloadPrometheusMatchLabels)...)
		if len(ret.UserWorkloads.Rules) == 0 && len(ret.UserWorkloads.COORules) == 0 {
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

	caTargetName := config.GetHubMtlsCASecretName(config.GetTrimmedClusterID(ret.HubClusterID))
	certTargetName := config.GetHubMtlsCertSecretName(config.GetTrimmedClusterID(ret.HubClusterID))

	var rawPatches []MonitoringStackPatch

	// Process Platform ScrapeConfigs
	if opts.Platform.Metrics.CollectionEnabled && len(ret.Platform.ScrapeConfigs) > 0 {
		var patches []MonitoringStackPatch
		var serverRemoteWrites []cooprometheusv1.RemoteWriteSpec
		ret.Platform.ScrapeConfigs, patches, serverRemoteWrites, err = o.processScrapeConfigs(
			ctx,
			ret.Platform.ScrapeConfigs,
			ret.Platform.PrometheusAgent,
			caTargetName,
			certTargetName,
			&ret.Secrets,
			isOpenShiftVendor,
			config.AlertmanagerPlatformNamespace,
		)
		if err != nil {
			return ret, err
		}
		if isOpenShiftVendor {
			rawPatches = append(rawPatches, patches...)
		} else {
			ret.PrometheusServerRemoteWrite = append(ret.PrometheusServerRemoteWrite, serverRemoteWrites...)
		}
	}

	// Process UserWorkloads ScrapeConfigs (OCP only)
	if isOpenShiftVendor && opts.UserWorkloads.Metrics.CollectionEnabled && len(ret.UserWorkloads.ScrapeConfigs) > 0 {
		var patches []MonitoringStackPatch
		ret.UserWorkloads.ScrapeConfigs, patches, _, err = o.processScrapeConfigs(
			ctx,
			ret.UserWorkloads.ScrapeConfigs,
			ret.UserWorkloads.PrometheusAgent,
			caTargetName,
			certTargetName,
			&ret.Secrets,
			isOpenShiftVendor,
			config.AlertmanagerUWLNamespace,
		)
		if err != nil {
			return ret, err
		}
		rawPatches = append(rawPatches, patches...)
	}

	ret.MonitoringStackPatches = rawPatches

	if isOpenShiftVendor {
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
	if len(platformAgents) == 0 {
		// Log a warning and return nil instead of an error to prevent failing the Manifests() build.
		// During uninstallation of the ClusterManagementAddOn, the child configuration resources
		// (like the PrometheusAgent) are garbage collected. Returning nil here ensures that the
		// manifests rendering completes successfully, allowing the addon framework to retrieve,
		// register, and execute the pre-delete cleanup job.
		o.Logger.Info("Warning: invalid number of configuration resources", "error", fmt.Errorf("%w: for application %s, found %d agents with labels %+v", errInvalidConfigResourcesCount, appName, len(platformAgents), labelsMatcher))
		return nil
	}
	if len(platformAgents) > 1 {
		o.Logger.Info("Warning: invalid number of configuration resources", "error", fmt.Errorf("%w: for application %s, found %d agents with labels %+v", errInvalidConfigResourcesCount, appName, len(platformAgents), labelsMatcher))
	}
	agent := platformAgents[0].DeepCopy()

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

	for _, resReq := range opts.ResourceReqs {
		if resReq.ContainerID == "statefulsets:"+appName+":prometheus" {
			agent.Spec.Resources = resReq.Resources
		}
	}

	// Set the built agent in the appropriate workload option
	switch appName {
	case config.PlatformMetricsCollectorApp:
		opts.Platform.PrometheusAgent = agent
	case config.UserWorkloadMetricsCollectorApp:
		opts.UserWorkloads.PrometheusAgent = agent
	default:
		return fmt.Errorf("%w: %s", errUnsupportedAppName, appName)
	}

	trimmedClusterID := config.GetTrimmedClusterID(opts.HubClusterID)
	caTargetName := config.GetHubMtlsCASecretName(trimmedClusterID)
	certTargetName := config.GetHubMtlsCertSecretName(trimmedClusterID)

	// Note: We dynamically rename the secrets referenced in the PrometheusAgent (and their corresponding TLSConfig file paths)
	// on the managed cluster to use the caTargetName and certTargetName secret names.
	// This maintains 100% naming consistency with the Alertmanager mTLS secrets deployed across all target namespaces
	// (addon installation namespace, openshift-monitoring, openshift-user-workload-monitoring, and COO namespaces).
	// Crucially, this consistency is what allows the raw metrics transpilation pipeline (processScrapeConfigs) to safely
	// reuse/copy the RemoteWrite configuration specified in the PrometheusAgent when configuring raw metrics collection
	// directly on the source Prometheus servers (since the file paths inside /etc/prometheus/secrets/... will perfectly align).
	for idx, secret := range agent.Spec.Secrets {
		switch secret {
		case config.HubCASecretName:
			agent.Spec.Secrets[idx] = caTargetName
		case config.ClientCertSecretName:
			agent.Spec.Secrets[idx] = certTargetName
		}
	}

	// Dynamically update the main acm-observability remote write TLSFilesConfig paths to use the new target secret names
	mainRw := &agent.Spec.RemoteWrite[remoteWriteSpecIdx]
	if mainRw.TLSConfig != nil {
		mainRw.TLSConfig.CAFile = fmt.Sprintf("/etc/prometheus/secrets/%s/%s", caTargetName, config.MTLSCASecretKey)
		mainRw.TLSConfig.CertFile = fmt.Sprintf("/etc/prometheus/secrets/%s/%s", certTargetName, config.MTLSCertSecretKey)
		mainRw.TLSConfig.KeyFile = fmt.Sprintf("/etc/prometheus/secrets/%s/%s", certTargetName, config.MTLSCertKeySecretKey)
	}

	// Fetch related secrets
	for _, secretName := range agent.Spec.Secrets {
		var sourceName string
		sourceNamespace := agent.Namespace
		if secretName == caTargetName {
			sourceName = config.HubCASecretName
			sourceNamespace = config.HubInstallNamespace
		} else if secretName == certTargetName {
			sourceName = config.ClientCertSecretName
			sourceNamespace = config.HubInstallNamespace
		} else if secretName == config.GetAlertmanagerAccessorSecretName(trimmedClusterID) {
			sourceName = config.AlertmanagerAccessorSecretName
			sourceNamespace = config.HubInstallNamespace
		} else {
			sourceName = secretName
		}

		// empty target namespace result in using default $.Release.Namespace in yaml
		if err := o.addSecret(ctx, &opts.Secrets, sourceName, sourceNamespace, secretName, ""); err != nil {
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
	if slices.IndexFunc(*secrets, func(s *corev1.Secret) bool { return s.Name == targetName && s.Namespace == targetNamespace }) != -1 {
		return nil
	}

	secret := &corev1.Secret{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, secret); err != nil {
		return fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, secretNamespace, err)
	}

	secret = secret.DeepCopy()
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
	if slices.IndexFunc(*configMaps, func(cm *corev1.ConfigMap) bool { return cm.Name == targetName && cm.Namespace == targetNamespace }) != -1 {
		return nil
	}

	configMap := &corev1.ConfigMap{}
	if err := o.Client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: configMapNamespace}, configMap); err != nil {
		return fmt.Errorf("failed to get configmap %s in namespace %s: %w", configMapName, configMapNamespace, err)
	}

	configMap = configMap.DeepCopy()
	if configMap.Annotations == nil {
		configMap.Annotations = map[string]string{}
	}
	configMap.Annotations[addoncfg.AnnotationOriginalResource] = fmt.Sprintf("%s/%s", configMap.Namespace, configMap.Name)

	configMap.Namespace = targetNamespace
	configMap.Name = targetName

	*configMaps = append(*configMaps, configMap)
	return nil
}

func (o *OptionsBuilder) getAvailableConfigResources(ctx context.Context, mcAddon *addonapiv1beta1.ManagedClusterAddOn) ([]client.Object, error) {
	ret := []client.Object{}

	for _, cfg := range mcAddon.Status.ConfigReferences {
		var obj client.Object
		switch cfg.Resource {
		case cooprometheusv1alpha1.PrometheusAgentName:
			obj = &cooprometheusv1alpha1.PrometheusAgent{}
		case cooprometheusv1alpha1.ScrapeConfigName:
			obj = &cooprometheusv1alpha1.ScrapeConfig{}
		case prometheusv1.PrometheusRuleName:
			switch cfg.Group {
			case cooprometheusv1.SchemeGroupVersion.Group:
				obj = &cooprometheusv1.PrometheusRule{}
			default:
				obj = &prometheusv1.PrometheusRule{}
			}
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
// It checks the feedback rules for the monitoringstacks.monitoring.rhobs CRD.
func (o *OptionsBuilder) cooIsSubscribed(ctx context.Context, managedCluster *clusterv1.ManagedCluster) (bool, error) {
	crdID := workv1.ResourceIdentifier{
		Group:    apiextensionsv1.GroupName,
		Resource: crdResourceName,
		Name:     config.AlertmanagerCRDName,
	}

	feedback, err := common.GetFeedbackValuesForResources(ctx, o.Client, managedCluster.Name, addoncfg.Name, crdID)
	if err != nil {
		return false, fmt.Errorf("failed to get feedback values for %s: %w", crdID.Name, err)
	}

	crdFeedback, ok := feedback[crdID]
	if !ok || len(crdFeedback) == 0 {
		o.Logger.V(2).Info(fmt.Sprintf("%s CRD not found in manifestwork status, considering COO as not subscribed", config.AlertmanagerCRDName))
		return false, nil
	}

	olmValues := common.FilterFeedbackValuesByName(crdFeedback, addoncfg.IsOLMManagedFeedbackName)
	for _, v := range olmValues {
		if v.Value.String != nil && strings.ToLower(*v.Value.String) == "true" {
			o.Logger.V(2).Info(fmt.Sprintf("found %s CRD with OLM label, considering COO as subscribed", config.AlertmanagerCRDName))
			return true, nil
		}
	}

	o.Logger.V(2).Info(fmt.Sprintf("%s CRD missing the OLM label, considering COO as not subscribed", config.AlertmanagerCRDName))
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

func (o *OptionsBuilder) addAlertmanagerMtlsSecrets(ctx context.Context, secrets *[]*corev1.Secret, trimmedClusterID string, opts addon.Options, isOCP bool) error {
	// Add secrets based on enabled metric collection
	if opts.Platform.Metrics.CollectionEnabled {
		targetNamespace := config.AlertmanagerPlatformNamespace
		if !isOCP {
			targetNamespace = "" // This is replaced by the default value in the help template that is the installation namespace
		}

		// Copy HubCASecretName (observability-managed-cluster-certs) to hub-mtls-ca-<id>
		caTargetName := config.GetHubMtlsCASecretName(trimmedClusterID)
		if err := o.addSecret(ctx, secrets, config.HubCASecretName, config.HubInstallNamespace, caTargetName, targetNamespace); err != nil {
			return fmt.Errorf("failed to add platform mtls ca secret: %w", err)
		}

		// Copy ClientCertSecretName (observability-controller...) to hub-mtls-cert-<id>
		certTargetName := config.GetHubMtlsCertSecretName(trimmedClusterID)
		if err := o.addSecret(ctx, secrets, config.ClientCertSecretName, config.HubInstallNamespace, certTargetName, targetNamespace); err != nil {
			return fmt.Errorf("failed to add platform mtls cert secret: %w", err)
		}

		// Copy AlertmanagerAccessorSecretName
		accessorTargetName := config.GetAlertmanagerAccessorSecretName(trimmedClusterID)
		if err := o.addSecret(ctx, secrets, config.AlertmanagerAccessorSecretName, config.HubInstallNamespace, accessorTargetName, targetNamespace); err != nil {
			return fmt.Errorf("failed to add platform accessor secret: %w", err)
		}
	}

	if isOCP && opts.UserWorkloads.Metrics.CollectionEnabled {
		targetNamespace := config.AlertmanagerUWLNamespace

		// Copy HubCASecretName (observability-managed-cluster-certs) to hub-mtls-ca-<id>
		caTargetName := config.GetHubMtlsCASecretName(trimmedClusterID)
		if err := o.addSecret(ctx, secrets, config.HubCASecretName, config.HubInstallNamespace, caTargetName, targetNamespace); err != nil {
			return fmt.Errorf("failed to add uwl mtls ca secret: %w", err)
		}

		// Copy ClientCertSecretName (observability-controller...) to hub-mtls-cert-<id>
		certTargetName := config.GetHubMtlsCertSecretName(trimmedClusterID)
		if err := o.addSecret(ctx, secrets, config.ClientCertSecretName, config.HubInstallNamespace, certTargetName, targetNamespace); err != nil {
			return fmt.Errorf("failed to add uwl mtls cert secret: %w", err)
		}

		// Copy AlertmanagerAccessorSecretName
		accessorTargetName := config.GetAlertmanagerAccessorSecretName(trimmedClusterID)
		if err := o.addSecret(ctx, secrets, config.AlertmanagerAccessorSecretName, config.HubInstallNamespace, accessorTargetName, targetNamespace); err != nil {
			return fmt.Errorf("failed to add uwl accessor secret: %w", err)
		}
	}

	return nil
}

// processScrapeConfigs filters, mutates, and transpiles scrape configs into MonitoringStack patches when targeted at COO on OCP,
// or directly into Prometheus Server remoteWrite configurations on non-OCP managed clusters.
//
// We return the filtered scrapeConfigs slice in addition to the transpiled remoteWrite patches because
// scrapeConfigs that do NOT have the raw resolution strategy annotation are preserved as-is (not transpiled)
// and must be returned so they can be exported to the managed cluster for standard, downsampled collection
// via the standard Prometheus Agent federation pipeline.
//
// Note: For standard platform/user-workload ScrapeConfigs targeted at the Cluster Monitoring Operator (CMO) ConfigMap,
// updates are managed asynchronously by the endpoint operator. For those, we do not transpile or generate patches;
// instead, we simply append the "-raw" suffix to their "app.kubernetes.io/component" label to ensure the endpoint
// operator recognizes them and updates the CMO ConfigMap correctly, keeping the ScrapeConfig in the returned filtered slice.
func (o *OptionsBuilder) processScrapeConfigs(
	ctx context.Context,
	scrapeConfigs []*cooprometheusv1alpha1.ScrapeConfig,
	agent *cooprometheusv1alpha1.PrometheusAgent,
	caTargetName, certTargetName string,
	secrets *[]*corev1.Secret,
	isOCP bool,
	defaultNamespace string,
) ([]*cooprometheusv1alpha1.ScrapeConfig, []MonitoringStackPatch, []cooprometheusv1.RemoteWriteSpec, error) {
	var filtered []*cooprometheusv1alpha1.ScrapeConfig
	var patches []MonitoringStackPatch
	var serverRemoteWrites []cooprometheusv1.RemoteWriteSpec

	for _, sc := range scrapeConfigs {
		sc = sc.DeepCopy()
		if sc.Annotations == nil {
			sc.Annotations = map[string]string{}
		}

		// 1. Raw Resolution Strategy (OCP & Non-OCP). Standard ScrapeConfigs (without raw strategy) are bypassed and kept as-is.
		if sc.Annotations[config.RawResolutionAnnotation] != config.RawResolutionValue {
			filtered = append(filtered, sc)
			continue
		}

		if sc.Labels == nil {
			sc.Labels = map[string]string{}
		}
		if comp, ok := sc.Labels[addoncfg.ComponentK8sLabelKey]; ok {
			if !strings.HasSuffix(comp, config.RawLabelSuffix) {
				sc.Labels[addoncfg.ComponentK8sLabelKey] = comp + config.RawLabelSuffix
			}
		}

		// 2. Check for COO target stack annotation (for OCP) or transpile directly for Prometheus Server (for non-OCP)
		if isOCP {
			targetStacks := parseCOOMonitoringStacks(sc.Annotations)
			if len(targetStacks) == 0 {
				// For standard platform/user-workloads on OCP, copy mTLS secrets to the standard target namespace (openshift-monitoring or openshift-user-workload-monitoring)
				// so that the default Prometheus instances can authenticate with the hub.
				if slices.IndexFunc(*secrets, func(s *corev1.Secret) bool { return s.Name == caTargetName && s.Namespace == defaultNamespace }) == -1 {
					if scErr := o.addSecret(ctx, secrets, config.HubCASecretName, config.HubInstallNamespace, caTargetName, defaultNamespace); scErr != nil {
						return nil, nil, nil, fmt.Errorf("failed to add standard mtls ca secret to %s: %w", defaultNamespace, scErr)
					}
				}
				if slices.IndexFunc(*secrets, func(s *corev1.Secret) bool { return s.Name == certTargetName && s.Namespace == defaultNamespace }) == -1 {
					if scErr := o.addSecret(ctx, secrets, config.ClientCertSecretName, config.HubInstallNamespace, certTargetName, defaultNamespace); scErr != nil {
						return nil, nil, nil, fmt.Errorf("failed to add standard mtls cert secret to %s: %w", defaultNamespace, scErr)
					}
				}

				// No COO stacks targeted, let it fall through to standard platform export
				filtered = append(filtered, sc)
				continue
			}

			for _, tStack := range targetStacks {
				targetNamespace := tStack.Namespace
				targetName := tStack.Name

				// Copy mTLS secrets to targetNamespace.
				// Note: The outer slices.IndexFunc guards are critical because addSecret appends directly to the output slice.
				// Since multiple ScrapeConfigs can target the exact same COO stack/namespace, the outer guard prevents
				// creating duplicate target secrets in secrets across multiple scrape config iterations.
				if slices.IndexFunc(*secrets, func(s *corev1.Secret) bool { return s.Name == caTargetName && s.Namespace == targetNamespace }) == -1 {
					if scErr := o.addSecret(ctx, secrets, config.HubCASecretName, config.HubInstallNamespace, caTargetName, targetNamespace); scErr != nil {
						return nil, nil, nil, fmt.Errorf("failed to add COO mtls ca secret: %w", scErr)
					}
				}
				if slices.IndexFunc(*secrets, func(s *corev1.Secret) bool { return s.Name == certTargetName && s.Namespace == targetNamespace }) == -1 {
					if scErr := o.addSecret(ctx, secrets, config.ClientCertSecretName, config.HubInstallNamespace, certTargetName, targetNamespace); scErr != nil {
						return nil, nil, nil, fmt.Errorf("failed to add COO mtls cert secret: %w", scErr)
					}
				}

				// Transpile to MonitoringStack patch
				rwSpecs, scErr := remotewrite.Transpile(sc, agent)
				if scErr != nil {
					return nil, nil, nil, fmt.Errorf("failed to transpile scrape config %s/%s: %w", sc.Namespace, sc.Name, scErr)
				}
				if len(rwSpecs) == 0 {
					o.Logger.Info("transpilation returned empty remote write specs for raw scrape config, no selectors parsed", "namespace", sc.Namespace, "name", sc.Name)
					continue
				}

				// We must override the agent's TLSFilesConfig (disk files) to use SafeTLSConfig (kubernetes secret selectors).
				// We do this because we rename the deployed secret to limit its name length and prevent duplicated mount
				// paths on the target Prometheus pod.
				for _, spec := range rwSpecs {
					spec.TLSConfig = createSafeTLSConfig(caTargetName, certTargetName)
				}

				patches = append(patches, MonitoringStackPatch{
					Namespace:        targetNamespace,
					Name:             targetName,
					RemoteWriteSpecs: rwSpecs,
				})
			}
			// Since it's targeted for COO, we do NOT export the ScrapeConfig itself to the managed cluster
			continue
		} else {
			// For non-OCP managed clusters, transpile directly for our Prometheus Server (acm-prometheus-k8s)
			rwSpecs, scErr := remotewrite.Transpile(sc, agent)
			if scErr != nil {
				return nil, nil, nil, fmt.Errorf("failed to transpile scrape config %s/%s: %w", sc.Namespace, sc.Name, scErr)
			}
			if len(rwSpecs) == 0 {
				o.Logger.Info("transpilation returned empty remote write specs for raw scrape config, no selectors parsed", "namespace", sc.Namespace, "name", sc.Name)
				continue
			}

			// We must override the agent's TLSFilesConfig (disk files) to use SafeTLSConfig (kubernetes secret selectors).
			// We do this because we rename the deployed secret to limit its name length and prevent duplicated mount
			// paths on the target Prometheus pod.
			for _, spec := range rwSpecs {
				spec.TLSConfig = createSafeTLSConfig(caTargetName, certTargetName)
				serverRemoteWrites = append(serverRemoteWrites, *spec)
			}
			// Since it's transpiled directly into the Prometheus Server's RemoteWrite, we do NOT export the ScrapeConfig itself
			continue
		}
	}
	return filtered, patches, serverRemoteWrites, nil
}

// getClusterID is used to get the cluster uid.
func getClusterID(ctx context.Context, c client.Client) (string, error) {
	clusterVersion := &ocinfrav1.ClusterVersion{}
	if err := c.Get(ctx, types.NamespacedName{Name: "version"}, clusterVersion); err != nil {
		return "", fmt.Errorf("failed to get clusterVersion: %w", err)
	}

	return string(clusterVersion.Spec.ClusterID), nil
}

// createSafeTLSConfig generates a TLSConfig configured to use SafeTLSConfig Kubernetes secret selectors.
func createSafeTLSConfig(caTargetName, certTargetName string) *cooprometheusv1.TLSConfig {
	return &cooprometheusv1.TLSConfig{
		SafeTLSConfig: cooprometheusv1.SafeTLSConfig{
			CA: cooprometheusv1.SecretOrConfigMap{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: caTargetName,
					},
					Key: config.MTLSCASecretKey,
				},
			},
			Cert: cooprometheusv1.SecretOrConfigMap{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: certTargetName,
					},
					Key: config.MTLSCertSecretKey,
				},
			},
		},
	}
}

// TargetStack represents a parsed namespace/name COO MonitoringStack target.
type TargetStack struct {
	Namespace string
	Name      string
}

// parseCOOMonitoringStacks parses the coo-monitoring-stacks annotation into a list of TargetStack pairs.
func parseCOOMonitoringStacks(annotations map[string]string) []TargetStack {
	if annotations == nil {
		return nil
	}
	cooStacks, ok := annotations[config.COOMonitoringStacksAnnotation]
	if !ok || cooStacks == "" {
		return nil
	}

	var parsed []TargetStack
	stacks := strings.SplitSeq(cooStacks, ",")
	for stack := range stacks {
		parts := strings.Split(strings.TrimSpace(stack), "/")
		if len(parts) == 2 {
			parsed = append(parsed, TargetStack{
				Namespace: parts[0],
				Name:      parts[1],
			})
		}
	}
	return parsed
}
