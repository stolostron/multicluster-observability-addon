package manifests

import "encoding/json"

type MetricsValues struct {
	PlatformEnabled        bool          `json:"platformEnabled"`
	UserWorkloadsEnabled   bool          `json:"userWorkloadsEnabled"`
	Secrets                []SecretValue `json:"secrets"`
	ConfigMaps             []string      `json:"configMaps"`
	PlatformAgentSpec      string        `json:"platformAgentSpec"`
	UserWorkloadsAgentSpec string        `json:"userWorkloadsAgentSpec"`
	Images                 ImagesValues  `json:"images"`
	PrometheusControllerID string        `json:"prometheusControllerID"`
	ScrapeConfigs          []string      `json:"scrapeConfigs"`
}

type ImagesValues struct {
	PrometheusOperator       string `json:"prometheusOperator"`
	PrometheusConfigReloader string `json:"prometheusConfigReloader"`
}

type SecretValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func BuildValues(opts Options) (MetricsValues, error) {
	values := MetricsValues{}

	// Build Prometheus Agent Spec for Platform
	if opts.Platform.PrometheusAgent != nil {
		values.PlatformEnabled = true

		agentJson, err := json.Marshal(opts.Platform.PrometheusAgent.Spec)
		if err != nil {
			return values, err
		}

		values.PlatformAgentSpec = string(agentJson)
	}

	// Build Prometheus Agent Spec for User Workloads
	if opts.UserWorkloads.PrometheusAgent != nil {
		values.UserWorkloadsEnabled = true

		agentJson, err := json.Marshal(opts.UserWorkloads.PrometheusAgent.Spec)
		if err != nil {
			return values, err
		}

		values.UserWorkloadsAgentSpec = string(agentJson)
	}

	// Build scrape configs
	for _, scrapeConfig := range opts.Platform.ScrapeConfigs {
		scrapeConfigJson, err := json.Marshal(scrapeConfig)
		if err != nil {
			return values, err
		}

		values.ScrapeConfigs = append(values.ScrapeConfigs, string(scrapeConfigJson))
	}

	// Build secrets and config maps
	var err error
	values.Secrets, err = buildSecrets(opts)
	if err != nil {
		return values, err
	}

	return values, nil
}
