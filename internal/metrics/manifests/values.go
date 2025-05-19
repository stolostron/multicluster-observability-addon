package manifests

import (
	"encoding/json"

	"github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"github.com/stolostron/multicluster-observability-addon/internal/metrics/handlers"
	corev1 "k8s.io/api/core/v1"
)

type MetricsValues struct {
	PlatformEnabled           bool          `json:"platformEnabled"`
	UserWorkloadsEnabled      bool          `json:"userWorkloadsEnabled"`
	Secrets                   []ConfigValue `json:"secrets"`
	Images                    ImagesValues  `json:"images"`
	PrometheusControllerID    string        `json:"prometheusControllerID"`
	PrometheusCAConfigMapName string        `json:"prometheusCAConfigMapName"`
	Platform                  Collector     `json:"platform"`
	UserWorkload              Collector     `json:"userWorkload"`
	UIEnabled                 bool          `json:"uiEnabled,omitempty"`
	UISpec                    UIValues      `json:"ui,omitempty"`
}

type Collector struct {
	AppName             string        `json:"appName"`
	ConfigMaps          []ConfigValue `json:"configMaps"`
	PrometheusAgentSpec ConfigValue   `json:"prometheusAgent"`
	ScrapeConfigs       []ConfigValue `json:"scrapeConfigs"`
	Rules               []ConfigValue `json:"rules"`
	ServiceMonitors     []ConfigValue `json:"serviceMonitors"` // For HCPs custom user workload serviceMonitors
}

type ImagesValues struct {
	PrometheusOperator       string `json:"prometheusOperator"`
	PrometheusConfigReloader string `json:"prometheusConfigReloader"`
}

type ConfigValue struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      string            `json:"data"`
	Labels    map[string]string `json:"labels"`
}

type UIValues struct {
	Enabled bool         `json:"enabled"`
	ACM     ACMValues    `json:"acm,omitempty"`
	Perses  PersesValues `json:"promes,omitempty"`
}

type ACMValues struct {
	Enabled bool `json:"enabled"`
}

type PersesValues struct {
	Enabled bool `json:"enabled"`
}

func BuildValues(opts handlers.Options) (*MetricsValues, error) {
	ret := &MetricsValues{
		PrometheusControllerID:    config.PrometheusControllerID,
		PrometheusCAConfigMapName: config.PrometheusCAConfigMapName,
		Platform: Collector{
			AppName: config.PlatformMetricsCollectorApp,
		},
		UserWorkload: Collector{
			AppName: config.UserWorkloadMetricsCollectorApp,
		},
	}

	// Build Prometheus Agent Spec for Platform
	if opts.Platform.PrometheusAgent != nil {
		ret.PlatformEnabled = true

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
	if opts.UserWorkloads.PrometheusAgent != nil {
		ret.UserWorkloadsEnabled = true

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

	// Set images
	ret.Images = ImagesValues{
		PrometheusOperator:       opts.Images.PrometheusOperator,
		PrometheusConfigReloader: opts.Images.PrometheusConfigReloader,
	}

	if opts.UI.Enabled {
		ret.UIEnabled = opts.UI.Enabled
		ret.UISpec = UIValues{
			Enabled: opts.UI.Enabled,
			ACM:     ACMValues{Enabled: opts.UI.Enabled},
			Perses:  PersesValues{Enabled: opts.UI.Enabled},
		}
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
			Name:   secret.Name,
			Data:   string(dataJSON),
			Labels: secret.Labels,
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
