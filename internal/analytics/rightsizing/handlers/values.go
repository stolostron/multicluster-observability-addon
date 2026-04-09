package handlers

import (
	"encoding/json"
)

// RightSizingValues contains the helm values for right-sizing
type RightSizingValues struct {
	NamespaceRightSizing      *ComponentValues `json:"namespaceRightSizing,omitempty"`
	VirtualizationRightSizing *ComponentValues `json:"virtRightSizing,omitempty"`
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

	return ret, nil
}
