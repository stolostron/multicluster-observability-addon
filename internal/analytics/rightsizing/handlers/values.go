package handlers

import (
	"encoding/json"

	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	mconfig "github.com/stolostron/multicluster-observability-addon/internal/metrics/config"
	"k8s.io/utils/ptr"
)

// RightSizingValues contains the helm values for right-sizing
type RightSizingValues struct {
	NamespaceRightSizing      *ComponentValues   `json:"namespaceRightSizing,omitempty"`
	VirtualizationRightSizing *ComponentValues   `json:"virtRightSizing,omitempty"`
	Prediction                *PredictionValues  `json:"prediction,omitempty"`
	ScrapeConfig              *ScrapeConfigValue `json:"scrapeConfig,omitempty"`
}

// PredictionValues are rendered into the chart as .Values.rightSizing.prediction.
type PredictionValues struct {
	Enabled               bool    `json:"enabled"`
	Provider              string  `json:"provider"`
	TrainingIntervalHours int     `json:"trainingIntervalHours"`
	HistoryDays           int     `json:"historyDays"`
	SafetyMarginPercent   float64 `json:"safetyMarginPercent"`
	NamespaceEnabled      bool    `json:"namespaceEnabled"`
	WorkloadEnabled       bool    `json:"workloadEnabled"`
	GPUEnabled            bool    `json:"gpuEnabled"`
	VMEnabled             bool    `json:"vmEnabled"`
}

// ScrapeConfigValue contains the helm values for a ScrapeConfig
type ScrapeConfigValue struct {
	Name        string            `json:"name"`
	Data        string            `json:"data"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// ComponentValues contains the helm values for a single right-sizing component
type ComponentValues struct {
	Enabled bool                  `json:"enabled"`
	Rules   []PrometheusRuleValue `json:"rules,omitempty"`
}

// PrometheusRuleValue contains the helm values for a PrometheusRule
type PrometheusRuleValue struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

// predictionConfigFile is persisted to the hub ConfigMap (config.json) and matches the Helm template schema.
type predictionConfigFile struct {
	Provider              string  `json:"provider"`
	TrainingIntervalHours int     `json:"trainingIntervalHours"`
	HistoryDays           int     `json:"historyDays"`
	SafetyMarginPercent   float64 `json:"safetyMarginPercent"`
}

func mergedPredictionSettings(opts Options) (PredictionValues, error) {
	pv := PredictionValues{
		Enabled:               true,
		Provider:              opts.PredictionProvider,
		TrainingIntervalHours: 1,
		HistoryDays:           7,
		SafetyMarginPercent:   115,
	}
	if pv.Provider == "" {
		pv.Provider = "builtin"
	}
	if len(opts.PredictionConfig) == 0 || string(opts.PredictionConfig) == "null" {
		return pv, nil
	}
	var overlay struct {
		Provider              string  `json:"provider"`
		TrainingIntervalHours int     `json:"trainingIntervalHours"`
		HistoryDays           int     `json:"historyDays"`
		SafetyMarginPercent   float64 `json:"safetyMarginPercent"`
	}
	if err := json.Unmarshal(opts.PredictionConfig, &overlay); err != nil {
		return PredictionValues{}, err
	}
	if overlay.Provider != "" {
		pv.Provider = overlay.Provider
	}
	if overlay.TrainingIntervalHours != 0 {
		pv.TrainingIntervalHours = overlay.TrainingIntervalHours
	}
	if overlay.HistoryDays != 0 {
		pv.HistoryDays = overlay.HistoryDays
	}
	if overlay.SafetyMarginPercent != 0 {
		pv.SafetyMarginPercent = overlay.SafetyMarginPercent
	}
	return pv, nil
}

func buildPredictionValues(opts Options) (*PredictionValues, error) {
	pv, err := mergedPredictionSettings(opts)
	if err != nil {
		return nil, err
	}
	pv.NamespaceEnabled = opts.NamespaceRightSizing.Enabled
	pv.WorkloadEnabled = opts.WorkloadPodRightSizing.Enabled
	pv.GPUEnabled = opts.GPURightSizing.Enabled
	pv.VMEnabled = opts.VirtualizationRightSizing.Enabled
	return &pv, nil
}

// PredictionHubConfigBytes returns JSON for the hub rs-prediction-config data key config.json.
func PredictionHubConfigBytes(opts Options) ([]byte, error) {
	pv, err := mergedPredictionSettings(opts)
	if err != nil {
		return nil, err
	}
	file := predictionConfigFile{
		Provider:              pv.Provider,
		TrainingIntervalHours: pv.TrainingIntervalHours,
		HistoryDays:           pv.HistoryDays,
		SafetyMarginPercent:   pv.SafetyMarginPercent,
	}
	return json.Marshal(file)
}

// BuildValues builds the helm values from the right-sizing options
func BuildValues(opts Options) (*RightSizingValues, error) {
	if !opts.NamespaceRightSizing.Enabled && !opts.VirtualizationRightSizing.Enabled && !opts.PredictionEnabled {
		return nil, nil
	}

	ret := &RightSizingValues{}

	// Build namespace right-sizing values
	if opts.NamespaceRightSizing.Enabled {
		nsValues := &ComponentValues{
			Enabled: true,
		}
		for _, rule := range opts.NamespaceRightSizing.PrometheusRules {
			ruleJSON, err := json.Marshal(rule.Spec)
			if err != nil {
				return nil, err
			}
			nsValues.Rules = append(nsValues.Rules, PrometheusRuleValue{
				Name: rule.Name,
				Data: string(ruleJSON),
			})
		}
		ret.NamespaceRightSizing = nsValues
	}

	// Build virtualization right-sizing values
	if opts.VirtualizationRightSizing.Enabled {
		virtValues := &ComponentValues{
			Enabled: true,
		}
		for _, rule := range opts.VirtualizationRightSizing.PrometheusRules {
			ruleJSON, err := json.Marshal(rule.Spec)
			if err != nil {
				return nil, err
			}
			virtValues.Rules = append(virtValues.Rules, PrometheusRuleValue{
				Name: rule.Name,
				Data: string(ruleJSON),
			})
		}
		ret.VirtualizationRightSizing = virtValues
	}

	if opts.PredictionEnabled {
		pred, err := buildPredictionValues(opts)
		if err != nil {
			return nil, err
		}
		ret.Prediction = pred
	}

	if opts.ScrapeConfig != nil {
		enrichScrapeConfigForPlatform(opts.ScrapeConfig)
		scJSON, err := json.Marshal(opts.ScrapeConfig.Spec)
		if err != nil {
			return nil, err
		}
		ret.ScrapeConfig = &ScrapeConfigValue{
			Name:        opts.ScrapeConfig.Name,
			Data:        string(scJSON),
			Labels:      opts.ScrapeConfig.Labels,
			Annotations: opts.ScrapeConfig.Annotations,
		}
	}

	return ret, nil
}

// enrichScrapeConfigForPlatform sets the platform target, scheme, and scrape class
// on the ScrapeConfig so it can be scraped by the platform PrometheusAgent.
// Right-sizing only runs on OpenShift clusters, so OCP defaults are used.
func enrichScrapeConfigForPlatform(sc *cooprometheusv1alpha1.ScrapeConfig) {
	sc.Spec.ScrapeClassName = ptr.To(mconfig.ScrapeClassCfgName)
	sc.Spec.Scheme = ptr.To("HTTPS")
	sc.Spec.StaticConfigs = []cooprometheusv1alpha1.StaticConfig{
		{
			Targets: []cooprometheusv1alpha1.Target{
				cooprometheusv1alpha1.Target(mconfig.ScrapeClassPlatformTarget),
			},
		},
	}
}
