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
	ScrapeConfig              *ScrapeConfigValue `json:"scrapeConfig,omitempty"`
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

// BuildValues builds the helm values from the right-sizing options
func BuildValues(opts Options) (*RightSizingValues, error) {
	if !opts.NamespaceRightSizing.Enabled && !opts.VirtualizationRightSizing.Enabled {
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
