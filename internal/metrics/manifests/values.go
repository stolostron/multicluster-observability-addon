package manifests

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/handlers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type MetricsValues struct {
	PlatformEnabled               bool          `json:"platformEnabled"`
	UserWorkloadsEnabled          bool          `json:"userWorkloadsEnabled"`
	Secrets                       []ConfigValue `json:"secrets"`
	Images                        ImagesValues  `json:"images"`
	PrometheusControllerID        string        `json:"prometheusControllerID"`
	PrometheusCAConfigMapName     string        `json:"prometheusCAConfigMapName"`
	Platform                      Collector     `json:"platform"`
	UserWorkload                  Collector     `json:"userWorkload"`
	DeployNonOCPStack             bool          `json:"deployNonOCPStack"`
	DeployCOOResources            bool          `json:"deployCOOResources"`
	PrometheusOperatorAnnotations string        `json:"prometheusOperatorAnnotations,omitempty"`
}

type Collector struct {
	AppName             string        `json:"appName"`
	ConfigMaps          []ConfigValue `json:"configMaps"`
	PrometheusAgentSpec ConfigValue   `json:"prometheusAgent"`
	ScrapeConfigs       []ConfigValue `json:"scrapeConfigs"`
	Rules               []ConfigValue `json:"rules"`
	ServiceMonitors     []ConfigValue `json:"serviceMonitors"` // For HCPs custom user workload serviceMonitors
	RBACProxyTLSSecret  string        `json:"rbacProxyTlsSecret"`
	RBACProxyPort       string        `json:"rbacProxyPort"`
}

type ImagesValues struct {
	CooPrometheusOperator    string `json:"cooPrometheusOperator"`
	PrometheusConfigReloader string `json:"prometheusConfigReloader"`
	KubeStateMetrics         string `json:"kubeStateMetrics"`
	NodeExporter             string `json:"nodeExporter"`
	RBACProxyImage           string `json:"rbacProxyImage"`
}

type ConfigValue struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      string            `json:"data"`
	Labels    map[string]string `json:"labels"`
}

func BuildValues(opts handlers.Options) (*MetricsValues, error) {
	ret := &MetricsValues{
		PrometheusControllerID:    config.PrometheusControllerID,
		PrometheusCAConfigMapName: config.PrometheusCAConfigMapName,
		Platform: Collector{
			AppName:            config.PlatformMetricsCollectorApp,
			RBACProxyTLSSecret: config.PlatformRBACProxyTLSSecret,
			RBACProxyPort:      strconv.Itoa(config.RBACProxyPort),
		},
		UserWorkload: Collector{
			AppName:            config.UserWorkloadMetricsCollectorApp,
			RBACProxyTLSSecret: config.UserWorkloadRBACProxyTLSSecret,
			RBACProxyPort:      strconv.Itoa(config.RBACProxyPort),
		},
	}

	isOCPCluster := opts.IsOCPCluster()
	if isOCPCluster {
		configureAgentForOCP(opts.Platform.PrometheusAgent)
		configureAgentForOCP(opts.UserWorkloads.PrometheusAgent)
	} else {
		configureAgentForNonOCP(opts.Platform.PrometheusAgent)
		configureAgentForNonOCP(opts.UserWorkloads.PrometheusAgent)
	}

	// Build Prometheus Agent Spec for Platform
	if opts.IsPlatformEnabled() {
		agentJson, err := json.Marshal(opts.Platform.PrometheusAgent.Spec)
		if err != nil {
			return ret, err
		}

		ret.Platform.PrometheusAgentSpec = ConfigValue{
			Data:   string(agentJson),
			Labels: opts.Platform.PrometheusAgent.Labels,
		}
	}

	// Build Prometheus Agent Spec for User Workloads
	if opts.IsUserWorkloadsEnabled() {
		agentJson, err := json.Marshal(opts.UserWorkloads.PrometheusAgent.Spec)
		if err != nil {
			return ret, err
		}

		ret.UserWorkload.PrometheusAgentSpec = ConfigValue{
			Data:   string(agentJson),
			Labels: opts.UserWorkloads.PrometheusAgent.Labels,
		}
	}

	// Build scrape configs
	for _, scrapeConfig := range opts.Platform.ScrapeConfigs {
		target := config.ScrapeClassPlatformTarget
		scheme := "HTTPS"
		scrapeClassName := config.ScrapeClassCfgName
		if !isOCPCluster {
			target = fmt.Sprintf("prometheus-k8s.%s.svc:9091", config.HubInstallNamespace) // TODO: replace with install namespace from the config
			scrapeClassName = config.NonOCPScrapeClassName
			scrapeConfig.Spec.TLSConfig = &cooprometheusv1.SafeTLSConfig{
				InsecureSkipVerify: ptr.To(true),
			}
		}

		scrapeConfig.Spec.ScrapeClassName = ptr.To(scrapeClassName)
		scrapeConfig.Spec.Scheme = ptr.To(scheme)
		scrapeConfig.Spec.StaticConfigs = []cooprometheusv1alpha1.StaticConfig{
			{
				Targets: []cooprometheusv1alpha1.Target{
					cooprometheusv1alpha1.Target(target),
				},
			},
		}

		scrapeConfigJson, err := json.Marshal(scrapeConfig.Spec)
		if err != nil {
			return ret, err
		}

		ret.Platform.ScrapeConfigs = append(ret.Platform.ScrapeConfigs, ConfigValue{
			Name:   scrapeConfig.Name,
			Data:   string(scrapeConfigJson),
			Labels: scrapeConfig.Labels,
		})
	}

	for _, scrapeConfig := range opts.UserWorkloads.ScrapeConfigs {
		if len(scrapeConfig.Spec.StaticConfigs) == 0 && isOCPCluster {
			scrapeConfig.Spec.ScrapeClassName = ptr.To(config.ScrapeClassCfgName)
			scrapeConfig.Spec.Scheme = ptr.To("HTTPS")
			scrapeConfig.Spec.StaticConfigs = []cooprometheusv1alpha1.StaticConfig{
				{
					Targets: []cooprometheusv1alpha1.Target{
						cooprometheusv1alpha1.Target(config.ScrapeClassUWLTarget),
					},
				},
			}
		}

		scrapeConfigJson, err := json.Marshal(scrapeConfig.Spec)
		if err != nil {
			return ret, err
		}

		ret.UserWorkload.ScrapeConfigs = append(ret.UserWorkload.ScrapeConfigs, ConfigValue{
			Name:   scrapeConfig.Name,
			Data:   string(scrapeConfigJson),
			Labels: scrapeConfig.Labels,
		})
	}

	// Build rules
	for _, rule := range opts.Platform.Rules {
		ruleJson, err := json.Marshal(rule.Spec)
		if err != nil {
			return ret, err
		}

		ret.Platform.Rules = append(ret.Platform.Rules, ConfigValue{
			Name:   rule.Name,
			Data:   string(ruleJson),
			Labels: rule.Labels,
		})
	}

	for _, rule := range opts.UserWorkloads.Rules {
		ruleJson, err := json.Marshal(rule.Spec)
		if err != nil {
			return ret, err
		}

		ret.UserWorkload.Rules = append(ret.UserWorkload.Rules, ConfigValue{
			Name:   rule.Name,
			Data:   string(ruleJson),
			Labels: rule.Labels,
		})
	}

	// Build HCP's serviceMonitors for userWorkloads
	for _, sm := range opts.UserWorkloads.ServiceMonitors {
		specJson, err := json.Marshal(sm.Spec)
		if err != nil {
			return ret, err
		}

		ret.UserWorkload.ServiceMonitors = append(ret.UserWorkload.ServiceMonitors, ConfigValue{
			Name:      sm.Name,
			Namespace: sm.Namespace,
			Labels:    sm.Labels,
			Data:      string(specJson),
		})
	}

	// Build secrets and config maps
	var err error
	ret.Secrets, err = buildSecrets(opts.Secrets)
	if err != nil {
		return ret, err
	}

	// Set config maps
	ret.Platform.ConfigMaps, err = buildConfigMaps(opts.Platform.ConfigMaps)
	if err != nil {
		return ret, err
	}

	ret.UserWorkload.ConfigMaps, err = buildConfigMaps(opts.UserWorkloads.ConfigMaps)
	if err != nil {
		return ret, err
	}

	ret.PlatformEnabled = opts.IsPlatformEnabled()
	ret.UserWorkloadsEnabled = opts.IsUserWorkloadsEnabled()
	ret.DeployNonOCPStack = !isOCPCluster && (ret.PlatformEnabled || ret.UserWorkloadsEnabled)
	ret.DeployCOOResources = !opts.IsHub && (ret.PlatformEnabled || ret.UserWorkloadsEnabled) && !opts.COOIsSubscribed
	ret.PrometheusOperatorAnnotations = opts.CRDEstablishedAnnotation

	// Set images
	ret.Images = ImagesValues{
		CooPrometheusOperator:    opts.Images.CooPrometheusOperatorImage,
		PrometheusConfigReloader: opts.Images.PrometheusConfigReloader,
		KubeStateMetrics:         opts.Images.KubeStateMetrics,
		NodeExporter:             opts.Images.NodeExporter,
		RBACProxyImage:           opts.Images.KubeRBACProxy,
	}

	return ret, nil
}

func buildSecrets(secrets []*corev1.Secret) ([]ConfigValue, error) {
	secretsValue := []ConfigValue{}
	for _, secret := range secrets {
		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return secretsValue, err
		}
		secretValue := ConfigValue{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Data:      string(dataJSON),
			Labels:    secret.Labels,
		}
		secretsValue = append(secretsValue, secretValue)
	}
	return secretsValue, nil
}

func buildConfigMaps(configMaps []*corev1.ConfigMap) ([]ConfigValue, error) {
	configMapsValue := []ConfigValue{}
	for _, configMap := range configMaps {
		dataJSON, err := json.Marshal(configMap.Data)
		if err != nil {
			return configMapsValue, err
		}
		configMapValue := ConfigValue{
			Name:   configMap.Name,
			Data:   string(dataJSON),
			Labels: configMap.Labels,
		}
		configMapsValue = append(configMapsValue, configMapValue)
	}
	return configMapsValue, nil
}

func configureAgentForOCP(agent *cooprometheusv1alpha1.PrometheusAgent) {
	if agent == nil {
		return
	}
	// Add prometheus-ca configmap
	if !slices.Contains(agent.Spec.ConfigMaps, config.PrometheusCAConfigMapName) {
		agent.Spec.ConfigMaps = append(agent.Spec.ConfigMaps, config.PrometheusCAConfigMapName)
	}

	// Add scrape class for ocp-monitoring
	desiredScrapeClass := cooprometheusv1.ScrapeClass{
		Authorization: &cooprometheusv1.Authorization{
			CredentialsFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		},
		Name: config.ScrapeClassCfgName,
		TLSConfig: &cooprometheusv1.TLSConfig{
			CAFile: fmt.Sprintf("/etc/prometheus/configmaps/%s/service-ca.crt", config.PrometheusCAConfigMapName),
		},
		Default: ptr.To(true),
	}

	index := slices.IndexFunc(agent.Spec.ScrapeClasses, func(e cooprometheusv1.ScrapeClass) bool {
		return e.Name == desiredScrapeClass.Name
	})

	if index >= 0 {
		// Preserve user-defined MetricRelabelings
		if len(agent.Spec.ScrapeClasses[index].MetricRelabelings) > 0 {
			desiredScrapeClass.MetricRelabelings = agent.Spec.ScrapeClasses[index].MetricRelabelings
		}
		// Replace existing scrape class
		agent.Spec.ScrapeClasses[index] = desiredScrapeClass
	} else {
		// Add new scrape class
		agent.Spec.ScrapeClasses = append(agent.Spec.ScrapeClasses, desiredScrapeClass)
	}
}

func configureAgentForNonOCP(agent *cooprometheusv1alpha1.PrometheusAgent) {
	if agent == nil {
		return
	}

	desiredScrapeClass := cooprometheusv1.ScrapeClass{
		Authorization: &cooprometheusv1.Authorization{
			CredentialsFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		},
		Name:    config.NonOCPScrapeClassName,
		Default: ptr.To(true),
	}

	index := slices.IndexFunc(agent.Spec.ScrapeClasses, func(e cooprometheusv1.ScrapeClass) bool {
		return e.Name == desiredScrapeClass.Name
	})

	if index >= 0 {
		if len(agent.Spec.ScrapeClasses[index].MetricRelabelings) > 0 {
			desiredScrapeClass.MetricRelabelings = agent.Spec.ScrapeClasses[index].MetricRelabelings
		}
		agent.Spec.ScrapeClasses[index] = desiredScrapeClass
	} else {
		agent.Spec.ScrapeClasses = append(agent.Spec.ScrapeClasses, desiredScrapeClass)
	}
}
